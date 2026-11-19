package hfs

import (
	"io"
	"strings"
)

// Open finds the HFS and HFS+ volumes in a disk image of the given size:
// a bare volume, a disk with an Apple partition map, such as a CD or a
// hard disk, or either inside a Disk Copy 4.2 or MacBinary file. It
// returns ErrNotImage if it finds none.
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
	switch string(head[1024:1026]) {
	case "BD":
		v, err := openHFS(r, size)
		if err != nil {
			return nil, err
		}
		return []*Volume{v}, nil
	case "H+", "HX":
		v, err := openHFSPlus(r, size)
		if err != nil {
			return nil, err
		}
		return []*Volume{v}, nil
	}
	if string(head[:2]) == "ER" || string(head[512:514]) == "PM" {
		if vs := partitions(r, size, head, depth); len(vs) > 0 {
			return vs, nil
		}
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
