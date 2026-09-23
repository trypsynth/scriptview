package scpt

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzDecompile feeds mutated scripts through every entry point; none may
// panic, whatever the input.
func FuzzDecompile(f *testing.F) {
	for _, dir := range []string{"../../testdata/scpt", "../../testdata/runonly"} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.scpt"))
		for _, p := range paths {
			if data, err := os.ReadFile(p); err == nil && len(data) < 20000 {
				f.Add(data)
			}
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Parse(data)
		if err != nil {
			return
		}
		_ = Decompile(file)
		_, _ = DecompileBytecode(file)
		_ = Disassemble(file)
		_ = Tree(file)
		_ = BestEffortSource(file)
	})
}
