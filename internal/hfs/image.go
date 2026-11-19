package hfs

import (
	"encoding/binary"
	"io"
	"strings"
)

// Open finds the HFS and HFS+ volumes in a disk image of the given size:
// a bare volume, a disk with an Apple partition map, such as a CD or a
// hard disk, or a GUID partition table, inside a Disk Copy 4.2 or
// MacBinary file or a .dmg image (UDIF), or none. It returns ErrNotImage
// if it finds none. A Disk Copy 6 image needs its resource fork: see
// OpenWithResources.
func Open(r io.ReaderAt, size int64) ([]*Volume, error) {
	return open(r, size, 0)
}

func open(r io.ReaderAt, size int64, depth int) ([]*Volume, error) {
	if size < 1536 || depth > 4 {
		return nil, ErrNotImage
	}
	head := make([]byte, 1536)
	if _, err := r.ReadAt(head, 0); err != nil {
		return nil, ErrNotImage
	}
	// A .dmg image (UDIF) ends with a 512-byte trailer. It comes first:
	// the image's data can start with its disk's first sectors,
	// uncompressed, which look like a bare volume or a partition map.
	koly := make([]byte, 512)
	if _, err := r.ReadAt(koly, size-512); err == nil && string(koly[:4]) == "koly" {
		d, err := udif(r, size, koly)
		if err != nil {
			return nil, err
		}
		return open(d, d.size, depth+1)
	}
	// A signature is two bytes, which other files can have by chance, so
	// the volume header's block size and version must make sense too.
	vh := head[1024:]
	switch {
	case string(vh[:2]) == "BD" && be.Uint32(vh[0x14:]) != 0 && be.Uint32(vh[0x14:])%512 == 0:
		v, err := openHFS(r, size)
		if err != nil {
			return nil, err
		}
		return []*Volume{v}, nil
	case (string(vh[:2]) == "H+" || string(vh[:2]) == "HX") && (be.Uint16(vh[2:]) == 4 || be.Uint16(vh[2:]) == 5) && validBlock(int64(be.Uint32(vh[40:]))):
		v, err := openHFSPlus(r, size)
		if err != nil {
			return nil, err
		}
		return []*Volume{v}, nil
	}
	if string(head[512:514]) == "PM" || string(head[:2]) == "ER" && string(head[512:520]) != "EFI PART" {
		if vs := partitions(r, size, head, depth); len(vs) > 0 {
			return vs, nil
		}
		if string(head[512:514]) == "PM" {
			return nil, ErrNoVolume
		}
	}
	if string(head[512:520]) == "EFI PART" {
		if vs := gptPartitions(r, size, head[512:], depth); len(vs) > 0 {
			return vs, nil
		}
		return nil, ErrNoVolume
	}
	// Disk Copy 4.2: a 64-byte name, the data's size at 64, and 0x0100 at
	// 82, then the disk from 84.
	if head[0] < 64 && head[82] == 1 && head[83] == 0 {
		if n := int64(be.Uint32(head[64:])); n > 0 && 84+n <= size {
			return open(io.NewSectionReader(r, 84, n), n, depth+1)
		}
	}
	// MacBinary: a 128-byte header with the name's length at 1, zeros at
	// 0 and 74, and the data fork's size at 83, then the data fork.
	if head[0] == 0 && head[74] == 0 && head[1] > 0 && head[1] < 64 {
		if n := int64(be.Uint32(head[83:])); n > 0 && 128+n <= size {
			return open(io.NewSectionReader(r, 128, n), n, depth+1)
		}
	}
	return nil, ErrNotImage
}

// gptPartitions opens the HFS+ partitions in a GUID partition table, as
// on Intel Macs' disks and .dmg images: the header at 512 gives the
// entries' first sector at 72, their number at 80, and their size at 84,
// all little-endian. Each entry has its first and last sectors at 32 and
// 40. scriptview opens any partition that holds an HFS or HFS+ volume.
func gptPartitions(r io.ReaderAt, size int64, hdr []byte, depth int) []*Volume {
	le := binary.LittleEndian
	at, n, es := int64(le.Uint64(hdr[72:]))*512, int(le.Uint32(hdr[80:])), int64(le.Uint32(hdr[84:]))
	if es < 128 || es > 4096 || n > 256 {
		return nil
	}
	var vols []*Volume
	for i := range n {
		e := make([]byte, 48)
		if _, err := r.ReadAt(e, at+int64(i)*es); err != nil {
			break
		}
		first, last := int64(le.Uint64(e[32:]))*512, (int64(le.Uint64(e[40:]))+1)*512
		if first <= 0 || first >= size || last <= first {
			continue
		}
		last = min(last, size)
		if vs, err := open(io.NewSectionReader(r, first, last-first), last-first, depth+1); err == nil {
			vols = append(vols, vs...)
		}
	}
	return vols
}

// partitions opens the HFS and HFS+ partitions in an Apple partition map:
// a driver descriptor with the block size at 2, then a 512-byte entry in
// each block from block 1, with "PM" at 0, the map's length in entries at
// 4, the partition's start and length in blocks at 8 and 12, and its type
// at 48.
func partitions(r io.ReaderAt, size int64, head []byte, depth int) []*Volume {
	bs := int64(512)
	if string(head[:2]) == "ER" && string(head[512:514]) != "PM" {
		bs = int64(be.Uint16(head[2:]))
	}
	if bs < 512 {
		return nil
	}
	var vols []*Volume
	n := int64(1)
	for i := int64(1); i <= n && i < 256; i++ {
		e := make([]byte, 512)
		if _, err := r.ReadAt(e, i*bs); err != nil || string(e[:2]) != "PM" {
			break
		}
		if i == 1 {
			n = int64(be.Uint32(e[4:]))
		}
		typ, _, _ := strings.Cut(string(e[48:80]), "\x00")
		if typ != "Apple_HFS" && typ != "Apple_HFSX" {
			continue
		}
		start, count := int64(be.Uint32(e[8:]))*bs, int64(be.Uint32(e[12:]))*bs
		if start <= 0 || start >= size {
			continue
		}
		count = min(count, size-start)
		if vs, err := open(io.NewSectionReader(r, start, count), count, depth+1); err == nil {
			vols = append(vols, vs...)
		}
	}
	return vols
}

// validBlock reports whether n is an HFS+ block size: a power of two, at
// least 512.
func validBlock(n int64) bool {
	return n >= 512 && n&(n-1) == 0
}
