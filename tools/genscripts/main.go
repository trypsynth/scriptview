//go:build darwin

// genscripts compiles AppleScript source files into .scpt test fixtures and
// generates their expected osadecompile output.
//
// Run from the repo root:
//
//	go run ./tools/genscripts
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	srcDir := filepath.Join("testdata", "scripts")
	scptDir := filepath.Join("testdata", "scpt")
	expectedDir := filepath.Join("testdata", "expected")

	for _, dir := range []string{scptDir, expectedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		log.Fatalf("reading %s: %v", srcDir, err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".applescript") {
			continue
		}

		base := strings.TrimSuffix(e.Name(), ".applescript")
		srcPath := filepath.Join(srcDir, e.Name())
		scptPath := filepath.Join(scptDir, base+".scpt")
		expectedPath := filepath.Join(expectedDir, base+".txt")

		if err := compile(srcPath, scptPath); err != nil {
			log.Printf("compile %s: %v (skipping)", e.Name(), err)
			continue
		}

		if err := decompile(scptPath, expectedPath); err != nil {
			log.Printf("decompile %s: %v (skipping)", scptPath, err)
			continue
		}

		fmt.Printf("generated: %s → %s + %s\n", srcPath, scptPath, expectedPath)
	}
}

func compile(src, dst string) error {
	out, err := exec.Command("osacompile", "-o", dst, src).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

func decompile(scpt, dst string) error {
	out, err := exec.Command("osadecompile", scpt).Output()
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	src := strings.TrimRight(string(out), "\n")
	return os.WriteFile(dst, []byte(src), 0o644)
}
