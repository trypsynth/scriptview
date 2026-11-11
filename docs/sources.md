# Sources

Apple never documented the compiled script format, but some published material helps. This page lists the sources that these notes rely on, and what each one contributes. Most of the rest comes from compiling small scripts and reading the bytes. See [Testing](testing.md).

## Apple documentation

- *Inside Macintosh: Interapplication Communication*, 1993. [PDF index](https://developer.apple.com/library/archive/documentation/mac/pdf/Interapplication_Communication/pdf.html). Chapter 8 describes the `aete` and `aeut` resources, and says that naming a standard suite in an `aete` switches on the whole suite, which explains the words that need pipes only with Standard Additions. Chapter 10 describes how scripts are stored:
  - [Saving Script Data](https://developer.apple.com/library/archive/documentation/mac/IAC/IAC-335.html): generic storage, the trailer that names the scripting component, the `scpt` resource with ID 128, the file type `osas`, and the `TEXT` and `styl` resources for the description.
  - [OSAStore](https://developer.apple.com/library/archive/documentation/mac/IAC/IAC-348.html) and [OSALoad](https://developer.apple.com/library/archive/documentation/mac/IAC/IAC-349.html): run-only storage (`kOSAModePreventGetSource`), and the errors -1758 (`errOSADataFormatObsolete`) and -1759 (`errOSADataFormatTooNew`).
  - [OSAGetStorageType](https://developer.apple.com/library/archive/documentation/mac/IAC/IAC-414.html) and the related pages: the functions that read, add and remove the trailer.
- The Open Scripting Architecture headers in the macOS SDK: `OSA.h`, `AppleScript.h`, `ASRegistry.h` and `MacErrors.h`. They define run-only mode and the error codes, and name the runtime's classes, such as `cClosure`, which helps explain handler kind 17.
- [AppleScript release notes for Mac OS X 10.3](https://developer.apple.com/library/archive/releasenotes/AppleScript/RN-AppleScript/RN-10_3/RN-10_3.html): script bundles (`.scptd`), and the end of resource-fork scripts from `store script`. The notes for later versions cover Unicode text and `#` comments.

## History and design

- William R. Cook, "AppleScript", *HOPL III*, 2007. [Draft PDF](https://www.cs.utexas.edu/~wcook/Drafts/2006/ashopl.pdf). The history of the language, by one of its designers. It explains that the stored form is language neutral and that printing is the dialect's job, which is why the syntax tree keeps your wording in flags. It describes the team's own runtime, "a garbage collector and byte-code interpreter," and the destructuring `set` that `MatchLiteral` implements.
- Jon Pugh, "Re: FADE DEAD", applescript-users mailing list, March 13, 2002. On the trailer marker, "a terminating sequence that won't appear in any script," and on the header: "I don't know what Fasd stands for."
- Chris Nebel, applescript-users mailing list, August 26, 2001, and March 2, 2003. Data-fork scripts in AppleScript 1.6, and Script Editor 1.9 saving new scripts in the data fork.
- Adam Engst, ["macOS 26.4's Script Editor Won't Open Some Older AppleScripts"](https://tidbits.com/2026/03/27/macos-26-4s-script-editor-wont-open-some-older-applescripts/), TidBITS, March 27, 2026. Error -1758 for old scripts that live in a resource fork.

## Other decompilers and tools

- Jinmo, [applescript-disassembler](https://github.com/Jinmo/applescript-disassembler). A port of AppleScript's loader and opcode table, with Apple's internal names. The opcode names on these pages come from here.
- Phil Stokes, SentinelLabs, ["FADE DEAD: Adventures in Reversing Malicious Run-Only AppleScripts"](https://www.sentinelone.com/labs/fade-dead-adventures-in-reversing-malicious-run-only-applescripts/), and the [aevt_decompile](https://github.com/SentineLabs/aevt_decompile) tool. Run-only scripts in malware, and how to read their disassembly.
- Peter Berba, ["Decompiling run-only AppleScripts"](https://pberba.github.io/security/2025/12/14/decompiling-run-only-applescripts/), 2025, and [applescript-decompiler](https://github.com/pberba/applescript-decompiler). Another run-only decompiler, with notes on the stack effects of several instructions.
- [n0kovo/applescript-decompiler](https://github.com/n0kovo/applescript-decompiler). A Python decompiler that describes its parser as a port of the runtime's FasLoad routine.
- [ViRb3/applescript-tools](https://github.com/ViRb3/applescript-tools). A disassembler, decompiler and deobfuscator in Go.
- The `file` command's magic database, `Magdir/apple`, which identifies compiled scripts by the string `FasdUAS`.
- quuxfu, ["chop-chop: repair corrupt AppleScript .scpt files"](http://quuxfu.blogspot.com/2014/09/repair-corrupt-applescript-scpt-files.html), 2014. Junk bytes after `FADEDEAD`.

## Original software

- *AppleScript: The Beta Release* and the Apple Developer CD Series, October 1993 to May 1994, through archive.org. The AppleScript 1.0b1 and 1.0 extensions, whose startup code builds the symbol tables, and the classic scripting additions and apps whose dictionaries `tools/genclassic` reads.
