# File format

This page describes how a compiled AppleScript file is laid out, from the container down to the individual records. Apple doesn't publish this format. The description comes from compiling test scripts, reading the bytes, disassembling the AppleScript runtime of 1993, and checking the result against more than 12,000 real scripts. Where a statement is an inference rather than an observation, the page says so.

For the syntax tree and the bytecode that the records describe, see [Syntax tree](syntax-tree.md) and [Bytecode](bytecode.md). For how often each structure occurs, see [Corpus statistics](statistics.md).

## Overview

A compiled script, or FASD file, has three parts:

1. A header of 12 or 16 bytes that starts with `FasdUAS`.
2. A body: a sequence of records that together form a graph of objects.
3. A trailer of 12 or more bytes that ends with `FA DE DE AD`.

All numbers are big-endian.

The object graph holds the whole script, and object 0 is its root. The root leads to two descriptions of the same program:

- A syntax tree, which records the source as you wrote it. Script Editor and `osadecompile` print the source from this tree. A run-only script has no syntax tree.
- Bytecode for a stack-based virtual machine, which is what runs. Every compiled script has bytecode.

## Containers

The FASD data can arrive inside several containers:

- A `.scpt` file holds it in the data fork, from the first byte.
- A script bundle (`.scptd`) and an applet (`.app`) hold it in `Contents/Resources/Scripts/main.scpt`.
- A classic Mac OS script holds it in the resource fork, as resource type `scpt` with ID 128. The data fork is empty.
- An AppleDouble file (`._name`, or `__MACOSX/._name` in a zip file) holds the resource fork as its entry 2.
- A MacBinary or BinHex file holds it in the resource fork.
- A script with a `#!` line holds it after the first line break.

Classic Mac OS kept compiled scripts in the resource fork, with the file type `osas`. Inside Macintosh suggests resource ID 128, which Script Editor uses, and keeps the script's description in `TEXT` and `styl` resources with ID 1128. AppleScript 1.6 (2001) could also read scripts from the data fork, Script Editor 1.9 (2003) saved new scripts there by default, and AppleScript 1.9.2, in Mac OS X 10.3, added `.scptd` bundles.

Some files with a `.scpt` name hold something else:

- A JavaScript for Automation script starts with `JsOsaDAS`, followed by a binary property list that holds the source text.
- Some `.scpt` files are plain source text. `osadecompile` compiles these before it prints them, so its output can differ from the file.

Scripts that pass through other systems sometimes gain junk bytes after the trailer. Deleting everything after the last `FA DE DE AD` repairs them.

## Header

The header is the ASCII text `FasdUAS`, a space, and one or two versions of four characters each, such as `1.10` followed by `1.10` in `FasdUAS 1.101.10`.

There are four headers:

- `FasdUAS 0.95`, 12 bytes: AppleScript 1.0b1, a beta from early 1993.
- `FasdUAS 0.98`, 12 bytes: an AppleScript 1.0 prerelease. Only one script with this header is known, the `listLib` sample library, which shipped unchanged with every AppleScript 1.0 installer from June 1993 on.
- `FasdUAS 1.00`, 12 bytes: AppleScript 1.0 (1993).
- `FasdUAS 1.101.10`, 16 bytes: AppleScript 1.1 (late 1993) and every version since.

All 12,150 current scripts in the test corpus have the header `FasdUAS 1.101.10`, and so do the 33 classic Mac OS scripts that AppleScript 1.1 wrote. The version fields haven't changed since then, although the format inside has grown: UTF-16 strings, `#` comments and `use` statements all came later. After AppleScript 1.1, the header doesn't tell old scripts from new ones.

Apple never explained the name. Jon Pugh, who tested early versions of AppleScript, wrote on the applescript-users list in 2002: "I don't know what Fasd stands for." `UAS` is the prefix of the runtime's internal names. The runtime's loader is called FasLoad, according to a port of it in another decompiler, which suggests that `Fasd` pairs with it as a fast dump, in the Lisp tradition of fast-load files. This is an inference.

