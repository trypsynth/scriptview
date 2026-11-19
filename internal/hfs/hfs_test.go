package hfs

import (
	"bytes"
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// testFile is a file to put on a synthetic volume.
type testFile struct {
	folder     string // "" for the root
	name       string
	typ        string
	data, rsrc []byte
}

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

func (im *image) file(parent, id int, f testFile) {
	data := im.extRecs(id, false, im.put(f.data))
	rsrc := im.extRecs(id, true, im.put(f.rsrc))
	rec := im.catKey(parent, f.name)
	if im.plus {
		d := be16(nil, 2)
		d = append(d, make([]byte, 6)...)
		d = be32(d, id)
		d = append(d, make([]byte, 48-len(d))...)
		d = append(d, (f.typ + "TEST")...)
		d = append(d, make([]byte, 88-len(d))...)
		for _, fk := range []struct {
			n    int
			exts []extent
		}{{len(f.data), data}, {len(f.rsrc), rsrc}} {
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
		d = append(d, (f.typ + "TEST")...)
		d = append(d, make([]byte, 20-len(d))...)
		d = be32(d, id)
		d = be16(d, 0)
		d = be32(be32(d, len(f.data)), len(f.data))
		d = be16(d, 0)
		d = be32(be32(d, len(f.rsrc)), len(f.rsrc))
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

func build(t *testing.T, plus bool, files []testFile) []byte {
	t.Helper()
	im := newImage(plus)
	im.folder(1, 2, "Disk")
	folders := map[string]int{"": 2}
	id := 16
	for _, f := range files {
		if _, ok := folders[f.folder]; !ok {
			folders[f.folder] = id
			im.folder(2, id, f.folder)
			id++
		}
	}
	for _, f := range files {
		im.file(folders[f.folder], id, f)
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

var testFiles = []testFile{
	{"", "ReadMe", "TEXT", []byte("hello"), nil},
	{"Scripts", "Backup", "osas", nil, bytes.Repeat([]byte("R"), 1500)},
	// 10 blocks: more extents than a catalog record holds, in both formats.
	{"Scripts", "Big", "STAK", bytes.Repeat([]byte("0123456789"), 500), []byte("fork")},
}

func check(t *testing.T, vs []*Volume, err error, plus bool) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Plus != plus {
		t.Fatalf("got %d volumes", len(vs))
	}
	v := vs[0]
	files := v.Files(nil)
	if len(files) != len(testFiles) {
		t.Fatalf("got %d files", len(files))
	}
	for i, f := range files {
		want := testFiles[i]
		path := strings.Join(f.Path, "/")
		if wp := strings.TrimPrefix(want.folder+"/"+want.name, "/"); path != wp {
			t.Errorf("file %d: path %q, want %q", i, path, wp)
		}
		if f.Type != want.typ || f.Creator != "TEST" {
			t.Errorf("%s: type %q creator %q", path, f.Type, f.Creator)
		}
		data, err := v.ReadData(f)
		if err != nil || !bytes.Equal(data, want.data) {
			t.Errorf("%s: data %d bytes, %v", path, len(data), err)
		}
		rsrc, err := v.ReadRsrc(f)
		if err != nil || !bytes.Equal(rsrc, want.rsrc) {
			t.Errorf("%s: rsrc %d bytes, %v", path, len(rsrc), err)
		}
	}
}

func TestHFS(t *testing.T) {
	vol := build(t, false, testFiles)
	vs, err := Open(bytes.NewReader(vol), int64(len(vol)))
	check(t, vs, err, false)
	if vs[0].Name != "Disk" {
		t.Errorf("name %q", vs[0].Name)
	}
}

func TestHFSPlus(t *testing.T) {
	vol := build(t, true, testFiles)
	vs, err := Open(bytes.NewReader(vol), int64(len(vol)))
	check(t, vs, err, true)
	if vs[0].Name != "Disk" {
		t.Errorf("name %q", vs[0].Name)
	}
}

// TestContainers opens a volume in a partition map, in a Disk Copy 4.2
// image, and in a MacBinary file.
func TestContainers(t *testing.T) {
	vol := build(t, false, testFiles)
	// A partition map: the driver descriptor, two map entries, then the
	// volume from block 3.
	disk := make([]byte, 3*512)
	copy(disk, "ER")
	binary.BigEndian.PutUint16(disk[2:], 512)
	for i, p := range []struct {
		typ          string
		start, count int
	}{{"Apple_partition_map", 1, 2}, {"Apple_HFS", 3, len(vol) / 512}} {
		e := disk[512*(i+1):]
		copy(e, "PM")
		binary.BigEndian.PutUint32(e[4:], 2)
		binary.BigEndian.PutUint32(e[8:], uint32(p.start))
		binary.BigEndian.PutUint32(e[12:], uint32(p.count))
		copy(e[48:], p.typ)
	}
	disk = append(disk, vol...)
	vs, err := Open(bytes.NewReader(disk), int64(len(disk)))
	check(t, vs, err, false)

	dc := make([]byte, 84)
	dc[0] = 4
	copy(dc[1:], "Disk")
	binary.BigEndian.PutUint32(dc[64:], uint32(len(disk)))
	dc[82] = 1
	dc = append(dc, disk...)
	vs, err = Open(bytes.NewReader(dc), int64(len(dc)))
	check(t, vs, err, false)

	mb := make([]byte, 128)
	mb[1] = 4
	copy(mb[2:], "Disk")
	binary.BigEndian.PutUint32(mb[83:], uint32(len(dc)))
	mb = append(mb, dc...)
	vs, err = Open(bytes.NewReader(mb), int64(len(mb)))
	check(t, vs, err, false)

	if _, err := Open(bytes.NewReader(make([]byte, 4096)), 4096); err != ErrNotImage {
		t.Errorf("zeros: %v", err)
	}
}

// TestNames decodes a classic volume's names in the encoding they use.
func TestNames(t *testing.T) {
	files := []testFile{{"", "\x83t\x83@\x83C\x83\x8b\x82\xcc\x88\xea\x97\x97", "TEXT", nil, nil}} // ファイルの一覧
	vol := build(t, false, files)
	vs, err := Open(bytes.NewReader(vol), int64(len(vol)))
	if err != nil {
		t.Fatal(err)
	}
	got := vs[0].Files(nil)[0].Path
	if !slices.Equal(got, []string{"ファイルの一覧"}) {
		t.Errorf("got %q", got)
	}
}

// FuzzOpen checks that damaged images return errors, not panics.
func FuzzOpen(f *testing.F) {
	f.Add(build(&testing.T{}, false, testFiles))
	f.Add(build(&testing.T{}, true, testFiles))
	f.Fuzz(func(t *testing.T, data []byte) {
		vs, err := Open(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		for _, v := range vs {
			// A damaged catalog can list thousands of files, each as
			// big as the image, which is slow to read but not wrong.
			for i, fl := range v.Files(nil) {
				if i == 50 {
					break
				}
				v.ReadData(fl)
				v.ReadRsrc(fl)
			}
		}
	})
}
