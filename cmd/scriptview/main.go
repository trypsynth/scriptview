package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/trypsynth/scriptview/internal/classic"
	"github.com/trypsynth/scriptview/internal/hfs"
	"github.com/trypsynth/scriptview/internal/hypercard"
	"github.com/trypsynth/scriptview/internal/mactext"
	"github.com/trypsynth/scriptview/internal/scpt"
)

const helpText = `Usage: scriptview [command] [options] <file or folder>...

A file can be a compiled AppleScript (.scpt), a script bundle (.scptd),
an applet (.app), or a HyperCard stack. With no command, scriptview prints
the file's source: the decompiled AppleScript, or every script in the
stack. Given a folder or a classic Mac disk image (HFS or HFS+, such as a
.dsk, .img, .toast, or .iso file), it finds every script and stack inside,
and prints each one after a line with its path.

Commands:
  decompile   print the reconstructed AppleScript source
  tree        print the raw syntax node graph (for format debugging)
  disasm      print the compiled bytecode as annotated assembly
  bytecode    decompile from the bytecode alone (as for run-only scripts)
  text        print the text of a stack's fields
  encodings   list the text encodings that --encoding takes
  version     print the version
  help        print this help

Options:
  -e, --encoding NAME   decode the text of classic Mac OS files in this
                        encoding, such as japanese, instead of the one
                        scriptview detects
  -o, --output DIR      write each result to a file in DIR, named after the
                        input, instead of printing it
`

// version is set at build time for releases.
var version = "dev"

var commands = []string{"decompile", "tree", "disasm", "bytecode", "text"}

// options are the command line's options.
type options struct {
	cmd    string
	enc    *mactext.Encoding
	outDir string
}

// errSkip marks a file that the command doesn't apply to, such as a script
// for the text command. A folder's walk passes over such files quietly.
var errSkip = errors.New("skip")

// usageError is an error in how scriptview was called.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func main() {
	opts, paths, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\nRun scriptview help for usage.\n", err)
		os.Exit(2)
	}
	switch opts.cmd {
	case "version":
		fmt.Println("scriptview", version)
		return
	case "help":
		fmt.Print(helpText)
		return
	case "encodings":
		for _, e := range mactext.All {
			fmt.Printf("%-20s %s\n", e.Short, e.Name)
		}
		return
	}
	if len(paths) == 0 {
		fmt.Fprint(os.Stderr, helpText)
		os.Exit(2)
	}
	os.Exit(run(opts, paths))
}

// parseArgs reads the command, options, and paths, in any order, with the
// command first among the words that aren't options.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := strings.Cut(a, "=")
		switch name {
		case "-e", "--encoding", "-o", "--output":
			if !hasVal {
				if i+1 >= len(args) {
					return opts, nil, fmt.Errorf("%s needs a value", a)
				}
				i++
				val = args[i]
			}
			if name == "-e" || name == "--encoding" {
				if opts.enc = mactext.ByName(val); opts.enc == nil {
					return opts, nil, fmt.Errorf("unknown encoding %q (see scriptview encodings)", val)
				}
			} else {
				opts.outDir = val
			}
		case "-h", "--help":
			opts.cmd = "help"
		case "-v", "--version":
			opts.cmd = "version"
		case "--":
			words = append(words, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return opts, nil, fmt.Errorf("unknown option %q", a)
			}
			words = append(words, a)
		}
	}
	if opts.cmd != "" {
		return opts, nil, nil
	}
	if len(words) > 0 {
		switch w := words[0]; {
		case slices.Contains(commands, w):
			opts.cmd, words = w, words[1:]
		case len(words) == 1 && (w == "help" || w == "version" || w == "encodings"):
			opts.cmd = w
			return opts, nil, nil
		}
	}
	if opts.cmd == "" {
		opts.cmd = "decompile"
	}
	return opts, words, nil
}

