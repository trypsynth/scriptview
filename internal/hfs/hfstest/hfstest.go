// Package hfstest builds small HFS and HFS+ volumes for tests.
package hfstest

import (
	"encoding/binary"
	"slices"
	"unicode/utf16"
)

// A File is a file to put on a volume. Its creator is TEST.
type File struct {
	Folder     string // a folder in the root, or "" for the root itself
	Name       string
	Type       string
	Data, Rsrc []byte
}

type extent struct{ start, count int64 }

// image builds a volume with 512-byte allocation blocks and nodes: the
// header, then the files' forks, then the extents file and the catalog. Each fork longer than one
// block is split into one-block extents, so a fork of more than 3 (HFS) or
// 8 (HFS+) blocks needs the overflow file.
type image struct {
	plus     bool
	vol      []byte
	next     int64 // next free allocation block
	ext, cat [][]byte
}

const bs = 512

func newImage(plus bool) *image {
	// HFS starts its allocation blocks after the 3 header sectors, HFS+
	// at 0, so HFS+ leaves its first 3 blocks to the header.
	im := &image{plus: plus}
	if plus {
		im.next = 3
	}
	return im
}

func (im *image) put(b []byte) []extent {
	var exts []extent
	for len(b) > 0 {
		n := min(len(b), bs)
		blk := make([]byte, bs)
		copy(blk, b[:n])
		im.write(im.next, blk)
		exts = append(exts, extent{im.next, 1})
		im.next++
		b = b[n:]
	}
	return exts
}

// write writes allocation block n.
func (im *image) write(n int64, b []byte) {
	off := 3*512 + n*bs
	if im.plus {
		off = n * bs
	}
	if need := off + int64(len(b)); int64(len(im.vol)) < need {
		im.vol = append(im.vol, make([]byte, need-int64(len(im.vol)))...)
	}
	copy(im.vol[off:], b)
}

func be16(b []byte, v int) []byte { return binary.BigEndian.AppendUint16(b, uint16(v)) }
func be32(b []byte, v int) []byte { return binary.BigEndian.AppendUint32(b, uint32(v)) }
func be64(b []byte, v int) []byte { return binary.BigEndian.AppendUint64(b, uint64(v)) }

// extRecs returns the extents that fit in a catalog record, and adds an
// overflow record for the rest.
func (im *image) extRecs(id int, rsrc bool, exts []extent) []extent {
	max := 3
	if im.plus {
		max = 8
	}
	if len(exts) <= max {
		return exts
	}
	ft := 0
	if rsrc {
		ft = 0xFF
	}
	per := 3
	if im.plus {
		per = 8
	}
	for at := max; at < len(exts); at += per {
		rest := exts[at:min(at+per, len(exts))]
		var rec []byte
		if im.plus {
			rec = be16(rec, 10)
			rec = append(rec, byte(ft), 0)
			rec = be32(rec, id)
			rec = be32(rec, at)
		} else {
			rec = append(rec, 7, byte(ft))
			rec = be32(rec, id)
			rec = be16(rec, at)
		}
		for i := range per {
			switch {
			case i < len(rest) && im.plus:
				rec = be32(be32(rec, int(rest[i].start)), int(rest[i].count))
			case i < len(rest):
				rec = be16(be16(rec, int(rest[i].start)), int(rest[i].count))
			case im.plus:
				rec = be64(rec, 0)
			default:
				rec = be32(rec, 0)
			}
		}
		im.ext = append(im.ext, rec)
	}
	return exts[:max]
}

func (im *image) catKey(parent int, name string) []byte {
	var k []byte
	if im.plus {
		u := utf16.Encode([]rune(name))
		k = be16(k, 6+2*len(u))
		k = be32(k, parent)
		k = be16(k, len(u))
		for _, c := range u {
			k = be16(k, int(c))
		}
		return k
	}
	k = append(k, byte(6+len(name)), 0)
	k = be32(k, parent)
	k = append(k, byte(len(name)))
	k = append(k, name...)
	if len(k)%2 == 1 {
		k = append(k, 0)
	}
	return k
}

func (im *image) folder(parent, id int, name string) {
	rec := im.catKey(parent, name)
	if im.plus {
		d := be16(nil, 1)
		d = append(d, make([]byte, 6)...)
		d = be32(d, id)
		d = append(d, make([]byte, 88-len(d))...)
		rec = append(rec, d...)
	} else {
		d := []byte{1, 0}
		d = append(d, make([]byte, 4)...)
		d = be32(d, id)
		d = append(d, make([]byte, 70-len(d))...)
		rec = append(rec, d...)
	}
	im.cat = append(im.cat, rec)
}

