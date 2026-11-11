# Testing

The format has no documentation, so the reference for every rule is the behavior of Apple's own tools: `osacompile`, which compiles, and `osadecompile`, which prints. scriptview is tested in four ways: probes, corpora, classic scripts, and fuzzing.

## Probes

A probe is a small script that tests one thing. To learn how a construct is stored:

1. Write the probe, for example five lines that use the negated comparisons.
2. Compile it with `osacompile`.
3. Print it with `osadecompile`, and look at its tree with `scriptview tree`.
4. Compare scriptview's output with `osadecompile`'s.

Most of the rules in these notes started as a probe.

The probes are in `testdata/scripts`. On a Mac, `go run ./tools/genscripts` compiles them into `testdata/scpt` and saves `osadecompile`'s output in `testdata/expected`. `go test` compares scriptview's output with those files. `testdata/runonly` holds run-only copies of some probes, with scriptview's own output as the expected result. On a Mac, `go test` also compiles that output again and checks that the bytecode matches.

Note: Compiling a script that talks to an app makes macOS load that app's dictionary, which can launch the app. For System Events, macOS asks for permission first.

## Corpora

Probes only find what you think to test. To find the rest, scriptview is compared with `osadecompile` on two large sets of real scripts:

- The compiled scripts that ship with macOS, in `/Library/Scripts` and `/System/Library`. macOS 26 has 163.
- Scripts from the 4,535 repositories that GitHub lists as AppleScript, found with `gh search repos --language=AppleScript`, with one search for each year of creation. The `.applescript` files are compiled on a Mac, and the `.scpt` files that the repositories contain are added. With duplicates removed, the set has 14,878 files, of which 2,621 are plain text saved as `.scpt`. Two repositories that collect AppleScript malware are left out: the macOS malware scanner flags their files, and they add nothing that the other scripts don't cover.

These scripts belong to their authors, so they aren't in the repository. You can build your own set with the tools described here.

### Comparing with osadecompile

`tools/corpus/compare.py` runs both tools on a list of files and saves a diff for each file that doesn't match. The approach is to fix the most common difference, run the comparison again, and repeat. The GitHub set is the most useful, because it has scripts from old compilers, scripts that use Objective-C, and every kind of odd formatting.

### Round trips

A run-only script has no syntax tree, so `osadecompile` can't print it, and there's nothing to compare with. The test for the bytecode decompiler is a round trip:

1. Decompile the bytecode.
2. Compile the result again with `osacompile`.
3. Compare the new bytecode with the original.

If the two are the same, the decompiled source means exactly what the original meant. `tools/corpus/roundtrip.py` runs this test on every script in a list, run-only or not.

An earlier test compared the bytecode decompiler's output with the syntax tree output of the same file, printed with all flags set to zero. That test is weak, because zeroing the flags also removes parentheses that the source needs.

### Results

The latest results, on macOS 26.5:

| Set | Matches `osadecompile` | Round trip gives the same bytecode |
| --- | --- | --- |
| macOS scripts | 163 of 163 | 163 of 163 |
| GitHub | 12,105 of 12,143 | 11,632 of 12,150 |

Most of the remaining GitHub differences have causes outside scriptview:

- Scripts for apps that aren't installed. `osadecompile` names their terms from the app's dictionary, and scriptview prints raw codes, or the other way around for an app that scriptview knows. In the round trip, some of these scripts don't compile without the app, or compile a raw code to something else.
- Scripts compiled by old versions of AppleScript, whose code the current compiler writes differently: an extra `GetResult` at the end of a script, or `time string of (current date)` as two references.
- `do shell script ("x" as string)`, where `osadecompile` drops parentheses that the script needs. See [Output quirks](quirks.md).

## Classic scripts

Scripts from before Mac OS X come from Apple's discs of 1993 and 1994, through archive.org mirrors:

- The AppleScript Developer's Toolkit, with 48 scripts.
- The Developer CD Series, from October 1993 to May 1994.
- *AppleScript: The Beta Release*, with AppleScript 1.0b1.
- The floppies of *The Tao of AppleScript*, second edition, with AppleScript 1.1.

Apart from the Toolkit, these discs hold 117 different AppleScript 1.0 scripts, 33 classic scripts from AppleScript 1.1, and 26 from the beta. 64 of the AppleScript 1.0 scripts are run-only: the speech rules that ship with PlainTalk. The rest are sample scripts that came with AppleScript, and scripts that came with other programs, such as JPEGView.

Current macOS doesn't open any AppleScript 1.0 script, so there's no reference output. These checks replace it:

- Every script decompiles with no placeholders left, and the output reads correctly by hand.
- The scripts that still have a syntax tree also decompile from their bytecode alone, as if they were run-only, and the two results agree. This check found the AppleScript 1.0 differences in handler parameters, list literals and script objects that [File format](file-format.md#applescript-10-files) lists.

The beta scripts have a better reference: the beta CD has plain-text copies of 16 of its 26 compiled scripts. scriptview's output matches them, apart from these differences:

- Beta spellings that scriptview prints in their 1.0 form: `on error … end error` as `try … on error … end try`, `actor` as `script`, `exit` as `exit repeat`, and `÷` as `/`.
- Names from the app's dictionary, which can differ from what the author typed: Quill's dictionary spells `Selection` and `Size` with capitals.
- Parentheses that the author wrote but the meaning doesn't need. The beta doesn't store parentheses at all.
- Terms from Excel 4, whose dictionary isn't on the discs, and print as raw codes.

The other 10 beta scripts belong to SimpleDialog, a sample app, and have no text copies. Their output reads correctly by hand.

Apple owns these scripts, so they aren't in the repository. The tests use synthetic files instead.

## Fuzzing

`go test -fuzz FuzzDecompile ./internal/scpt` feeds scriptview damaged files. Fuzzing found these bugs:

- Nodes that refer to themselves, which made the printer recurse until it crashed.
- A list of call arguments that loops back on itself, which made one function run forever.
- A binary property list with offsets past the end of the file.
- A marker character that the printer used internally, which could also appear in a string.
- Deep recursion in two helper functions.

Every walk over the object graph now has a limit. The inputs that found bugs are in `internal/scpt/testdata/fuzz`, and run with the normal tests.
