# Notes

These are the things we learned about compiled AppleScript while we built scriptview. Apple doesn't document the format, so most of this comes from compiling small test scripts with `osacompile`, reading the bytes, and comparing our output with `osadecompile` on a few thousand real scripts.

Some of it is a guess that happens to match every script we tried. We say so when that's the case.

- [The file format](file-format.md): the container, the object graph and the records in it.
- [The syntax tree](syntax-tree.md): how a script's source is stored, and how flags record the way you wrote it.
- [The bytecode](bytecode.md): the virtual machine, its instructions, and how we decompile run-only scripts.
- [Terminology](terminology.md): where words like `window` and `display dialog` come from, and when names need `|pipes|`.
- [Applications](applications.md): how a script remembers an app, and how `osadecompile` names it.
- [Output quirks](quirks.md): the odd things `osadecompile` does that we copy.
- [How we tested](testing.md): probes, corpora and fuzzing.
- [Sources](sources.md): papers, books and tools that helped.
