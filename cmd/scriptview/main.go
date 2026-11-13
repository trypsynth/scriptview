package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/trypsynth/scriptview/classic"
	"github.com/trypsynth/scriptview/scpt"
)

const helpText = `Usage: scriptview [command] <file.scpt | bundle.scptd | applet.app>

With no command, scriptview decompiles the file.

Commands:
  decompile   print the reconstructed AppleScript source
  tree        print the raw syntax node graph (for format debugging)
  disasm      print the compiled bytecode as annotated assembly
  bytecode    decompile from the bytecode alone (as for run-only scripts)
  version     print the version
  help        print this help
`

// version is set at build time for releases.
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 1 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Println("scriptview", version)
			return
		case "help", "--help", "-h":
			fmt.Print(helpText)
			return
		}
		args = []string{"decompile", args[0]}
	}
	if len(args) != 2 {
		fmt.Fprint(os.Stderr, helpText)
		os.Exit(2)
	}

	cmd, path := args[0], args[1]

	switch cmd {
	case "decompile", "tree", "disasm", "bytecode":
	default:
		fmt.Fprintf(os.Stderr, "scriptview: unknown command %q\n\n%s", cmd, helpText)
		os.Exit(2)
	}

	data, err := readScript(scriptPath(path))
	if err == nil && len(data) == 0 {
		err = fmt.Errorf("%s is empty, and has no resource fork or ._ file with a script in it", path)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
		os.Exit(1)
	}

	f, err := scpt.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "decompile":
		src := scpt.Decompile(f)
		fmt.Print(src)
		if len(src) > 0 && src[len(src)-1] != '\n' {
			fmt.Println()
		}
	case "bytecode":
		src, err := scpt.DecompileBytecode(f)
		fmt.Print(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
			os.Exit(1)
		}
	case "disasm":
		fmt.Print(scpt.Disassemble(f))
	case "tree":
		fmt.Print(scpt.Tree(f))
	}
}

// scriptPath maps a script bundle or applet directory to its main script.
func scriptPath(path string) string {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return filepath.Join(path, "Contents", "Resources", "Scripts", "main.scpt")
	}
	return path
}

// readScript reads a script file. A classic Mac script keeps its compiled
// data in the resource fork and leaves the data fork empty: then read the
// resource fork.
func readScript(path string) ([]byte, error) {
	f, err := classic.ReadFile(path)
	if err != nil || len(f.Data) > 0 {
		return f.Data, err
	}
	return f.Rsrc, nil
}
