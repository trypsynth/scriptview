package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/trypsynth/scriptview/internal/hfs/hfstest"
	"github.com/trypsynth/scriptview/internal/mactext"
)

func TestParseArgs(t *testing.T) {
	for _, c := range []struct {
		args  []string
		cmd   string
		enc   *mactext.Encoding
		out   string
		paths []string
	}{
		{[]string{"a.scpt"}, "decompile", nil, "", []string{"a.scpt"}},
		{[]string{"tree", "a.scpt", "b.scpt"}, "tree", nil, "", []string{"a.scpt", "b.scpt"}},
		{[]string{"-e", "japanese", "a.scpt"}, "decompile", mactext.Japanese, "", []string{"a.scpt"}},
		{[]string{"text", "x", "--encoding=korean", "-o", "out"}, "text", mactext.Korean, "out", []string{"x"}},
		{[]string{"--", "-e"}, "decompile", nil, "", []string{"-e"}},
		{[]string{"help"}, "help", nil, "", nil},
		{[]string{"-h"}, "help", nil, "", nil},
		{[]string{"version"}, "version", nil, "", nil},
		{[]string{"encodings"}, "encodings", nil, "", nil},
	} {
		opts, paths, err := parseArgs(c.args)
		if err != nil {
			t.Errorf("parseArgs(%q): %v", c.args, err)
			continue
		}
		if opts.cmd != c.cmd || opts.enc != c.enc || opts.outDir != c.out || !slices.Equal(paths, c.paths) {
			t.Errorf("parseArgs(%q) = %+v, %q", c.args, opts, paths)
		}
	}
	for _, args := range [][]string{{"-e"}, {"-e", "klingon", "a"}, {"--bogus", "a"}} {
		if _, _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%q) succeeded", args)
		}
	}
}

// TestWalk decompiles a folder into an output folder: scripts are found
// whatever their names, and other files and AppleDouble files are passed
// over.
func TestWalk(t *testing.T) {
	script, err := os.ReadFile("../../testdata/scpt/closeWindows.scpt")
	if err != nil {
		t.Fatal(err)
	}
	in, out := t.TempDir(), t.TempDir()
	write := func(name string, data []byte) {
		p := filepath.Join(in, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("one.scpt", script)
	write("sub/Two", script)
	write("notes.txt", []byte("tell me something\n"))
	write("._notes.txt", []byte("junk"))
	write(".hidden/three.scpt", script)

	if s := run(options{cmd: "decompile", outDir: out}, []string{in}); s != 0 {
		t.Fatalf("status %d", s)
	}
	var got []string
	filepath.WalkDir(out, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(out, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	base := filepath.Base(in)
	want := []string{base + "/one.applescript", base + "/sub/Two.applescript"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	src, _ := os.ReadFile(filepath.Join(out, want[0]))
	if !strings.Contains(string(src), "tell application") {
		t.Errorf("output:\n%s", src)
	}
}

// TestImage decompiles the scripts on a disk image, given itself and
// found in a folder, and on an image inside it.
func TestImage(t *testing.T) {
	script, err := os.ReadFile("../../testdata/scpt/closeWindows.scpt")
	if err != nil {
		t.Fatal(err)
	}
	inner := hfstest.Build(true, []hfstest.File{{Name: "Inner", Type: "osas", Data: script}})
	img := hfstest.Build(false, []hfstest.File{
		{Folder: "Scripts", Name: "Close/Open", Type: "osas", Data: script},
		{Name: "ReadMe", Type: "TEXT", Data: []byte("hello")},
		{Name: "Nested.img", Type: "dImg", Data: inner},
	})
	in := t.TempDir()
	if err := os.WriteFile(filepath.Join(in, "disk.dsk"), img, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(in, "disk.dsk"), in} {
		out := t.TempDir()
		if s := run(options{cmd: "decompile", outDir: out}, []string{path}); s != 0 {
			t.Fatalf("%s: status %d", path, s)
		}
		var got []string
		filepath.WalkDir(out, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(out, p)
				got = append(got, filepath.ToSlash(rel))
			}
			return nil
		})
		prefix := "disk.dsk/Disk/"
		if path == in {
			prefix = filepath.Base(in) + "/" + prefix
		}
		want := []string{prefix + "Nested.img/Disk/Inner.applescript", prefix + "Scripts/Close_Open.applescript"}
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}
}

func TestFileName(t *testing.T) {
	for in, want := range map[string]string{"Home": "Home", "HyperCard ": "HyperCard_", "a/b:c?": "a_b_c_", "": "_", "Read Me…": "Read Me…"} {
		if got := fileName(in); got != want {
			t.Errorf("fileName(%q) = %q, want %q", in, got, want)
		}
	}
}
