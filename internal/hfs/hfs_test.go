package hfs

import (
	"bytes"
	"encoding/binary"
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
