// Package hfs reads the files on classic Mac disks: HFS and HFS+ volumes,
// in raw disk images, Disk Copy 4.2 and MacBinary files, and partitioned
// disks such as CDs and hard disks.
//
// It reads a file's data and resource forks, its type and creator, and its
// path. It reads through an io.ReaderAt, so a large image isn't loaded into
// memory.
package hfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/trypsynth/scriptview/internal/mactext"
)

// ErrNotImage is returned for data that holds no HFS or HFS+ volume.
var ErrNotImage = errors.New("not an HFS or HFS+ disk image")

var be = binary.BigEndian

// A Volume is one HFS or HFS+ volume.
type Volume struct {
	Name  string
	Plus  bool // HFS+, not classic HFS
	r     io.ReaderAt
	size  int64
	block int64 // allocation block size
	start int64 // offset of allocation block 0
	files []*File
	// overflow holds the extents overflow file's records, by fork.
	overflow map[forkKey][]extent
	rawNames [][]byte // classic HFS names, to pick their encoding
}

// A File is a file on a volume.
type File struct {
	// Path is the file's folders and name, from the volume's root, which
	// isn't included.
	Path          []string
	Type, Creator string
	data, rsrc    fork
	id            uint32
	rawPath       [][]byte // classic HFS: the undecoded path
}

// DataSize and RsrcSize are the sizes of the file's forks.
func (f *File) DataSize() int64 { return f.data.size }
func (f *File) RsrcSize() int64 { return f.rsrc.size }

type extent struct{ start, count int64 }

type fork struct {
	size    int64
	extents []extent
}

type forkKey struct {
	id   uint32
	rsrc bool
}

// Files returns the volume's files. A classic HFS volume's names are in a
// classic Mac encoding: enc decodes them, or if nil, the encoding that
// mactext.Detect picks from them.
func (v *Volume) Files(enc *mactext.Encoding) []*File {
	if !v.Plus {
		if enc == nil {
			enc = mactext.Detect(v.rawNames, nil)
		}
		for _, f := range v.files {
			f.Path = f.Path[:0]
			for _, n := range f.rawPath {
				f.Path = append(f.Path, enc.DecodeOrRoman(n))
			}
		}
	}
	return v.files
}

// ReadData and ReadRsrc read a file's data and resource forks.
func (v *Volume) ReadData(f *File) ([]byte, error) { return v.readFork(f.id, false, f.data) }
func (v *Volume) ReadRsrc(f *File) ([]byte, error) { return v.readFork(f.id, true, f.rsrc) }

func (v *Volume) readFork(id uint32, rsrc bool, fk fork) ([]byte, error) {
	if fk.size == 0 {
		return nil, nil
	}
	if fk.size < 0 || fk.size > v.size {
		return nil, fmt.Errorf("hfs: fork of %d bytes on a volume of %d", fk.size, v.size)
	}
	exts := fk.extents
	have := int64(0)
	for _, e := range exts {
		have += e.count
	}
	if have*v.block < fk.size {
		// The rest of the extents are in the overflow file, in order.
		exts = append(slices.Clip(exts), v.overflow[forkKey{id, rsrc}]...)
	}
	out := make([]byte, 0, fk.size)
	for _, e := range exts {
		if int64(len(out)) >= fk.size {
			break
		}
		n := min(e.count*v.block, fk.size-int64(len(out)))
		buf := make([]byte, n)
		if _, err := v.r.ReadAt(buf, v.start+e.start*v.block); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		out = append(out, buf...)
	}
	if int64(len(out)) < fk.size {
		return out, fmt.Errorf("hfs: fork has %d of its %d bytes", len(out), fk.size)
	}
	return out, nil
}

// readAll reads a whole fork, for the catalog and extents files.
func (v *Volume) readAll(id uint32, fk fork) ([]byte, error) {
	return v.readFork(id, false, fk)
}