func (im *image) file(parent, id int, f File) {
	data := im.extRecs(id, false, im.put(f.Data))
	rsrc := im.extRecs(id, true, im.put(f.Rsrc))
	rec := im.catKey(parent, f.Name)
	if im.plus {
		d := be16(nil, 2)
		d = append(d, make([]byte, 6)...)
		d = be32(d, id)
		d = append(d, make([]byte, 48-len(d))...)
		d = append(d, (f.Type + "TEST")...)
		d = append(d, make([]byte, 88-len(d))...)
		for _, fk := range []struct {
			n    int
			exts []extent
		}{{len(f.Data), data}, {len(f.Rsrc), rsrc}} {
			d = be64(d, fk.n)
			d = be32(d, 0)
			d = be32(d, (fk.n+bs-1)/bs)
			for i := range 8 {
				if i < len(fk.exts) {
					d = be32(be32(d, int(fk.exts[i].start)), int(fk.exts[i].count))
				} else {
					d = be64(d, 0)
				}
			}
		}
		rec = append(rec, d...)
	} else {
		d := []byte{2, 0, 0, 0}
		d = append(d, (f.Type + "TEST")...)
		d = append(d, make([]byte, 20-len(d))...)
		d = be32(d, id)
		d = be16(d, 0)
		d = be32(be32(d, len(f.Data)), len(f.Data))
		d = be16(d, 0)
		d = be32(be32(d, len(f.Rsrc)), len(f.Rsrc))
		d = append(d, make([]byte, 74-len(d))...)
		for _, exts := range [][]extent{data, rsrc} {
			for i := range 3 {
				if i < len(exts) {
					d = be16(be16(d, int(exts[i].start)), int(exts[i].count))
				} else {
					d = be32(d, 0)
				}
			}
		}
		d = append(d, make([]byte, 102-len(d))...)
		rec = append(rec, d...)
	}
	im.cat = append(im.cat, rec)
}

// tree returns a B-tree file: a header node, then leaf nodes holding the
// records, each linked to the next.
func tree(recs [][]byte) []byte {
	var leaves [][]byte
	for len(recs) > 0 || len(leaves) == 0 {
		leaf := make([]byte, bs)
		leaf[8] = 0xFF
		leaf[9] = 1
		off, n := 14, 0
		for n < len(recs) && off+len(recs[n])+2*(n+2) <= bs {
			binary.BigEndian.PutUint16(leaf[bs-2*(n+1):], uint16(off))
			off += copy(leaf[off:], recs[n])
			n++
		}
		binary.BigEndian.PutUint16(leaf[10:], uint16(n))
		binary.BigEndian.PutUint16(leaf[bs-2*(n+1):], uint16(off))
		if len(leaves) > 0 {
			binary.BigEndian.PutUint32(leaves[len(leaves)-1], uint32(len(leaves)+1))
		}
		leaves = append(leaves, leaf)
		recs = recs[n:]
	}
	hdr := make([]byte, bs)
	hdr[8] = 1
	binary.BigEndian.PutUint16(hdr[10:], 3)
	binary.BigEndian.PutUint16(hdr[14:], 1) // depth
	binary.BigEndian.PutUint32(hdr[16:], 1) // root
	binary.BigEndian.PutUint32(hdr[24:], 1) // first leaf
	binary.BigEndian.PutUint32(hdr[28:], uint32(len(leaves)))
	binary.BigEndian.PutUint16(hdr[32:], bs) // node size
	return slices.Concat(append([][]byte{hdr}, leaves...)...)
}

// Build returns a classic HFS or HFS+ volume holding files. Its name is
// Disk. A fork longer than one block is split into one-block extents, so a
// fork of more than 3 (HFS) or 8 (HFS+) blocks needs the extents overflow
// file.
func Build(plus bool, files []File) []byte {
	im := newImage(plus)
	im.folder(1, 2, "Disk")
	folders := map[string]int{"": 2}
	id := 16
	for _, f := range files {
		if _, ok := folders[f.Folder]; !ok {
			folders[f.Folder] = id
			im.folder(2, id, f.Folder)
			id++
		}
	}
	for _, f := range files {
		im.file(folders[f.Folder], id, f)
		id++
	}
	ext := im.put(tree(im.ext))
	cat := im.put(tree(im.cat))
	vol := im.vol
	if len(vol) < 1536 {
		vol = append(vol, make([]byte, 1536-len(vol))...)
	}
	if plus {
		vh := vol[1024:1536]
		copy(vh, "H+")
		binary.BigEndian.PutUint32(vh[40:], bs)
		for _, fk := range []struct {
			at   int
			exts []extent
		}{{192, ext}, {272, cat}} {
			binary.BigEndian.PutUint64(vh[fk.at:], uint64(len(fk.exts)*bs))
			binary.BigEndian.PutUint32(vh[fk.at+16:], uint32(fk.exts[0].start))
			binary.BigEndian.PutUint32(vh[fk.at+20:], uint32(len(fk.exts)))
		}
	} else {
		mdb := vol[1024:1536]
		copy(mdb, "BD")
		binary.BigEndian.PutUint32(mdb[0x14:], bs)
		binary.BigEndian.PutUint16(mdb[0x1C:], 3)
		mdb[0x24] = 4
		copy(mdb[0x25:], "Disk")
		for _, fk := range []struct {
			at   int
			exts []extent
		}{{0x82, ext}, {0x92, cat}} {
			binary.BigEndian.PutUint32(mdb[fk.at:], uint32(len(fk.exts)*bs))
			binary.BigEndian.PutUint16(mdb[fk.at+4:], uint16(fk.exts[0].start))
			binary.BigEndian.PutUint16(mdb[fk.at+6:], uint16(len(fk.exts)))
		}
	}
	return vol
}
