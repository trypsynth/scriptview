# How we tested

With no documentation, the only way to learn the format was to ask the real tools. We used three kinds of tests.

## Probes

A probe is a small script that tests one thing. We write it, compile it with `osacompile`, and decompile it with `osadecompile`. Then we look at the tree with `scriptview tree` and compare our output with Apple's.

For example, to learn how `is not greater than or equal to` is stored, we compiled five lines with the negated comparisons and looked at the flags. Most of the rules in these notes came from a probe like that.

The probes live in `testdata/scripts`. `go run ./tools/genscripts` compiles them into `testdata/scpt` and saves Apple's output in `testdata/expected`, and `go test` compares ours with it. `testdata/runonly` has run-only copies of some probes, with our own output as the expected result.

A heads-up if you run the probes yourself: compiling a script that talks to an app makes macOS fetch that app's dictionary. For System Events, macOS asks for permission first. That's normal.

## Corpora

Probes only find what you think to test. To find the rest, we compared scriptview with `osadecompile` on two big sets of real scripts:

- The compiled scripts that ship with macOS, in `/Library/Scripts` and `/System/Library`. There are 194.
- Scripts from about 1,370 AppleScript repositories on GitHub. We compiled the `.applescript` files ourselves, and added any `.scpt` files the repositories had. There are 6,142.

`tools/corpus/compare.py` runs both tools on a list of files and saves a diff for each file that doesn't match. We fixed the most common diff, ran it again, and repeated. The GitHub set was the most useful. It has old scripts from old compilers, scripts that use Objective-C, and all sorts of odd formatting.

We can't include these scripts in the repository, because other people wrote them. The script and the instructions are there so you can build your own set.

For the bytecode decompiler, we first compared its output with the syntax tree output of the same file, with all flags set to zero. That turned out to be a weak test, because zeroing the flags also removes parentheses that the source needs.

A better test is a round trip: decompile the bytecode, compile the result again with `osacompile`, and compare the new bytecode with the original. If they're the same, the decompiled source means exactly what the original meant. `tools/corpus/roundtrip.py` does this. It found several bugs that the first test missed, such as `use` statements in the wrong place and constant lists that printed as garbage.

## Fuzzing

`go test -fuzz FuzzDecompile ./internal/scpt` feeds scriptview damaged files. It found five bugs:

- Nodes that point back at themselves, which made the printer recurse until it crashed.
- A list of call arguments that loops back on itself, which made one function run forever.
- A binary property list with offsets past the end of the file.
- A marker character we used inside the printer that could also appear in a string.
- Deep recursion in two helper functions.

Every walk over the object graph has a limit now. The inputs that found bugs are saved in `internal/scpt/testdata/fuzz` and run with the normal tests.