// openHFS reads a classic HFS volume whose master directory block is at
// offset 1024 of r. An HFS volume that wraps an HFS+ volume opens as the
// HFS+ one.
func openHFS(r io.ReaderAt, size int64) (*Volume, error) {
	mdb := make([]byte, 162)
	if _, err := r.ReadAt(mdb, 1024); err != nil {
		return nil, err
	}
	block := int64(be.Uint32(mdb[0x14:]))
	start := int64(be.Uint16(mdb[0x1C:])) * 512
	if block == 0 || block%512 != 0 {
		return nil, fmt.Errorf("hfs: allocation block size %d", block)
	}
	if string(mdb[0x7C:0x7E]) == "H+" {
		// An HFS+ volume embedded in an HFS wrapper, for Macs that start
		// only from HFS.
		off := start + int64(be.Uint16(mdb[0x7E:]))*block
		n := int64(be.Uint16(mdb[0x80:])) * block
		return openHFSPlus(io.NewSectionReader(r, off, n), n)
	}
	v := &Volume{r: r, size: size, block: block, start: start, overflow: map[forkKey][]extent{}}
	nameLen := min(int(mdb[0x24]), 27)
	v.rawNames = append(v.rawNames, mdb[0x25:0x25+nameLen])
	v.Name = mactext.MacRoman(mdb[0x25 : 0x25+nameLen])
	ext, err := v.readAll(3, fork{int64(be.Uint32(mdb[0x82:])), hfsExtents(mdb[0x86:])})
	if err != nil {
		return nil, err
	}
	err = eachLeafRecord(ext, func(rec []byte) {
		if len(rec) < 8+12 || rec[0] < 7 {
			return
		}
		k := forkKey{be.Uint32(rec[2:]), rec[1] == 0xFF}
		v.overflow[k] = append(v.overflow[k], hfsExtents(rec[8:])...)
	})
	if err != nil {
		return nil, fmt.Errorf("hfs: extents file: %w", err)
	}
	cat, err := v.readAll(4, fork{int64(be.Uint32(mdb[0x92:])), hfsExtents(mdb[0x96:])})
	if err != nil {
		return nil, err
	}
	type folder struct {
		parent uint32
		name   []byte
	}
	folders := map[uint32]folder{}
	type entry struct {
		parent uint32
		name   []byte
		f      *File
	}
	var files []entry
	err = eachLeafRecord(cat, func(rec []byte) {
		if len(rec) < 7 {
			return
		}
		kl := int(rec[0])
		if kl < 6 || 1+kl > len(rec) {
			return
		}
		parent := be.Uint32(rec[2:])
		nl := int(rec[6])
		if 7+nl > 1+kl {
			return
		}
		name := rec[7 : 7+nl]
		off := 1 + kl
		off += off & 1
		if off >= len(rec) {
			return
		}
		d := rec[off:]
		switch d[0] {
		case 1: // folder
			if len(d) >= 10 {
				folders[be.Uint32(d[6:])] = folder{parent, name}
			}
		case 2: // file
			if len(d) < 98 {
				return
			}
			f := &File{
				Type:    mactext.MacRoman(d[4:8]),
				Creator: mactext.MacRoman(d[8:12]),
				id:      be.Uint32(d[20:]),
				data:    fork{int64(be.Uint32(d[26:])), hfsExtents(d[74:])},
				rsrc:    fork{int64(be.Uint32(d[36:])), hfsExtents(d[86:])},
			}
			files = append(files, entry{parent, name, f})
		}
	})
	if err != nil {
		return nil, fmt.Errorf("hfs: catalog: %w", err)
	}
	for _, e := range files {
		path := [][]byte{e.name}
		for id, n := e.parent, 0; id != 2 && n < 100; n++ {
			fo, ok := folders[id]
			if !ok {
				break
			}
			path = append([][]byte{fo.name}, path...)
			id = fo.parent
		}
		e.f.rawPath = path
		v.rawNames = append(v.rawNames, e.name)
		v.files = append(v.files, e.f)
	}
	for _, fo := range folders {
		v.rawNames = append(v.rawNames, fo.name)
	}
	return v, nil
}

// hfsExtents reads a classic HFS extent record: three 2-byte start
// blocks and counts.
func hfsExtents(b []byte) []extent {
	var out []extent
	for i := 0; i+4 <= min(len(b), 12); i += 4 {
		if n := int64(be.Uint16(b[i+2:])); n > 0 {
			out = append(out, extent{int64(be.Uint16(b[i:])), n})
		}
	}
	return out
}

