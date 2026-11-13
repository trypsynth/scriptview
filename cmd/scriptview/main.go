package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/trypsynth/scriptview/classic"
	"github.com/trypsynth/scriptview/hypercard"
	"github.com/trypsynth/scriptview/scpt"
)

const helpText = `Usage: scriptview [command] <file>

The file can be a compiled AppleScript (.scpt), a script bundle (.scptd),
an applet (.app), or a HyperCard stack. With no command, scriptview prints
the file's source: the decompiled AppleScript, or every script in the
stack.

Commands:
  decompile   print the reconstructed AppleScript source
  tree        print the raw syntax node graph (for format debugging)
  disasm      print the compiled bytecode as annotated assembly
  bytecode    decompile from the bytecode alone (as for run-only scripts)
  text        print the text of a stack's fields
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
	case "decompile", "tree", "disasm", "bytecode", "text":
	default:
		fmt.Fprintf(os.Stderr, "scriptview: unknown command %q\n\n%s", cmd, helpText)
		os.Exit(2)
	}

	file, err := classic.ReadFile(scriptPath(path))
	// A classic Mac script keeps its compiled data in the resource fork and
	// leaves the data fork empty.
	data := file.Data
	if len(data) == 0 {
		data = file.Rsrc
	}
	if err == nil && len(data) == 0 {
		err = fmt.Errorf("%s is empty, and has no resource fork or ._ file with a script in it", path)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
		os.Exit(1)
	}

	if stack, ok := stackFile(file); ok {
		runStack(cmd, stack)
		return
	}
	if cmd == "text" {
		fmt.Fprintln(os.Stderr, "scriptview: text works only on HyperCard stacks")
		os.Exit(2)
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

// stackFile returns f as a HyperCard stack, unwrapping MacBinary, BinHex,
// and AppleSingle files, or false if it isn't one.
func stackFile(f classic.File) (classic.File, bool) {
	if hypercard.IsStack(f.Data) {
		return f, true
	}
	if inner, ok := classic.Unwrap(f.Data); ok && hypercard.IsStack(inner.Data) {
		return inner, true
	}
	return f, false
}

// runStack prints a HyperCard stack's scripts, or with the text command,
// the text of its fields.
func runStack(cmd string, f classic.File) {
	s, err := hypercard.Parse(f.Data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
		os.Exit(1)
	}
	s.AddResources(f.Rsrc)
	switch cmd {
	case "decompile":
		fmt.Print(s.Scripts())
	case "text":
		fmt.Print(s.Text())
	default:
		fmt.Fprintf(os.Stderr, "scriptview: %s works only on compiled AppleScript files\n", cmd)
		os.Exit(2)
	}
}
