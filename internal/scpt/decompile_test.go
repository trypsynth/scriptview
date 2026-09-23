package scpt

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDecompileGolden checks every fixture against the osadecompile output
// recorded in testdata/expected (generated on macOS by tools/genscripts).
func TestDecompileGolden(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/scpt/*.scpt")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".scpt")
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("../../testdata/expected", name+".txt"))
			if err != nil {
				t.Skip("no expected output")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			got := Decompile(f)
			if normalize(got) != normalize(string(want)) {
				t.Errorf("mismatch\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

// openSpace matches spaces after an opening brace or bracket, which the
// source may contain but the bytecode does not record.
var openSpace = regexp.MustCompile(`([{\[]) +`)

// normalize drops blank lines and trailing whitespace and trims spaces after
// "{"/"["; neither is recoverable from the bytecode.
func normalize(s string) string {
	s = openSpace.ReplaceAllString(s, "$1")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimRight(line, " \t"); line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// TestStrictParse checks that every fixture decodes as a clean record
// sequence, without falling back to the tolerant scanner.
func TestStrictParse(t *testing.T) {
	paths, _ := filepath.Glob("../../testdata/scpt/*.scpt")
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(data, Magic) {
			continue // JXA / plain text
		}
		body := data[len(Magic) : len(data)-trailerLen(data)]
		if err := parseRecords(body, newBuilder()); err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)
		}
	}
}
