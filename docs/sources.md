# Sources

Apple never documented the compiled script format, but some published material helped. This is what we read, and what each source gave us.

- William R. Cook, "AppleScript", HOPL III, 2007. [Draft PDF](https://www.cs.utexas.edu/~wcook/Drafts/2006/ashopl.pdf). The history of the language, from one of its designers. It explains that the stored form is language-neutral and printing is the dialect's job, which is why the syntax tree records your wording in flags. It also covers the Lisp background of the team and the destructuring `set` that `MatchLiteral` implements.
- Inside Macintosh: Interapplication Communication, 1993. [PDF index](https://developer.apple.com/library/archive/documentation/mac/pdf/Interapplication_Communication/pdf.html). Chapter 8 describes the `aete` and `aeut` resources, and says that naming a standard suite in an `aete` switches on the whole suite. That explained the words that need pipes only with Standard Additions. Chapter 10 describes how scripts are stored, and the trailer.
- The Open Scripting Architecture headers in the macOS SDK (`OSA.h`, `AppleScript.h`, `ASRegistry.h`). They define run-only mode (`kOSAModePreventGetSource`) and name the runtime's classes, such as `cClosure`, which helped explain handler kind 17.
- Jinmo, [applescript-disassembler](https://github.com/Jinmo/applescript-disassembler). A port of AppleScript's own loader and opcode table, with Apple's internal names. Our opcode names come from here.
- Phil Stokes, SentinelLabs, ["FADE DEAD: Adventures in Reversing Malicious Run-Only AppleScripts"](https://www.sentinelone.com/labs/fade-dead-adventures-in-reversing-malicious-run-only-applescripts/), and the [aevt_decompile](https://github.com/SentineLabs/aevt_decompile) tool. Run-only scripts in malware, and how to read their disassembly.
- Peter Berba, ["Decompiling run-only AppleScripts"](https://pberba.github.io/security/2025/12/14/decompiling-run-only-applescripts/), 2025, and [applescript-decompiler](https://github.com/pberba/applescript-decompiler). Another run-only decompiler, with notes on the stack effects of several instructions.
- The AppleScript release notes for macOS 10.3 and 10.5, in Apple's documentation archive. Bundles, Unicode text, and `#` comments.
- quuxfu, ["chop-chop: repair corrupt AppleScript .scpt files"](http://quuxfu.blogspot.com/2014/09/repair-corrupt-applescript-scpt-files.html). Junk bytes after `FADEDEAD`.

Most of the rest came from compiling small scripts and reading the bytes. See [How we tested](testing.md).
