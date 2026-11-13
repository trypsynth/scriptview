# scriptview notes

These pages describe the file formats that scriptview reads. Apple documented none of them. The descriptions come from reading the bytes of real files, compiling test scripts, and disassembling Apple's software. Where a statement is an inference rather than an observation, the page says so.

## AppleScript

These pages come from compiling test scripts with `osacompile`, reading the bytes, disassembling the AppleScript runtime of 1993, and comparing scriptview's output with `osadecompile` on more than 12,000 real scripts.

Format reference:

- [File format](applescript/file-format.md): containers, the header and trailer, the records, typed vectors, terms, strings, and the differences in AppleScript 1.0 and its beta.
- [Syntax tree](applescript/syntax-tree.md): how a script's source is stored, every node type, and the flags that record how you wrote it.
- [Bytecode](applescript/bytecode.md): handlers, every opcode, references, and how scriptview decompiles run-only scripts.
- [Corpus statistics](applescript/statistics.md): how often each structure occurs in real scripts.

Printing:

- [Terminology](applescript/terminology.md): where words such as `window` and `display dialog` come from, and when names need pipes.
- [Applications](applescript/applications.md): how a script remembers an app, and how `osadecompile` names it.
- [Output quirks](applescript/quirks.md): the rules of Script Editor's printer that scriptview copies.

About these notes:

- [Testing](applescript/testing.md): probes, corpora, round trips, classic scripts, and fuzzing.
- [Sources](applescript/sources.md): documentation, papers, mailing list posts, and tools.
