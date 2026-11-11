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

| Container | Where the FASD data is |
| --- | --- |
| `.scpt` file | The data fork, from the first byte. |
| Script bundle (`.scptd`) | `Contents/Resources/Scripts/main.scpt`. |
| Applet (`.app`) | `Contents/Resources/Scripts/main.scpt`. |
| Classic Mac OS script | The resource fork, as resource type `scpt`, ID 128. The data fork is empty. |
| AppleDouble file (`._name`, or `__MACOSX/._name` in a zip file) | The resource fork entry, entry ID 2. |
| MacBinary or BinHex file | The resource fork. |
| Script with a `#!` line | The bytes after the first line break. |

Classic Mac OS kept compiled scripts in the resource fork, with the file type `osas`. Inside Macintosh suggests resource ID 128, which Script Editor uses, and keeps the script's description in `TEXT` and `styl` resources with ID 1128. AppleScript 1.6 (2001) could also read scripts from the data fork, Script Editor 1.9 (2003) saved new scripts there by default, and AppleScript 1.9.2, in Mac OS X 10.3, added `.scptd` bundles.

Some files with a `.scpt` name hold something else:

- A JavaScript for Automation script starts with `JsOsaDAS`, followed by a binary property list that holds the source text.
- Some `.scpt` files are plain source text. `osadecompile` compiles these before it prints them, so its output can differ from the file.

Scripts that pass through other systems sometimes gain junk bytes after the trailer. Deleting everything after the last `FA DE DE AD` repairs them.

## Header

The header is the ASCII text `FasdUAS ` and one or two versions of four characters each:

```
46 61 73 64 55 41 53 20  31 2E 31 30  31 2E 31 30
F  a  s  d  U  A  S  ␠   1  .  1  0   1  .  1  0
```

| Header | Written by | Length |
| --- | --- | --- |
| `FasdUAS 0.95` | AppleScript 1.0b1, a beta from 1992 | 12 bytes |
| `FasdUAS 0.98` | An AppleScript 1.0 prerelease | 12 bytes |
| `FasdUAS 1.00` | AppleScript 1.0 (1993) | 12 bytes |
| `FasdUAS 1.101.10` | AppleScript 1.1 (late 1993) and every version since | 16 bytes |

All 12,150 current scripts in the test corpus have the header `FasdUAS 1.101.10`, and so do the 33 classic Mac OS scripts that AppleScript 1.1 wrote. The version fields haven't changed since then, although the format inside has grown: UTF-16 strings, `#` comments and `use` statements all came later. After AppleScript 1.1, the header doesn't tell old scripts from new ones.

Apple never explained the name. Jon Pugh, who tested early versions of AppleScript, wrote on the applescript-users list in 2002: "I don't know what Fasd stands for." `UAS` is the prefix of the runtime's internal names. The runtime's loader is called FasLoad, according to a port of it in another decompiler, which suggests that `Fasd` pairs with it as a fast dump, in the Lisp tradition of fast-load files. This is an inference.

Current macOS refuses scripts with a version before 1.10 with the error -1758, `errOSADataFormatObsolete`: "Data format is obsolete." The error was already part of the Open Scripting Architecture in 1993, together with -1759, `errOSADataFormatTooNew`.

## Trailer

The trailer comes from the Open Scripting Architecture, not from AppleScript. Inside Macintosh explains its purpose: when a script is stored as generic storage (`typeOSAGenericStorage`, the code `scpt`), "the data … is always followed by a trailer containing the subtype value for the scripting component that created the script data." The generic scripting component reads the trailer to choose the component that loads the script, and the functions `OSAGetStorageType`, `OSAAddStorageType` and `OSARemoveStorageType` read, add and remove it. Apple doesn't document the trailer's bytes. Their layout comes from reading files:

| Bytes | Field |
| --- | --- |
| 4 | The scripting component's type, `ascr` for AppleScript |
| 2 | Version |
| 2 | Length of the trailer in bytes, counted back from its end |
| 4 | The marker `FA DE DE AD` |

The length is usually 12. A length of 13 includes one pad byte before `ascr`. The body is everything between the header and the trailer.