// openHFSPlus reads an HFS+ volume whose volume header is at offset 1024
// of r.
func openHFSPlus(r io.ReaderAt, size int64) (*Volume, error) {
	vh := make([]byte, 512)
	if _, err := r.ReadAt(vh, 1024); err != nil {
		return nil, err
	}
	block := int64(be.Uint32(vh[40:]))
	if block < 512 || block&(block-1) != 0 {
		return nil, fmt.Errorf("hfs: block size %d", block)
	}
	v := &Volume{Plus: true, r: r, size: size, block: block, overflow: map[forkKey][]extent{}}
	ext, err := v.readAll(3, plusFork(vh[192:]))
	if err != nil {
		return nil, err
	}
	err = eachLeafRecord(ext, func(rec []byte) {
		if len(rec) < 12+64 || be.Uint16(rec) < 10 {
			return
		}
		k := forkKey{be.Uint32(rec[4:]), rec[2] == 0xFF}
		v.overflow[k] = append(v.overflow[k], plusExtents(rec[12:])...)
	})
	if err != nil {
		return nil, fmt.Errorf("hfs: extents file: %w", err)
	}
	cat, err := v.readAll(4, plusFork(vh[272:]))
	if err != nil {
		return nil, err
	}
	type folder struct {
		parent uint32
		name   string
	}
	folders := map[uint32]folder{}
	type entry struct {
		parent uint32
		name   string
		f      *File
	}
	var files []entry
	err = eachLeafRecord(cat, func(rec []byte) {
		if len(rec) < 8 {
			return
		}
		kl := int(be.Uint16(rec))
		if kl < 6 || 2+kl > len(rec) {
			return
		}
		parent := be.Uint32(rec[2:])
		nl := int(be.Uint16(rec[6:]))
		if 8+2*nl > 2+kl {
			return
		}
		u := make([]uint16, nl)
		for i := range u {
			u[i] = be.Uint16(rec[8+2*i:])
		}
		name := string(utf16.Decode(u))
		d := rec[2+kl:]
		if len(d) < 2 {
			return
		}
		switch be.Uint16(d) {
		case 1: // folder
			if len(d) >= 12 {
				if parent == 1 {
					v.Name = name
				}
				folders[be.Uint32(d[8:])] = folder{parent, name}
			}
		case 2: // file
			if len(d) < 248 {
				return
			}
			f := &File{
				Type:    mactext.MacRoman(d[48:52]),
				Creator: mactext.MacRoman(d[52:56]),
				id:      be.Uint32(d[8:]),
				data:    plusFork(d[88:]),
				rsrc:    plusFork(d[168:]),
			}
			files = append(files, entry{parent, name, f})
		}
	})
	if err != nil {
		return nil, fmt.Errorf("hfs: catalog: %w", err)
	}
	for _, e := range files {
		path := []string{e.name}
		hidden := strings.HasPrefix(e.name, "\x00")
		for id, n := e.parent, 0; id != 2 && n < 100; n++ {
			fo, ok := folders[id]
			if !ok {
				break
			}
			// The folder for hard links' files, whose name starts with
			// NULs, isn't one a user sees.
			hidden = hidden || strings.HasPrefix(fo.name, "\x00")
			path = append([]string{fo.name}, path...)
			id = fo.parent
		}
		if hidden {
			continue
		}
		e.f.Path = path
		v.files = append(v.files, e.f)
	}
	return v, nil
}

// plusFork reads an HFS+ fork data record: its 8-byte size, then from 16,
// eight 4-byte start blocks and counts.
func plusFork(b []byte) fork {
	return fork{int64(be.Uint64(b)), plusExtents(b[16:])}
}

func plusExtents(b []byte) []extent {
	var out []extent
	for i := 0; i+8 <= min(len(b), 64); i += 8 {
		if n := int64(be.Uint32(b[i+4:])); n > 0 {
			out = append(out, extent{int64(be.Uint32(b[i:])), n})
		}
	}
	return out
}

// eachLeafRecord calls fn with each record in a B-tree file's leaf nodes,
// from the first leaf node on. Classic HFS and HFS+ share the layout: a
// 14-byte node descriptor, with the next node at 0, the kind at 8, and the
// record count at 10, records whose offsets are 2-byte numbers at the end
// of the node, and a header node whose record gives the first leaf at 24
// and the node size at 32.
func eachLeafRecord(tree []byte, fn func(rec []byte)) error {
	if len(tree) < 512 {
		if len(tree) == 0 {
			return nil
		}
		return errors.New("tree too short")
	}
	nodeSize := int(be.Uint16(tree[32:]))
	if nodeSize < 512 || nodeSize > len(tree) {
		return fmt.Errorf("tree node size %d", nodeSize)
	}
	seen := map[uint32]bool{}
	for n := be.Uint32(tree[24:]); n != 0 && !seen[n]; {
		seen[n] = true
		off := int(n) * nodeSize
		if off+nodeSize > len(tree) || off < 0 {
			return fmt.Errorf("tree node %d past the end", n)
		}
		node := tree[off : off+nodeSize]
		if int8(node[8]) != -1 {
			return fmt.Errorf("tree node %d isn't a leaf", n)
		}
		count := int(be.Uint16(node[10:]))
		for i := range count {
			p := nodeSize - 2*(i+1)
			q := nodeSize - 2*(i+2) // the next record's offset is its end
			if q < 14 {
				break
			}
			start, end := int(be.Uint16(node[p:])), int(be.Uint16(node[q:]))
			if start < 14 || end > nodeSize || start >= end {
				continue
			}
			fn(node[start:end])
		}
		n = be.Uint32(node)
	}
	return nil
}
