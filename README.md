# scriptview

scriptview shows you the source code of compiled AppleScript files (`.scpt`). It runs on macOS, Linux, and Windows, so you can read a script without a Mac.

On a Mac you can do this with `osadecompile`. scriptview tries to print exactly what `osadecompile` prints, line for line.

## Install

Download a binary for macOS, Linux, or Windows from the [releases page](https://github.com/trypsynth/scriptview/releases).

Or build it yourself with Go 1.26 or later:

```sh
go install github.com/trypsynth/scriptview/cmd/scriptview@latest
```

## Use

```sh
scriptview MyScript.scpt
```

This prints the script's source. It's the same as `scriptview decompile MyScript.scpt`.

You can also give it a script bundle (`.scptd`) or an applet (`.app`). scriptview finds the compiled script inside.

Other commands:

- `decompile`: the source code.
- `bytecode`: the source code, rebuilt from the bytecode only.
- `disasm`: the bytecode as annotated assembly.
- `tree`: the raw syntax tree, for debugging.

scriptview also reads JavaScript for Automation (JXA) scripts and `.scpt` files that hold plain text.

It reads classic Mac OS scripts too, back to AppleScript 1.0 in 1993, including Japanese ones. Current macOS refuses to open many of these ("data format obsolete"). Before Mac OS X, a compiled script kept its data in the file's resource fork. scriptview finds it in the resource fork itself (on a Mac), in the `._name` file or `__MACOSX` folder that a Mac makes when it copies or zips the file, and in MacBinary (`.bin`) and BinHex (`.hqx`) files.

## How well it works

scriptview is tested against `osadecompile` on two sets of scripts:

- The 163 compiled scripts that ship with macOS 26: all 163 match.
- 12,143 compiled scripts from 4,535 public AppleScript repositories on GitHub: 12,106 match.

Most of the 37 that don't match depend on the Mac that ran the test, such as which apps are installed there. scriptview includes the terminology of the apps that ship with macOS. For other apps it prints raw codes, such as `«class pURL»`, which is what `osadecompile` does when it can't find the app. For details, see [Testing](docs/testing.md).

When scriptview runs on a Mac, it checks whether a script's apps are installed, like `osadecompile` does. On other systems it shows those apps by their file name, for example `application "Firefox.app"`.

## Run-only scripts

A run-only script has no source code in it, only bytecode. scriptview decompiles the bytecode, so you can still read these scripts. Some things are lost when the script is compiled, and scriptview can't get them back:

- Comments.
- Line breaks (`¬`) and other formatting choices.
- Some choices of words, for example `is equal to` compared with `=`.
- Where a `using terms from` block ends, and where `global` and `use` statements were.

scriptview adds a comment at the top of the output to tell you that it decompiled a run-only script.

To test this, the decompiled source is compiled again, and the new bytecode is compared with the original. All 163 macOS scripts and 11,887 of the 12,150 GitHub scripts come back identical.

## Limitations

- Terms from apps that don't ship with macOS print as raw codes, such as `«class pURL»`, unless the app is installed and you're on a Mac. Scripts from classic Mac OS also get the terms of about 80 classic scripting additions and apps (see [Terminology](docs/terminology.md)).
- Run-only scripts lose their comments and formatting (see above).
- Run-only scripts from the AppleScript 1.0 beta (`FasdUAS 0.95`) print nothing, because the beta's bytecode isn't decoded.
- A value that was worked out when the script was compiled can't be turned back into the code that made it. For example, `property lib : load script …` comes out as the loaded script itself.

## How it works

A compiled script is a serialized graph of objects. It holds a syntax tree, with flags that record how you wrote each part, and the bytecode that the AppleScript virtual machine runs. scriptview reads the syntax tree and prints it back as source. For run-only scripts it simulates the virtual machine's stack to rebuild a syntax tree from the bytecode.

The code is in three packages, which other Go programs can import:

- `scpt` reads compiled AppleScript and JavaScript for Automation scripts. `Parse` reads a file, `Decompile` prints its source, `DecompileBytecode` rebuilds the source from the bytecode alone, and `Disassemble` lists the bytecode.
- `classic` reads the files of classic Mac OS: resource forks, and the AppleSingle, AppleDouble, MacBinary, and BinHex wrappers that carry them on other systems.
- `mactext` decodes the classic Mac text encodings, Mac Roman and Mac Japanese.

Inside `scpt`:

- `objects.go` and `parse.go` read the object graph.
- `stmt.go`, `expr.go`, and `scope.go` print the syntax tree.
- `bytecode.go` and the `bc*.go` files disassemble and decompile bytecode.
- `terms.tsv` holds the terminology of AppleScript, Standard Additions, and the apps that ship with macOS. `tools/genterms` makes it from the dictionaries on a Mac.

## Notes

The [docs](docs/README.md) folder is a reference for the format: the file layout, the syntax tree, every opcode of the bytecode, terminology, how often each structure occurs in real scripts, and the rules of `osadecompile`'s printer.

## Tests

```sh
go test ./...
```

The tests compare scriptview's output with `osadecompile` output saved on a Mac. The test scripts are in `testdata/scripts`. To add a test, put a new `.applescript` file there and run this on a Mac:

```sh
go run ./tools/genscripts
```

This compiles each script into `testdata/scpt` and saves the `osadecompile` output in `testdata/expected`.

To compare scriptview with `osadecompile` on your own set of scripts, use `tools/corpus/compare.py`. The instructions are at the top of the file.

## How this was made

Claude, an AI model, wrote most of the code and the notes. A human directed the work, reviewed it, and pushed back when it was wrong.

## License

MIT. See [LICENSE](LICENSE).
