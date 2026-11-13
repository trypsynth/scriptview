package classic

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "classic", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestUnwrap reads the same resource fork from each wrapper.
func TestUnwrap(t *testing.T) {
	want := fixture(t, "handlers2.rsrc")
	for name, wantName := range map[string]string{
		"handlers2.appledouble": "",
		"handlers2.bin":         "Classic Script",
		"handlers2.hqx":         "Classic Script",
	} {
		f, ok := Unwrap(fixture(t, name))
		if !ok {
			t.Fatalf("%s: not unwrapped", name)
		}
		if f.Name != wantName || string(f.Rsrc) != string(want) || len(f.Data) != 0 {
			t.Errorf("%s: name %q, %d data bytes, resource fork matches: %v", name, f.Name, len(f.Data), string(f.Rsrc) == string(want))
		}
	}
	if _, ok := Unwrap([]byte("FasdUAS 1.101.10")); ok {
		t.Error("a plain file unwrapped")
	}
}

// TestFinderInfo reads the type and creator that MacBinary records.
func TestFinderInfo(t *testing.T) {
	data := fixture(t, "handlers2.bin")
	copy(data[65:], "osasToyS")
	f, ok := Unwrap(data)
	if !ok || f.Type != "osas" || f.Creator != "ToyS" {
		t.Errorf("got type %q, creator %q", f.Type, f.Creator)
	}
}

func TestResources(t *testing.T) {
	res := Resources(fixture(t, "handlers2.rsrc"), "scpt")
	if len(res) != 1 || len(res[128]) == 0 {
		t.Errorf("got %d scpt resources", len(res))
	}
	if len(Resources([]byte("short"), "scpt")) != 0 {
		t.Error("a short fork has resources")
	}
}

// TestReadFile finds the resource fork in an AppleDouble file next to the
// file, and in a __MACOSX folder.
func TestReadFile(t *testing.T) {
	want := fixture(t, "handlers2.rsrc")
	double := fixture(t, "handlers2.appledouble")
	dir := t.TempDir()
	for _, p := range []struct{ file, double string }{
		{"a.scpt", "._a.scpt"},
		{"sub/b.scpt", "__MACOSX/sub/._b.scpt"},
	} {
		for _, f := range []string{p.file, p.double} {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, p.file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, p.double), double, 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := ReadFile(filepath.Join(dir, p.file))
		if err != nil || string(f.Rsrc) != string(want) {
			t.Errorf("%s: err %v, resource fork found: %v", p.file, err, string(f.Rsrc) == string(want))
		}
	}
}

// TestResourceList reads a resource fork with one named resource.
func TestResourceList(t *testing.T) {
	be := binary.BigEndian
	data := []byte("\x00\x00\x00\x03abc")         // the resource's data, with its length
	typeList := be.AppendUint16(nil, 0)           // one type
	typeList = append(typeList, "XCMD"...)        // its type
	typeList = be.AppendUint16(typeList, 0)       // one resource
	typeList = be.AppendUint16(typeList, 10)      // its reference list, from the type list
	ref := be.AppendUint16(nil, uint16(0x8000|5)) // ID -32763
	ref = be.AppendUint16(ref, 0)                 // name at 0 in the name list
	ref = be.AppendUint32(ref, 0)                 // data at 0
	ref = append(ref, 0, 0, 0, 0)
	names := append([]byte{6}, "AESend"...)
	resMap := make([]byte, 28)
	be.PutUint16(resMap[24:], 28)                                // type list
	be.PutUint16(resMap[26:], uint16(28+len(typeList)+len(ref))) // name list
	resMap = append(append(append(resMap, typeList...), ref...), names...)
	fork := make([]byte, 16)
	be.PutUint32(fork, 16)
	be.PutUint32(fork[4:], uint32(16+len(data)))
	fork = append(append(fork, data...), resMap...)

	res := ResourceList(fork)
	if len(res) != 1 || res[0].Type != "XCMD" || res[0].ID != -32763 || res[0].Name != "AESend" || string(res[0].Data) != "abc" {
		t.Errorf("got %+v", res)
	}
}