Current macOS refuses scripts with a version before 1.10 with the error -1758, `errOSADataFormatObsolete`: "Data format is obsolete." The error was already part of the Open Scripting Architecture in 1993, together with -1759, `errOSADataFormatTooNew`.

## Trailer

The trailer comes from the Open Scripting Architecture, not from AppleScript. Inside Macintosh explains its purpose: when a script is stored as generic storage (`typeOSAGenericStorage`, the code `scpt`), "the data … is always followed by a trailer containing the subtype value for the scripting component that created the script data." The generic scripting component reads the trailer to choose the component that loads the script, and the functions `OSAGetStorageType`, `OSAAddStorageType`, and `OSARemoveStorageType` read, add, and remove it. Apple doesn't document the trailer's bytes. Their layout comes from reading files:

- 4 bytes: the scripting component's type, `ascr` for AppleScript.
- 2 bytes: a version.
- 2 bytes: the length of the trailer in bytes, counted back from its end.
- 4 bytes: the marker `FA DE DE AD`.

The length is usually 12. A length of 13 includes one pad byte before `ascr`. The body is everything between the header and the trailer.

Jon Pugh described the marker as "a terminating sequence that won't appear in any script."

## Records

The body is a sequence of records. Each record starts with the same five bytes:

- 1 byte: the record type.
- 2 bytes: the reference, a signed 16-bit object number.
- 2 bytes: the size.

What the size means depends on the record type: a count of references, a count of bytes, or the value itself. The payload follows.

### References

Records refer to each other by object number:

- A negative number names an inline object. The object is the next record in the stream at the point where something first refers to it.
- A number of zero or more names a shared object. Several objects can refer to the same one. Object 0 is the root.

The writer emits each object depth first, the first time something refers to it, so the body is the whole graph in that order, with nothing in between. A reader can decode the records one after another and keep a table from object number to object.

### Record types

In the following list, the size field counts references, counts bytes, or is the value itself, as each item says.