// run runs the command on each path, and returns the exit status.
func run(opts options, paths []string) int {
	// One file prints as it always has. Several files, or a folder, print
	// each result after a line with its path.
	several := len(paths) > 1
	status := 0
	for _, path := range paths {
		if fi, err := os.Stat(path); err == nil && fi.IsDir() && !isBundle(path) {
			several = true
			if s := walk(opts, path); s > status {
				status = s
			}
			continue
		}
		if vols, closer, err := openImage(path); vols != nil || err != nil {
			if err != nil {
				fmt.Fprintf(os.Stderr, "scriptview: %s: %v\n", path, err)
				status = max(status, 1)
				continue
			}
			several = true
			status = max(status, walkImage(opts, path, filepath.Base(path), vols, 0))
			closer.Close()
			continue
		}
		out, ext, err := runFile(opts, path, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
			if errors.As(err, new(usageError)) {
				status = max(status, 2)
			} else {
				status = max(status, 1)
			}
			if out == "" {
				continue
			}
		}
		if err := emit(opts, path, filepath.Base(bundleName(path)), out, ext, several); err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
			status = max(status, 1)
		}
	}
	return status
}

// walk runs the command on every script and stack in a folder.
func walk(opts options, root string) int {
	status := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
			status = 1
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "__MACOSX" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		// AppleDouble files (._name) carry resource forks, which
		// classic.ReadFile finds from the file they belong to.
		if !d.Type().IsRegular() || strings.HasPrefix(name, "._") || name == ".DS_Store" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.Join(filepath.Base(root), rel)
		if vols, closer, err := openImage(path); vols != nil {
			status = max(status, walkImage(opts, path, rel, vols, 0))
			closer.Close()
			return nil
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %s: %v\n", path, err)
			status = 1
			return nil
		}
		if fi, err := d.Info(); err != nil || fi.Size() > maxSize {
			return nil
		}
		out, ext, err := runFile(opts, path, false)
		if errors.Is(err, errSkip) {
			return nil
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %s: %v\n", path, err)
			status = 1
			if out == "" {
				return nil
			}
		}
		if err := emit(opts, path, rel, out, ext, true); err != nil {
			fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
			status = 1
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
		status = 1
	}
	return status
}

// maxSize is the largest file or fork that a walk reads. Scripts and
// stacks are far smaller.
const maxSize = 64 << 20

// openImage opens path as a disk image, or returns no volumes if it isn't
// one.
func openImage(path string) ([]*hfs.Volume, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil // runFile reports it
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, nil
	}
	vols, err := hfs.Open(f, fi.Size())
	if err != nil {
		f.Close()
		if errors.Is(err, hfs.ErrNotImage) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return vols, f, nil
}

// maxDepth is how deep a walk opens images inside images.
const maxDepth = 3

// walkImage runs the command on every script and stack in a disk image's
// volumes. path names the image in headers, and rel in the output folder.
func walkImage(opts options, path, rel string, vols []*hfs.Volume, depth int) int {
	status := 0
	for _, v := range vols {
		for _, f := range v.Files(opts.enc) {
			if f.DataSize() > maxSize || f.RsrcSize() > maxSize {
				continue
			}
			// Mac names can hold a slash, which the Finder shows as a
			// colon in a path.
			shown := []string{v.Name}
			frel := []string{rel, fileName(v.Name)}
			for _, n := range f.Path {
				shown = append(shown, strings.ReplaceAll(n, "/", ":"))
				frel = append(frel, fileName(n))
			}
			fpath := path + "/" + strings.Join(shown, "/")
			data, err := v.ReadData(f)
			if err == nil {
				var rsrc []byte
				rsrc, err = v.ReadRsrc(f)
				if err == nil {
					status = max(status, runInImage(opts, classic.File{Name: f.Path[len(f.Path)-1], Type: f.Type, Creator: f.Creator, Data: data, Rsrc: rsrc}, fpath, filepath.Join(frel...), depth))
					continue
				}
			}
			fmt.Fprintf(os.Stderr, "scriptview: %s: %v\n", fpath, err)
			status = 1
		}
	}
	return status
}

// fileName makes a Mac file name one that every system takes: Mac names
// can hold characters that Windows doesn't allow, and end in spaces or
// dots, which Windows drops.
func fileName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, n)
	if t := strings.TrimRight(n, " ."); t != n {
		n = t + strings.Repeat("_", len(n)-len(t))
	}
	if n == "" {
		return "_"
	}
	return n
}

// runInImage runs the command on a file from a disk image, opening it in
// turn if it's an image itself.
func runInImage(opts options, f classic.File, path, rel string, depth int) int {
	if depth < maxDepth {
		if vols, err := hfs.Open(bytes.NewReader(f.Data), int64(len(f.Data))); err == nil {
			return walkImage(opts, path, rel, vols, depth+1)
		}
	}
	out, ext, err := runClassic(opts, f, path, false)
	if errors.Is(err, errSkip) {
		return 0
	}
	status := 0
	if err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %s: %v\n", path, err)
		status = 1
		if out == "" {
			return status
		}
	}
	if err := emit(opts, path, rel, out, ext, true); err != nil {
		fmt.Fprintf(os.Stderr, "scriptview: %v\n", err)
		status = 1
	}
	return status
}

