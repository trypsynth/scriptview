# How we tested

With no documentation, the only way to learn the format was to ask the real tools. We used three kinds of tests.

## Probes

A probe is a small script that tests one thing. We write it, compile it with `osacompile`, and decompile it with `osadecompile`. Then we look at the tree with `scriptview tree` and compare our output with Apple's.

For example, to learn how `is not greater than or equal to` is stored, we compiled five lines with the negated comparisons and looked at the flags. Most of the rules in these notes came from a probe like that.

The probes live in `testdata/scripts`. `go run ./tools/genscripts` compiles them into `testdata/scpt` and saves Apple's output in `testdata/expected`, and `go test` compares ours with it. `testdata/runonly` has run-only copies of some probes, with our own output as the expected result.

A heads-up if you run the probes yourself: compiling a script that talks to an app makes macOS fetch that app's dictionary. For System Events, macOS asks for permission first. That's normal.

## Corpora

Probes only find what you think to test. To find the rest, we compared scriptview with `osadecompile` on two big sets of real scripts:

- The compiled scripts that ship with macOS, in `/Library/Scripts` and `/System/Library`. macOS 26 has 163.
- Scripts from the 4,535 repositories GitHub lists as AppleScript, found with `gh search repos --language=AppleScript`, one search per year created. We compiled the `.applescript` files ourselves, and added any `.scpt` files the repositories had. With duplicates removed, there are 14,878, of which 2,621 are plain text saved as `.scpt`. We left out two repositories that collect AppleScript malware: macOS's malware scanner flags their files, and the scripts are nothing we need.

`tools/corpus/compare.py` runs both tools on a list of files and saves a diff for each file that doesn't match. We fixed the most common diff, ran it again, and repeated. The GitHub set was the most useful. It has old scripts from old compilers, scripts that use Objective-C, and all sorts of odd formatting.

We can't include these scripts in the repository, because other people wrote them. The script and the instructions are there so you can build your own set.

For the bytecode decompiler, we first compared its output with the syntax tree output of the same file, with all flags set to zero. That turned out to be a weak test, because zeroing the flags also removes parentheses that the source needs.

A better test is a round trip: decompile the bytecode, compile the result again with `osacompile`, and compare the new bytecode with the original. If they're the same, the decompiled source means exactly what the original meant. `tools/corpus/roundtrip.py` does this. It found several bugs that the first test missed, such as `use` statements in the wrong place and constant lists that printed as garbage.

The latest results, on macOS 26.5:

| Set | Matches `osadecompile` | Round trip gives the same bytecode |
| --- | --- | --- |
| macOS scripts | 163 of 163 | 163 of 163 |
| GitHub | 12,105 of 12,143 | 11,632 of 12,150 |

Most of the GitHub differences left aren't ours to fix:

- Scripts for apps that aren't installed. `osadecompile` prints their names from the app's dictionary, and we print raw codes, or the other way around for an app we know. In the round trip, some of these don't compile at all without the app, or compile a raw code to something else.
- Scripts compiled by old versions of AppleScript, whose code the current compiler writes differently: an extra `GetResult` at the end of a script, or `time string of (current date)` as two references.
- `do shell script ("x" as string)`, where `osadecompile` drops parentheses that the script needs (see [Quirks](quirks.md)).

## Classic scripts

For scripts from before Mac OS X, we used compiled scripts from Apple's discs of 1993 and 1994, from archive.org mirrors: the 48 on the AppleScript Developer's Toolkit, and those on the Developer CD Series from October 1993 to May 1994, and the floppies that came with *The Tao of AppleScript*. The Developer CDs and the floppies hold 106 different AppleScript 1.0 scripts. 64 of them are run-only: the speech rules that ship with PlainTalk. The rest are the sample scripts that came with AppleScript, and scripts that came with other programs, such as JPEGView.

Current macOS won't open a single one of them, so there's nothing to compare against. We read the output by hand, and checked that every script decompiles with no placeholders left. The scripts that still have a syntax tree gave us one more test: decompile them again from the bytecode alone, as if they were run-only, and compare the two results. That found the 1.0 differences in handler parameters, list literals and script objects listed in [the file format](file-format.md#applescript-10-files).

Apple owns these scripts, so they aren't in the repository. The tests use synthetic files instead.

## Fuzzing

`go test -fuzz FuzzDecompile ./internal/scpt` feeds scriptview damaged files. It found five bugs:

- Nodes that point back at themselves, which made the printer recurse until it crashed.
- A list of call arguments that loops back on itself, which made one function run forever.
- A binary property list with offsets past the end of the file.
- A marker character we used inside the printer that could also appear in a string.
- Deep recursion in two helper functions.

Every walk over the object graph has a limit now. The inputs that found bugs are saved in `internal/scpt/testdata/fuzz` and run with the normal tests.
