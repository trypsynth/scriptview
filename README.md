# scriptview

scriptview shows you the source code of compiled AppleScript files (`.scpt`). It runs on macOS, Linux and Windows, so you can read a script without a Mac.

On a Mac you can do this with `osadecompile`. scriptview tries to print exactly what `osadecompile` prints, line for line.

## Install

Download a binary for macOS, Linux or Windows from the [releases page](https://github.com/trypsynth/scriptview/releases).

Or build it yourself with Go 1.26 or later:

```sh
go install github.com/trypsynth/scriptview/cmd/scriptview@latest
```

## Use

```sh
scriptview decompile MyScript.scpt
```

You can also give it a script bundle (`.scptd`) or an applet (`.app`). scriptview finds the compiled script inside.

Other commands:

| Command | What it prints |
| --- | --- |
| `decompile` | The source code |
| `bytecode` | The source code, rebuilt from the bytecode only |
| `disasm` | The bytecode as annotated assembly |
| `tree` | The raw syntax tree, for debugging |
| `dump` | Everything scriptview extracts, with notes |
| `strings`, `idents`, `ints`, `handlers` | String literals, identifier names, integers or handler names |

scriptview also reads JavaScript for Automation (JXA) scripts and `.scpt` files that hold plain text.

It reads classic Mac OS scripts too, back to AppleScript 1.0 in 1993. Current macOS refuses to open many of these ("data format obsolete"). Before Mac OS X, a compiled script kept its data in the file's resource fork. scriptview finds it in the resource fork itself (on a Mac), in the `._name` file or `__MACOSX` folder that a Mac makes when it copies or zips the file, and in MacBinary (`.bin`) and BinHex (`.hqx`) files.

## How well it works

We compare scriptview with `osadecompile` on two sets of scripts:

- The 194 compiled scripts that ship with macOS: all 194 match.
- 6,142 compiled scripts from about 1,370 public AppleScript repositories on GitHub: 6,133 match.

Most of the 9 that don't match depend on the Mac we tested on, such as which apps are installed there. scriptview includes the terminology of the apps that ship with macOS. For other apps it prints raw codes such as `«class pURL»`, which is what `osadecompile` does when it can't find the app.

When scriptview runs on a Mac, it checks whether a script's apps are installed, like `osadecompile` does. On other systems it shows those apps by their file name, for example `application "Firefox.app"`.

## Run-only scripts

A run-only script has no source code in it, only bytecode. scriptview decompiles the bytecode, so you can still read these scripts. Some things are lost when the script is compiled, and scriptview can't get them back:

- Comments.
- Line breaks (`¬`) and other formatting choices.
- Some choices of words, for example `is equal to` compared with `=`.
- `global` declarations and `using terms from` blocks.

scriptview adds a comment at the top of the output to tell you that it decompiled a run-only script.

## How it works

A compiled script is a serialized graph of objects. It holds a syntax tree, with flags that record how you wrote each part, and the bytecode that the AppleScript virtual machine runs. scriptview reads the syntax tree and prints it back as source. For run-only scripts it simulates the virtual machine's stack to rebuild a syntax tree from the bytecode.

The code is in `internal/scpt`:

- `objects.go` and `parse.go` read the object graph.
- `stmt.go`, `expr.go` and `scope.go` print the syntax tree.
- `bytecode.go` and the `bc*.go` files disassemble and decompile bytecode.
- `terms.tsv` holds the terminology of AppleScript, Standard Additions and the apps that ship with macOS. `tools/genterms` makes it from the dictionaries on a Mac.

## Notes

The [docs](docs/README.md) folder has what we learned about the format along the way: the object graph, the syntax tree, the bytecode, terminology, and the odd things `osadecompile` does.

## Tests

```sh
go test ./...
```

The tests compare scriptview's output with `osadecompile` output that we saved from a Mac. The test scripts are in `testdata/scripts`. To add a test, put a new `.applescript` file there and run this on a Mac:

```sh
go run ./tools/genscripts
```

This compiles each script into `testdata/scpt` and saves the `osadecompile` output in `testdata/expected`.

To compare scriptview with `osadecompile` on your own set of scripts, use `tools/corpus/compare.py`. The instructions are at the top of the file.

## How this was made

Claude, an AI model, wrote most of the code and the notes. A human directed the work, reviewed it, and pushed back when it was wrong.

## License

MIT. See [LICENSE](LICENSE).