The marker is a joke that also works: "a terminating sequence that won't appear in any script," as Jon Pugh put it.

## Records

The body is a sequence of records. Each record starts with the same five bytes:

| Bytes | Field |
| --- | --- |
| 1 | Record type |
| 2 | Reference: a signed 16-bit object number |
| 2 | Size |

What the size means depends on the record type: a count of references, a count of bytes, or the value itself. The payload follows.

### References

Records refer to each other by object number:

- A negative number names an inline object. The object is the next record in the stream at the point where something first refers to it.
- A number of zero or more names a shared object. Several objects can refer to the same one. Object 0 is the root.

The writer emits each object depth first, the first time something refers to it, so the body is the whole graph in that order, with nothing in between. A reader can decode the records one after another and keep a table from object number to object.

### Record types

| Type | Name | Size field | Payload |
| --- | --- | --- | --- |
| `01` | Symbol | Bytes | Nothing in current files. In AppleScript 1.0 files, 4 bytes that index the runtime's symbol table. |
| `02` | List cell | References | Two references: head and tail. A list is a chain of cells. In AppleScript 1.0 files, a cell with no references is an empty list. |
| `03` | Small integer | The value | None. The size field is the integer. |
| `04` | Typed vector | References | A kind byte, then the references. Not seen in the corpus, which uses `0e`. |
| `06` | Binding cell | References | Three references: key, value, and the next cell. Records and labeled parameters are chains of these. |
| `07` | Integer | Bytes | A signed 32-bit integer. |
| `08` | Real | Bytes | An IEEE 754 double. |
| `09` | Boolean | The value | None. The size field is the value. Not seen in the corpus. |
| `0a` | Term | Bytes | A kind byte, then a code. See [Terms](#terms). |
| `0b` | Identifier | Bytes | A kind byte, then two names. See [Identifiers](#identifiers). |
| `0c` | Byte string | Bytes | Text in the script system's encoding. See [Strings](#strings). |
| `0d` | Syntax node | References | A kind byte, flags (2 bytes), code start (2 bytes) and code end (2 bytes), then the references. See [Syntax tree](syntax-tree.md). |
| `0e` | Typed vector | References | A kind byte, then the references. See [Typed vectors](#typed-vectors). |
| `0f` | Descriptor | Bytes | A kind byte, then an Apple event value. See [Descriptors](#descriptors). |
| `10` | Vector | References | The references, with no kind byte. |
| `11` | Raw bytes | Bytes | UTF-16 text, or bytecode. |
| `12` | Long descriptor | Unused | A kind byte, a 32-bit length, then the value. |
| `13` | Long raw bytes | Unused | A 32-bit length, then the bytes. A string longer than 64 KB in UTF-16 uses this type. |

## Typed vectors

The typed vector (`0e`) is the general container of the format. Its kind byte says what it holds:

| Kind | Meaning | Items |
| --- | --- | --- |
| 0 | A plain list: names, or a list literal that the compiler evaluated | The items |
| 4 | Positional parameters | Count, then a kind 0 vector of names |
| 15 | A script object, including the root | Name, syntax tree, parent, handler table |
| 16 | A handler | See [Handlers](bytecode.md#handlers) |
| 17 | A closure: a handler whose variables a script object inside it still uses | Same as kind 16 |
| 19 | The placeholder for `it` inside a `whose` filter | None |
| 20 | `a reference to`, as a constant | The reference |
| 21 | Property reference constant | Container, property |
| 22 | `every` constant | Container, class |
| 24 | Index reference constant | Container, class, index |
| 25 | Key reference constant (`id`, `named`) | Container, class, value, key form |
| 26 | `whose` filter constant | Reference, comparison |
| 27 | Range constant | Container, class, from, to |
| 30, 31, 32 | `beginning`, `end` and `middle` constants | Container |
| 33 to 41 | Comparison constants inside filters | Left side, right side |
| 122 | A command name, with the target it was compiled for | Name, target |
| 123 | A handler parameter with a type or a default value | Name, class, default |
| 124 | Not understood. Seen as the value of a record field. | Five items, `{0, 0, nil, nil, nil}` in every file seen |
| 177 (`b1`) | A UTF-16 string | A `11` record with the text |

Kinds 20 to 32 are references that the compiler evaluated ahead of time, because every part of them is a constant: `item 2 of {1, 2}`, or a reference to an app object written out in full. Each kind is the matching `MakeObjectAlias` form plus 21 (see [References](bytecode.md#references)), and kind 20 stands for `a reference to`. Kinds 33 to 41 are the `MakeComp` comparisons plus 33: equal, not equal, greater than, greater than or equal, less than, less than or equal, starts with, ends with, and contains.

The AppleScript 1.0b1 beta uses kinds 17, 18 and 28 to 31 in ways that later versions don't. See [The AppleScript 1.0b1 beta](#the-applescript-10b1-beta).

## Terms

A term record (`0a`) holds a code from a scripting dictionary. Its kind byte says what sort of code it is:

| Kind | Payload | Meaning |
| --- | --- | --- |
| `0a` | 4 bytes | A class, property or constant, such as `cwin` for `window` |
| `0b` | 8 bytes | A typed constant: the enumeration code, then the value code, such as `savo` `ask ` |
| `2e` | 24 bytes | An event, such as `sysodlog` for `display dialog`: suite (4 bytes), event ID (4 bytes), reply type (4 bytes), two flag words (8 bytes) and direct parameter type (4 bytes) |
| `2f` | 4 bytes | A class code, in AppleScript 1.0 files |
| `31` | 24 bytes | An event, in AppleScript 1.0b1 files |
| `32` | 4 bytes | A class code, in AppleScript 1.0b1 files |

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

| Kind | Payload | Meaning |
| --- | --- | --- |
| `08` | An alias record, or the type `null` | An application specifier, such as `application "Safari"`. See [Applications](applications.md). |
| `0d` | A four-character type, then the data | Any other value |

The corpus has kind `0d` descriptors of these types:

- `ldt `: a date, as seconds since January 1, 1904.
- `alis`: an alias literal.
- `utxt`, `furl`, `fss `, and image types such as `PICT`, `TIFF` and `JPEG`. These print as `«data TYPE0123ABCD»`.

An application specifier can also hold an `aprl` URL instead of an alias, for an app named by a URL.

## The root object

Object 0 is the root. In every file from AppleScript 1.0 on, it's a kind 15 script object with four items:

```
{name, syntax tree, parent, handler table}
```

- The name is nil for the top-level script.
- The syntax tree is the top block of statements. A run-only script has nil here. That is the only difference between a run-only script and a normal one: the bytecode is still all there.
- The parent is an empty kind 15 vector, which the loader replaces with a shared empty script object. It's probably the parent script, which isn't saved with the file.
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

Current macOS no longer has the symbol table. scriptview's copy, in `internal/scpt/legacy.go`, comes from the startup code of the AppleScript 1.0 extension: resource `PROC` 128 of version 1.0, from Apple's October 1993 Developer CD. That code builds the table one entry at a time. It pushes a four-character code, or an event's class and ID, calls a constructor, and stores the result in the next slot. Counting the calls in a disassembly gives each entry's index.

The table has 139 entries. About 30 of them are objects that the runtime uses only internally, such as anonymous `obj ` placeholders, and scriptview leaves them out. A symbol that isn't in the table prints as a `<ref>` placeholder.

## The AppleScript 1.0b1 beta

The beta wrote `FasdUAS 0.95`. Its symbol table holds mostly the same terms in a different order. scriptview has a copy (`betaSymbols`), read from the beta's extension the same way. The rest of the beta's format differs more, so scriptview doesn't decompile these files yet. The 26 scripts on the beta CD show these differences:

- The records are the same, and every file parses.
- The root is a kind 17 vector of 5 items, or a kind 18 vector of 8 items, not a kind 15 script object.
- Terms use kinds `31` for events and `32` for classes, and identifiers use kind `33`.
- The syntax tree uses different node letters. The script body is a `U` node, and `y` and `z` nodes appear that no later version uses.
