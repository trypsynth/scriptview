package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/trypsynth/scriptview/internal/scpt"
)

const helpText = `Usage: scriptview <command> <file.scpt | bundle.scptd | applet.app>

Commands:
  strings     print string literals found in the bytecode
  idents      print identifier names found in the bytecode
  ints        print integer literals found in the bytecode
  handlers    print handler (function) names found in the bytecode
  decompile   print the reconstructed AppleScript source
  dump        print all extracted content with annotations
  tree        print the raw syntax node graph (for format debugging)
  disasm      print the compiled bytecode as annotated assembly
  bytecode    decompile from the bytecode alone (as for run-only scripts)
  version     print the version
`

// version is set at build time for releases.
var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println("scriptview", version)
		return
	}
	if len(os.Args) != 3 {
		fmt.Fprint(os.Stderr, helpText)
		os.Exit(2)
	}

	cmd, path := os.Args[1], os.Args[2]

	switch cmd {
	case "strings", "idents", "ints", "handlers", "decompile", "dump", "tree", "disasm", "bytecode":
	default:
		fmt.Fprintf(os.Stderr, "scriptview: unknown command %q\n\n%s", cmd, helpText)
		os.Exit(2)
	}

	data, err := os.ReadFile(scriptPath(path))
	if err == nil && len(data) == 0 {
		err = fmt.Errorf("%s is empty (a cloud placeholder or a script whose data lives in the resource fork?)", path)
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
	case "strings":
		for _, s := range f.Strings {
			fmt.Println(s)
		}
	case "idents":
		for _, id := range f.Idents {
			fmt.Println(id)
		}
	case "ints":
		for _, v := range f.Ints {
			fmt.Println(v)
		}
	case "handlers":
		for _, h := range f.Handlers {
			fmt.Println(h.Name)
		}
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
	case "dump":
		src := scpt.BestEffortSource(f)
		fmt.Print(src)
		if len(src) > 0 && src[len(src)-1] != '\n' {
			fmt.Println()
		}
	}
}

// scriptPath maps a script bundle or applet directory to its main script.
func scriptPath(path string) string {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return filepath.Join(path, "Contents", "Resources", "Scripts", "main.scpt")
	}
	return path
}
