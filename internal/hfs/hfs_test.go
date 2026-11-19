package hfs

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/trypsynth/scriptview/internal/hfs/hfstest"
)

var testFiles = []hfstest.File{
	{Name: "ReadMe", Type: "TEXT", Data: []byte("hello")},
	{Folder: "Scripts", Name: "Backup", Type: "osas", Rsrc: bytes.Repeat([]byte("R"), 1500)},
	// 10 blocks: more extents than a catalog record holds, in both formats.
	{Folder: "Scripts", Name: "Big", Type: "STAK", Data: bytes.Repeat([]byte("0123456789"), 500), Rsrc: []byte("fork")},
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
		if wp := strings.TrimPrefix(want.Folder+"/"+want.Name, "/"); path != wp {
			t.Errorf("file %d: path %q, want %q", i, path, wp)
		}
		if f.Type != want.Type || f.Creator != "TEST" {
			t.Errorf("%s: type %q creator %q", path, f.Type, f.Creator)
		}
		data, err := v.ReadData(f)
		if err != nil || !bytes.Equal(data, want.Data) {
			t.Errorf("%s: data %d bytes, %v", path, len(data), err)
		}
		rsrc, err := v.ReadRsrc(f)
		if err != nil || !bytes.Equal(rsrc, want.Rsrc) {
			t.Errorf("%s: rsrc %d bytes, %v", path, len(rsrc), err)
		}
	}
}

func TestHFS(t *testing.T) {
	vol := hfstest.Build(false, testFiles)
	vs, err := Open(bytes.NewReader(vol), int64(len(vol)))
	check(t, vs, err, false)
	if vs[0].Name != "Disk" {
		t.Errorf("name %q", vs[0].Name)
	}
}

func TestHFSPlus(t *testing.T) {
	vol := hfstest.Build(true, testFiles)
	vs, err := Open(bytes.NewReader(vol), int64(len(vol)))
	check(t, vs, err, true)
	if vs[0].Name != "Disk" {
		t.Errorf("name %q", vs[0].Name)
	}
}