- `01`, symbol. The size counts bytes. Current files have no payload. In AppleScript 1.0 files, 4 bytes index the runtime's symbol table.
- `02`, list cell. The size counts references. Two references, the head and the tail. A list is a chain of cells. In AppleScript 1.0 files, a cell with no references is an empty list.
- `03`, small integer. The size field is the integer, and there's no payload.
- `04`, typed vector. The size counts references. A kind byte, then the references. Not seen in the corpus, which uses `0e`.
- `06`, binding cell. The size counts references. Three references: the key, the value, and the next cell. Records and labeled parameters are chains of these.
- `07`, integer. The size counts bytes. A signed 32-bit integer.
- `08`, real. The size counts bytes. An IEEE 754 double.
- `09`, boolean. The size field is the value, and there's no payload. Not seen in the corpus.
- `0a`, term. The size counts bytes. A kind byte, then a code. See [Terms](#terms).
- `0b`, identifier. The size counts bytes. A kind byte, then two names. See [Identifiers](#identifiers).
- `0c`, byte string. The size counts bytes. Text in the script system's encoding. See [Strings](#strings).
- `0d`, syntax node. The size counts references. A kind byte, the flags (2 bytes), the code start (2 bytes) and the code end (2 bytes), then the references. See [Syntax tree](syntax-tree.md).
- `0e`, typed vector. The size counts references. A kind byte, then the references. See [Typed vectors](#typed-vectors).
- `0f`, descriptor. The size counts bytes. A kind byte, then an Apple event value. See [Descriptors](#descriptors).
- `10`, vector. The size counts references. The references, with no kind byte.
- `11`, raw bytes. The size counts bytes. UTF-16 text, or bytecode.
- `12`, long descriptor. The size is unused. A kind byte, a 32-bit length, then the value.
- `13`, long raw bytes. The size is unused. A 32-bit length, then the bytes. A string longer than 64 KB in UTF-16 uses this type.

## Typed vectors

The typed vector (`0e`) is the general container of the format. Its kind byte says what it holds:

- 0: a plain list, of names or of a list literal that the compiler evaluated. The items follow.
- 4: positional parameters. A count, then a kind 0 vector of names. A kind 4 vector inside it is a pattern parameter, as in `on f({a, b})`.
- 15: a script object, including the root. The name, the syntax tree, the parent, and the handler table.
- 16: a handler. See [Handlers](bytecode.md#handlers).
- 17: a closure, which is a handler whose variables a script object inside it still uses. Laid out like kind 16.
- 19: the placeholder for `it` inside a `whose` filter. No items.
- 20: `a reference to`, as a constant. The reference.
- 21: a property reference constant. The container and the property.
- 22: an `every` constant. The container and the class.
- 24: an index reference constant. The container, the class, and the index.
- 25: a key reference constant, for `id` or `named`. The container, the class, the value, and the key form.
- 26: a `whose` filter constant. The reference and the comparison.
- 27: a range constant. The container, the class, the start, and the end.
- 28 and 29: `before` and `after` constants. The container and the class.
- 30, 31, and 32: `beginning`, `end`, and `middle` constants. The container.
- 33 to 41: comparison constants inside filters. The left side and the right side.
- 122: a command name, with the target it was compiled for. The name and the target.
- 123: a handler parameter with a type or a default value. The name, the class, and the default.
- 124: not understood. Seen as the value of a record field, always with the five items `{0, 0, nil, nil, nil}`.
- 177 (`b1`): a UTF-16 string. A `11` record with the text.

Kinds 20 to 32 are references that the compiler evaluated ahead of time, because every part of them is a constant: `item 2 of {1, 2}`, or a reference to an app object written out in full. Each kind is the matching `MakeObjectAlias` form plus 21 (see [References](bytecode.md#references)), and kind 20 stands for `a reference to`. Kinds 33 to 41 are the `MakeComp` comparisons plus 33: equal, not equal, greater than, greater than or equal, less than, less than or equal, starts with, ends with, and contains.

In the AppleScript 1.0b1 beta, the root is a kind 17 or 18 vector instead. See [The AppleScript 1.0b1 beta](#the-applescript-10b1-beta).

## Terms

A term record (`0a`) holds a code from a scripting dictionary. Its kind byte says what sort of code it is:

- `0a`, 4 bytes: a class, property, or constant, such as `cwin` for `window`.
- `0b`, 8 bytes: a typed constant, the enumeration code and then the value code, such as `savo` `ask `. In AppleScript 1.0b1 files, only the 4-byte value code.
- `2e`, 24 bytes: an event, such as `sysodlog` for `display dialog`. The suite (4 bytes), the event ID (4 bytes), the reply type (4 bytes), two flag words (8 bytes), and the direct parameter's type (4 bytes).
- `2f`, 4 bytes: a property code, in AppleScript 1.0 files.
- `31`: an event, in AppleScript 1.0b1 files. The 8-byte code, then parameter information.
- `32`, 4 bytes: a property code, in AppleScript 1.0b1 files.

Only the first 8 bytes of an event matter for printing. Codes aren't always printable text. The corpus has codes with NUL bytes and other control characters in them.

## Identifiers

An identifier record (`0b`) holds the name of a variable or handler. The kind byte is `30` (`33` in the AppleScript 1.0b1 beta). Two names follow, each a 16-bit length and then the bytes:

1. The name in lowercase. The runtime compares this copy, because identifiers ignore case.
2. The name as you wrote it. An empty name means you wrote the identifier in pipes, such as `|my var|`.

Current files store names as UTF-8, and classic Mac OS files store them in the script system's encoding. Try UTF-8 first.

## Strings

A string literal has one of two forms:

- Current files store it as UTF-16, in a raw `11` record wrapped in a kind 177 vector.
- AppleScript 1.0 files store it as a byte string (`0c`) in the script system's encoding.

A comment keeps both a byte string and a UTF-16 copy. Use the UTF-16 copy, because the byte string loses any character that its encoding can't represent.

### Text encodings

Classic Mac OS stored text in the encoding of the script system that was running: Mac Roman for Western languages, and a Shift JIS variant for Japanese. A compiled script doesn't record which encoding it used.

scriptview reads a file as Japanese when these conditions are all true:

- Every byte string with non-ASCII bytes decodes as Shift JIS.
- Together, the strings hold several kana or kanji.
- No string reads as AppleScript raw code syntax in Mac Roman, such as `«class cfol»`.

Western text almost never passes the test: an accented Mac Roman letter such as `é` is a Shift JIS lead byte, and the byte after it rarely makes a valid pair. The Shift JIS table comes from the macOS CP932 converter, through `tools/gensjis`.

## Descriptors

A descriptor record (`0f`) holds an Apple event value. The kind byte says how:

- `08`: an application specifier, such as `application "Safari"`, as an alias record or the type `null`. See [Applications](applications.md).
- `0d`: any other value, as a four-character type and then the data.

The corpus has kind `0d` descriptors of these types:

- `ldt `: a date, as seconds since January 1, 1904.
- `alis`: an alias literal.
- `utxt`, `furl`, `fss `, and image types such as `PICT`, `TIFF`, and `JPEG`. These print as `«data TYPE0123ABCD»`.

An application specifier can also hold an `aprl` URL instead of an alias, for an app named by a URL.

## The root object

Object 0 is the root. In every file from AppleScript 1.0 on, it's a kind 15 script object with four items: the name, the syntax tree, the parent, and the handler table.

- The name is nil for the top-level script.
- The syntax tree is the top block of statements. A run-only script has nil here. That is the only difference between a run-only script and a normal one: the bytecode is still all there.
- The parent is the script's parent object. An empty kind 15 vector stands for AppleScript, the default, which isn't saved with the file. A script with `property parent : application "Finder"` has the application specifier here, and its handler table has an entry named `pare` with no value.
- The handler table is a list, `[nil, [names], entry, entry, …]`. Each entry is a handler (kind 16 or 17), a script object (kind 15), or the saved value of a property or top-level variable, in the order of the names.

A script object inside the script has the same layout: a kind 15 vector with a name and its own table.

## AppleScript 1.0 files

Files with the versions 0.98 and 1.00 use the same records as current files, with these differences:

- Many built-in terms are symbols (`01`) with a 4-byte payload instead of term records. The payload indexes a table inside the AppleScript runtime, stored as `index * 8 + 1`. For example, index 14 is `string`, 109 is `return`, 121 and 122 are `true` and `false`, and 58 and 127 are the `run` and `open` events.
- Class codes in term records use kind `2f` instead of `0a`.
- String literals are byte strings (`0c`), not UTF-16.
- An empty list is a list cell (`02`) with no references.
- A handler lists its positional parameters as plain names, not as a kind 4 vector with a count.
- The bytecode builds every list literal with `MakeList`. Later versions use `MakeVector` for `{…}`, and keep `MakeList` for the rarely used `[…]` list.
- A script object saved in a property has no name of its own. The property names it.
- A run-only script that consists of only a run handler is saved as the handler itself, a kind 16 root with seven items, with no script object around it. The PlainTalk speech rules are stored this way.

### The symbol table

Current macOS no longer has the symbol table. scriptview's copy, in `scpt/legacy.go`, comes from the startup code of the AppleScript 1.0 extension: resource `PROC` 128 of version 1.0, from Apple's October 1993 Developer CD. That code builds the table one entry at a time. It pushes a four-character code, or an event's class and ID, calls a constructor, and stores the result in the next slot. Counting the calls in a disassembly gives each entry's index.

The table has 139 entries. About 30 of them are objects that the runtime uses only internally, such as anonymous `obj ` placeholders, and scriptview leaves them out. A symbol that isn't in the table prints as a `<ref>` placeholder.

## The AppleScript 1.0b1 beta

The beta wrote `FasdUAS 0.95`. Its records are the same as later versions', and its syntax tree holds the same statements in nearly the same layouts, under different names. scriptview translates a beta file into the AppleScript 1.0 form when it reads it (`scpt/beta.go`), and then prints it like any other script. The beta CD has plain-text copies of 16 of its 26 compiled scripts, which give the expected output.

### Node letters

Later versions added node types in the middle of the alphabet, which moved every letter after them. The beta's letters map to the 1.0 letters by four offsets:

- `4` to `E` map to `1` to `B`, 3 letters back: references, insertion points, and the comparisons from `=` to `≤`.
- `H` to `N` map to `C` to `I`, 5 letters back: `starts with` through `not`, and commands.
- `P` to `i` map to `J` to `c`, 6 letters back: lists, records, control statements, arithmetic, and `as`.
- `m` to `z` map to `f` to `s`, 7 letters back: `me`, `it`, script objects, handlers, blocks, variables, `set`, and `copy`.

For example, `tell` is `U` in the beta and `O` later, and `set` is `y` and `r`. The beta files never use `E`, `F`, `G`, `O`, `S`, `T`, `V`, `X`, or `j` to `l`, so the mapping of those letters is an inference from their neighbors. `F` and `G` have no 1.0 counterpart, and scriptview maps `j` to unary minus and `k` to `get`.

### Layouts

These nodes store their children differently:

- A handler (`p`) holds the name, the body, the direct parameter, and the positional parameters. In 1.0, it holds a signature node, then the body.
- `try` (`W`) holds the body, the error variable, the error parameters, and the handler. In 1.0, it holds the body, an `error` clause node, and the handler.
- A script object (`o`) holds a `0c` string with the name, then the body. In 1.0, the name is an identifier.
- A call (`N`) holds the name, the direct parameter, and then either the labeled or the positional parameters. In 1.0, positional parameters are in the second slot.
- A line (`s`) holds a statement or expression, then a comment, like 1.0's `l`, with other flag values.

An empty body, such as the body of an empty `repeat` loop, is a literal of the empty symbol rather than no node.

The beta's error handler is `on error … end error`, inside the statement it protects. The beta compiler stores it as a `try` node around the whole enclosing block, so a handler written at the end of a `tell` block wraps the `tell`. scriptview prints it as `try … on error … end try`. In the same way, it prints `actor` as `script` and `exit` as `exit repeat`.

### Flags and parentheses

The `s` node's flags say what kind of line or wrapper it is: 0 for a statement with a `--` comment, 3 for a `--` comment on its own line, 4 for a `(* … *)` comment, 5 for a blank line, 6 for an expression after a `¬` break, 7 for one before a `¬` break, and 8 for the article `the`. Other nodes use their own flag values for alternative wordings, such as `is` and `is equal to`, `in front of`, and `begins with`. scriptview maps them to the 1.0 flags where a 1.0 spelling exists.

The beta doesn't store parentheses. scriptview adds them where the meaning needs them: around the left side of `as` when it's a reference or an arithmetic expression, as in `((size of word i) * percentage) as integer`, and around references at the ends of a range.

### Terms and symbols

Events use term kind `31`, with parameter information after the 8-byte code. Property codes use kind `32`, and class codes use `0a`, as later. An enumerator (`0b`) has only its 4-byte code, without the type. Identifiers use kind `33`.

The beta's symbol table holds mostly the same terms as 1.0's, in a different order. scriptview has a copy (`betaSymbols`), read from the beta's extension the same way as the 1.0 table.

### Run-only beta scripts

scriptview reads only the beta's syntax tree. The beta's bytecode uses other opcodes, which haven't been mapped, so a run-only beta script prints nothing. One is known, a FileMaker Pro demo applet from 1993.

### Two builds

The beta files come from two builds. Most have a kind 17 root vector of 5 items. The 10 scripts in SimpleDialog, a sample app on the same CD, have a kind 18 root of 8 items. Their symbol table is missing two entries before `true` and `false`: they store `true` as entry 123 and `false` as 124, which are the `index` and `ID` key forms in the other files. Each SimpleDialog script sets a check box or radio button to one of these two values, which shows which one is which.

