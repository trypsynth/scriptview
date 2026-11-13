package scpt

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestBytecodeCorpus validates instruction decoding over the files listed in
// $SCRIPTVIEW_CORPUS (one path per line): every handler must decode to its
// end with every branch landing on an instruction boundary.
func TestBytecodeCorpus(t *testing.T) {
	list := os.Getenv("SCRIPTVIEW_CORPUS")
	if list == "" {
		t.Skip()
	}
	fh, _ := os.Open(list)
	sc := bufio.NewScanner(fh)
	errs := map[string]int{}
	examples := map[string]string{}
	files, handlers, bad := 0, 0, 0
	for sc.Scan() {
		data, err := os.ReadFile(sc.Text())
		if err != nil {
			continue
		}
		f, err := Parse(data)
		if err != nil || f.dc == nil || f.dc.objects == nil {
			continue
		}
		files++
		for _, h := range f.dc.allHandlers() {
			handlers++
			prog, err := disassemble(h.code)
			key := ""
			if err != nil {
				last := "start"
				if len(prog) > 0 {
					last = prog[len(prog)-1].name
				}
				key = "decode after " + last + ": " + err.Error()[:min(len(err.Error()), 20)]
			} else {
				starts := map[int]bool{len(h.code): true}
				for _, in := range prog {
					starts[in.off] = true
				}
				for _, in := range prog {
					if in.isBranch() && len(in.args) > 0 && !starts[in.jumpTarget(in.args[len(in.args)-1])] {
						key = "bad target " + in.name
						break
					}
				}
			}
			if key != "" {
				bad++
				errs[key]++
				examples[key] = sc.Text()
			}
		}
	}
	t.Logf("files %d, handlers %d, bad %d", files, handlers, bad)
	var keys []string
	for k := range errs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return errs[keys[i]] > errs[keys[j]] })
	for _, k := range keys[:min(len(keys), 25)] {
		t.Errorf("%d× %s (e.g. %s)", errs[k], k, examples[k])
	}
}

// TestBytecodeFixtures checks that every fixture's bytecode decodes cleanly.
func TestBytecodeFixtures(t *testing.T) {
	paths, _ := filepath.Glob("../../testdata/scpt/*.scpt")
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := Parse(data)
		if err != nil || f.dc == nil || f.dc.objects == nil {
			continue
		}
		handlers := f.dc.handlerTable()
		if len(handlers) == 0 {
			t.Errorf("%s: no handlers found", filepath.Base(path))
		}
		for _, h := range handlers {
			if _, err := disassemble(h.code); err != nil {
				t.Errorf("%s: %v", filepath.Base(path), err)
			}
		}
	}
}

// TestRunOnlyGolden decompiles run-only fixtures (compiled with
// `osacompile -x`, so they carry bytecode but no syntax tree) and compares
// with their recorded output.
func TestRunOnlyGolden(t *testing.T) {
	paths, _ := filepath.Glob("../../testdata/runonly/*.scpt")
	if len(paths) == 0 {
		t.Fatal("no run-only fixtures")
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".scpt")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			if !f.RunOnly() {
				t.Fatal("fixture is not run-only")
			}
			want, err := os.ReadFile(strings.TrimSuffix(path, ".scpt") + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			if got := Decompile(f); got != string(want) {
				t.Errorf("mismatch\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

// TestRunOnlyRoundTrip compiles each run-only fixture's decompiled source
// again with osacompile (macOS only) and checks that the bytecode matches.
func TestRunOnlyRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("osacompile"); err != nil {
		t.Skip("osacompile not available")
	}
	paths, _ := filepath.Glob("../../testdata/runonly/*.scpt")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			orig := mustParse(t, path)
			src, err := DecompileBytecode(orig)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			in, out := filepath.Join(dir, "src.applescript"), filepath.Join(dir, "out.scpt")
			if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			if msg, err := exec.Command("osacompile", "-o", out, in).CombinedOutput(); err != nil {
				t.Fatalf("recompile: %v\n%s", err, msg)
			}
			if got, want := Disassemble(mustParse(t, out)), Disassemble(orig); got != want {
				t.Errorf("bytecode differs after a round trip\n--- source\n%s", src)
			}
		})
	}
}

func mustParse(t *testing.T, path string) *File {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