// TestContainers opens a volume in a partition map, in a Disk Copy 4.2
// image, and in a MacBinary file.
func TestContainers(t *testing.T) {
	vol := hfstest.Build(false, testFiles)
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
	files := []hfstest.File{{Name: "\x83t\x83@\x83C\x83\x8b\x82\xcc\x88\xea\x97\x97", Type: "TEXT"}} // ファイルの一覧
	vol := hfstest.Build(false, files)
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
	f.Add(hfstest.Build(false, testFiles))
	f.Add(hfstest.Build(true, testFiles))
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

// rsrcFork builds a resource fork holding one resource.
func rsrcFork(typ string, id int16, data []byte) []byte {
	body := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	body = append(body, data...)
	m := make([]byte, 28)
	binary.BigEndian.PutUint16(m[24:], 28) // type list
	binary.BigEndian.PutUint16(m[26:], 28+2+8+12)
	m = binary.BigEndian.AppendUint16(m, 0) // one type
	m = append(m, typ...)
	m = binary.BigEndian.AppendUint16(m, 0)  // one resource
	m = binary.BigEndian.AppendUint16(m, 10) // its reference, from the type list
	m = binary.BigEndian.AppendUint16(m, uint16(id))
	m = binary.BigEndian.AppendUint16(m, 0xFFFF)
	m = append(m, 0, 0, 0, 0, 0, 0, 0, 0) // attributes, data offset 0, handle
	hdr := make([]byte, 16)
	binary.BigEndian.PutUint32(hdr, 256)
	binary.BigEndian.PutUint32(hdr[4:], uint32(256+len(body)))
	binary.BigEndian.PutUint32(hdr[8:], uint32(len(body)))
	binary.BigEndian.PutUint32(hdr[12:], uint32(len(m)))
	fork := append(hdr, make([]byte, 256-16)...)
	return append(append(fork, body...), m...)
}

// adcLiterals compresses b with ADC literals only.
func adcLiterals(b []byte) []byte {
	var out []byte
	for len(b) > 0 {
		n := min(len(b), 128)
		out = append(append(out, byte(0x80|(n-1))), b[:n]...)
		b = b[n:]
	}
	return out
}

func TestADC(t *testing.T) {
	// "ABCD", then a short copy of 18 bytes and a long one of 67, both
	// from 4 back.
	in := []byte{0x83, 'A', 'B', 'C', 'D', 0x3c, 3, 0x7f, 0, 3}
	want := bytes.Repeat([]byte("ABCD"), 23)[:4+18+67]
	if got := adc(in, 1000); !bytes.Equal(got, want) {
		t.Errorf("got %q", got)
	}
}

// chunks splits a disk into 8-sector chunks for a compressed image,
// alternating between two kinds, with zero chunks where it's all zeros.
func chunked(disk []byte, encode func(i int, b []byte) (kind uint32, data []byte)) (sectors [][3]int, data []byte, kinds []uint32) {
	for s := 0; s*512 < len(disk); s += 8 {
		b := disk[s*512 : min((s+8)*512, len(disk))]
		kind, enc := uint32(0), []byte(nil)
		if bytes.Count(b, []byte{0}) != len(b) {
			kind, enc = encode(s/8, b)
		}
		sectors = append(sectors, [3]int{s, (len(b) + 511) / 512, len(data)})
		kinds = append(kinds, kind)
		data = append(data, enc...)
	}
	return
}

func TestNDIF(t *testing.T) {
	vol := hfstest.Build(false, testFiles)
	sectors, data, kinds := chunked(vol, func(i int, b []byte) (uint32, []byte) {
		if i%2 == 0 {
			return 0x02, b
		}
		return 0x83, adcLiterals(b)
	})
	bcem := make([]byte, 0x80)
	binary.BigEndian.PutUint32(bcem[0x44:], uint32(len(vol)/512))
	binary.BigEndian.PutUint32(bcem[0x7C:], uint32(len(sectors)+1))
	for i, s := range sectors {
		end := len(data)
		if i+1 < len(sectors) {
			end = sectors[i+1][2]
		}
		e := []byte{byte(s[0] >> 16), byte(s[0] >> 8), byte(s[0]), byte(kinds[i])}
		e = binary.BigEndian.AppendUint32(e, uint32(s[2]))
		e = binary.BigEndian.AppendUint32(e, uint32(end-s[2]))
		bcem = append(bcem, e...)
	}
	bcem = append(bcem, byte(len(vol)/512>>16), byte(len(vol)/512>>8), byte(len(vol)/512), 0xFF, 0, 0, 0, 0, 0, 0, 0, 0)
	vs, err := OpenWithResources(bytes.NewReader(data), int64(len(data)), rsrcFork("bcem", 128, bcem))
	check(t, vs, err, false)
}

// TestUDIF opens an HFS+ volume on a disk with a GUID partition table, in
// a .dmg image.
func TestUDIF(t *testing.T) {
	vol := hfstest.Build(true, testFiles)
	le := binary.LittleEndian
	disk := make([]byte, 34*512)
	copy(disk[512:], "EFI PART")
	le.PutUint64(disk[512+72:], 2)
	le.PutUint32(disk[512+80:], 1)
	le.PutUint32(disk[512+84:], 128)
	le.PutUint64(disk[1024+32:], 34)
	le.PutUint64(disk[1024+40:], uint64(34+len(vol)/512-1))
	disk = append(disk, vol...)

	sectors, data, kinds := chunked(disk, func(i int, b []byte) (uint32, []byte) {
		if i%2 == 0 {
			return 0x00000001, b
		}
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		w.Write(b)
		w.Close()
		return 0x80000005, z.Bytes()
	})
	mish := make([]byte, 204)
	copy(mish, "mish")
	binary.BigEndian.PutUint32(mish[200:], uint32(len(sectors)+1))
	for i, s := range sectors {
		end := len(data)
		if i+1 < len(sectors) {
			end = sectors[i+1][2]
		}
		e := binary.BigEndian.AppendUint32(nil, kinds[i])
		e = append(e, 0, 0, 0, 0)
		e = binary.BigEndian.AppendUint64(e, uint64(s[0]))
		e = binary.BigEndian.AppendUint64(e, uint64(s[1]))
		e = binary.BigEndian.AppendUint64(e, uint64(s[2]))
		e = binary.BigEndian.AppendUint64(e, uint64(end-s[2]))
		mish = append(mish, e...)
	}
	mish = append(mish, 0xFF, 0xFF, 0xFF, 0xFF)
	mish = append(mish, make([]byte, 36)...)
	plist := []byte("<plist><dict><key>resource-fork</key><dict><key>blkx</key><array><dict><key>Data</key><data>\n" +
		base64.StdEncoding.EncodeToString(mish) + "\n</data></dict></array></dict></dict></plist>")
	img := append(data, plist...)
	koly := make([]byte, 512)
	copy(koly, "koly")
	binary.BigEndian.PutUint64(koly[216:], uint64(len(data)))
	binary.BigEndian.PutUint64(koly[224:], uint64(len(plist)))
	img = append(img, koly...)
	vs, err := Open(bytes.NewReader(img), int64(len(img)))
	check(t, vs, err, true)
}

// FuzzNDIF checks that a damaged Disk Copy 6 layout returns an error, not
// a panic or a huge allocation.
func FuzzNDIF(f *testing.F) {
	vol := hfstest.Build(false, testFiles)
	bcem := make([]byte, 0x80+24)
	binary.BigEndian.PutUint32(bcem[0x44:], uint32(len(vol)/512))
	binary.BigEndian.PutUint32(bcem[0x7C:], 2)
	copy(bcem[0x80:], []byte{0, 0, 0, 2, 0, 0, 0, 0})
	binary.BigEndian.PutUint32(bcem[0x88:], uint32(len(vol)))
	bcem[0x80+12+3] = 0xFF
	f.Add(bcem)
	f.Fuzz(func(t *testing.T, bcem []byte) {
		vs, err := OpenWithResources(bytes.NewReader(vol), int64(len(vol)), rsrcFork("bcem", 128, bcem))
		if err != nil {
			return
		}
		for _, v := range vs {
			for i, fl := range v.Files(nil) {
				if i == 50 {
					break
				}
				v.ReadData(fl)
			}
		}
	})
}

// TestStraySignature opens files that have a volume's signature at 1024 by
// chance, as compiled scripts can: they aren't images.
func TestStraySignature(t *testing.T) {
	for _, sig := range []string{"H+\x00\x00", "HX\x00\x00", "BD\x00\x00"} {
		data := make([]byte, 4096)
		copy(data, "FasdUAS 1.101.10")
		copy(data[1024:], sig)
		if _, err := Open(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrNotImage) {
			t.Errorf("%q: %v", sig[:2], err)
		}
	}
}