// emit prints a result, or writes it to the output folder as rel plus
// ext. With header set, a printed result follows a line with its path.
func emit(opts options, path, rel, out, ext string, header bool) error {
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if opts.outDir != "" {
		// a.scpt becomes a.applescript, not a.scpt.applescript.
		if e := filepath.Ext(rel); e == ".scpt" || e == ".scptd" || e == ".app" {
			rel = strings.TrimSuffix(rel, e)
		}
		dst := filepath.Join(opts.outDir, rel+ext)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(out), 0o644)
	}
	if header {
		fmt.Printf("==> %s <==\n", path)
		defer fmt.Println()
	}
	fmt.Print(out)
	return nil
}

// runFile runs the command on one file, and returns its output and the
// file extension for it. With explicit set, the user named the file, so a
// plain-text file named .scpt decompiles to itself, and a file the command
// doesn't apply to is a usage error, not errSkip.
func runFile(opts options, path string, explicit bool) (out, ext string, err error) {
	file, err := classic.ReadFile(scriptPath(path))
	if err != nil {
		return "", "", err
	}
	return runClassic(opts, file, path, explicit)
}

// runClassic runs the command on a file's forks, like runFile.
func runClassic(opts options, file classic.File, path string, explicit bool) (out, ext string, err error) {
	if stack, ok := stackFile(file); ok {
		return runStack(opts, stack, explicit)
	}
	// A classic Mac script keeps its compiled data in the resource fork and
	// leaves the data fork empty.
	data := file.Data
	if !scpt.IsCompiled(data) && scpt.IsCompiled(file.Rsrc) || len(data) == 0 {
		data = file.Rsrc
	}
	if !explicit && !scpt.IsCompiled(data) {
		return "", "", errSkip
	}
	if len(data) == 0 {
		return "", "", fmt.Errorf("%s is empty, and has no resource fork or ._ file with a script in it", path)
	}
	if opts.cmd == "text" {
		return "", "", notFor(explicit, "text works only on HyperCard stacks")
	}
	f, err := scpt.ParseWith(data, scpt.Options{Encoding: opts.enc})
	if err != nil {
		return "", "", err
	}
	ext = ".applescript"
	if bytes.HasPrefix(data, scpt.JXAMagic) {
		ext = ".js"
	}
	switch opts.cmd {
	case "decompile":
		return scpt.Decompile(f), ext, nil
	case "bytecode":
		src, err := scpt.DecompileBytecode(f)
		return src, ext, err
	case "disasm":
		return scpt.Disassemble(f), ".disasm.txt", nil
	default: // tree
		return scpt.Tree(f), ".tree.txt", nil
	}
}

// notFor is the error for a file that a command doesn't apply to.
func notFor(explicit bool, msg string) error {
	if explicit {
		return usageError{msg}
	}
	return errSkip
}

// isBundle reports whether a folder is a script bundle or applet, which
// scriptview reads as its main script.
func isBundle(path string) bool {
	_, err := os.Stat(filepath.Join(path, "Contents", "Resources", "Scripts", "main.scpt"))
	return err == nil
}

// scriptPath maps a script bundle or applet folder to its main script.
func scriptPath(path string) string {
	if isBundle(path) {
		return filepath.Join(path, "Contents", "Resources", "Scripts", "main.scpt")
	}
	return path
}

// bundleName returns path without a trailing slash, so that a bundle's
// output is named after the bundle.
func bundleName(path string) string {
	return strings.TrimRight(path, string(filepath.Separator))
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

// runStack runs the command on a HyperCard stack: its scripts, or with
// the text command, the text of its fields.
func runStack(opts options, f classic.File, explicit bool) (out, ext string, err error) {
	switch opts.cmd {
	case "decompile", "text":
	default:
		return "", "", notFor(explicit, opts.cmd+" works only on compiled AppleScript files")
	}
	s, err := hypercard.ParseWith(f.Data, opts.enc)
	if err != nil {
		return "", "", err
	}
	s.AddResources(f.Rsrc)
	if opts.cmd == "text" {
		return s.Text(), ".text.txt", nil
	}
	return s.Scripts(), ".hypertalk", nil
}
