# Corpus statistics

This page counts how often each part of the format occurs in real scripts. Use it to judge which structures matter: a record type in every file needs care, and one in three files needs a test fixture more than a fast path.

The counts come from scriptview's own parser, run over four sets of scripts:

- GitHub: 12,150 compiled scripts and compiled sources from 4,535 GitHub repositories, compiled on macOS 26.5.
- macOS: the 163 compiled scripts in `/Library/Scripts` and `/System/Library` on macOS 26.5.
- AppleScript 1.0: 117 scripts from Apple's 1993 and 1994 discs, with the versions 0.98 and 1.00.
- AppleScript 1.1: 33 classic Mac OS scripts from the same discs, with the version 1.10.

The AppleScript 1.0b1 beta set, 26 scripts, appears where it differs. For how the sets were built, see [Testing](testing.md).

Each count is the number of scripts that contain the structure at least once. Unless an item says otherwise, the four numbers are in the order GitHub, macOS, AppleScript 1.0, and AppleScript 1.1.

## Headers and roots

- `FasdUAS 1.101.10`: 12,150, 163, none, and 33.
- `FasdUAS 1.00`: only in the AppleScript 1.0 set, 116 scripts.
- `FasdUAS 0.98`: only in the AppleScript 1.0 set, 1 script.

The root object:

- A kind 15 script object with 4 items: 12,146, 163, 53, and 33.
- A kind 16 handler with 7 items, for a run-only script that is only a run handler: only in the AppleScript 1.0 set, 64 scripts.

Run-only scripts: 110 in the GitHub set, 64 in the AppleScript 1.0 set (the PlainTalk speech rules), and none in the others.

## Record types

- `01` symbol: 12,147, 163, 117, and 33.
- `02` list cell: 12,125, 163, 117, and 33.
- `0e` typed vector: 12,146, 163, 117, and 33.
- `10` vector: 12,146, 163, 117, and 33.
- `11` raw bytes: 12,140, 163, 116, and 33.
- `0a` term: 12,076, 162, 79, and 33.
- `0d` syntax node: 12,036, 163, 53, and 33.
- `0b` identifier: 10,694, 156, 77, and 33.
- `03` small integer: 10,424, 148, 79, and 32.
- `0f` descriptor: 9,055, 147, 29, and 24.
- `0c` byte string: 8,487, 155, 65, and 28.
- `06` binding cell: 8,149, 152, 32, and 17.
- `08` real: 1,703, 6, 11, and 1.
- `07` integer: 306, 21, 4, and none.
- `13` long raw bytes: only in the GitHub set, 15 scripts.
- `12` long descriptor: only in the GitHub set, 3 scripts.

No script uses the types `04` or `09`.

Symbols with a 4-byte payload, which index the runtime's symbol table, occur only in AppleScript 1.0 files: 115 of the 117.

## Typed vector kinds

- 177, UTF-16 string: 11,172, 160, none, and none.
- 16, handler: 12,084, 162, 116, and 33.
- 15, script object: 12,146, 163, 53, and 33.
- 4, positional parameters: 6,134, 131, none, and 20.
- 0, plain list: 6,125, 131, none, and 20.
- 19, `it` placeholder in a filter: 2,511, none, 2, and 2.
- 20, `a reference to` constant: 2,103, 4, 2, and 2.
- 24, index reference constant: 1,692, 4, 2, and 2.
- 22, `every` constant: only in the GitHub set, 1,563 scripts.
- 17, closure: 185, none, 1, and none.
- 25, key reference constant: only in the GitHub set, 171 scripts.
- 21, property reference constant: 157, none, 1, and 2.
- 122, command name with target: only in the GitHub set, 142 scripts.
- 123, typed or default parameter: only in the GitHub set, 79 scripts.
- 124, not understood: only in the GitHub set, 47 scripts.
- 26, filter constant: only in the GitHub set, 5 scripts.
- 33, 36, and 40, comparison constants: only in the GitHub set, 4, 1, and 1 scripts.
- 27, range constant: only in the GitHub set, 3 scripts.

AppleScript 1.0 has no kind 177 strings and no kind 4 parameter lists: it stores strings as byte strings and parameters as plain names.

## Term kinds

- `0a`, class, property, or constant: 11,340, 161, 35, and 33, and 19 beta scripts.
- `2e`, event: 11,958, 162, 62, and 33.
- `0b`, typed constant: 7,609, 109, 32, and 15.
- `2f`, property, AppleScript 1.0: only in the AppleScript 1.0 set, 67 scripts.
- `31`, event, beta: 25 beta scripts.
- `32`, property, beta: 19 beta scripts.

## Descriptors

- `08`, application specifier with an alias: 8,705, 145, none, and none.
- `08` with the type `null`, an application specifier: 312, 2, 27, and 23.
- `0d` `ldt `, date: 72, none, 2, and none.
- `0d` `alis`, alias: 87, 5, 3, and 3.
- `0d` `utxt`, Unicode text: 18, 2, none, and none.
- `0d` `furl`, file URL: only in the GitHub set, 13 scripts.
- `0d` with an image type (`PICT`, `TIFF`, `JPEG`, or `sPic`): only in the GitHub set, 6 scripts.

## Syntax node types

The 20 most common node types in the GitHub set, out of the 12,036 scripts that have a syntax tree, from most to least common:

- `l`, line or wrapper: 12,036 scripts.
- `k`, block: 11,981.
- `m`, literal: 11,936.
- `I`, call: 11,482.
- `o`, variable: 10,521.
- `r`, `set`: 10,279.
- `n`, `x of y`: 9,973.
- `1`, property: 9,526.
- `O`, `tell`: 9,372.
- `4`, element by index: 8,739.
- `Z`, `if`: 7,801.
- `b`, `&`: 6,779.
- `=`, `=`: 6,749.
- `i`, handler: 6,332.
- `L`, `return`: 6,166.
- `J`, list: 5,712.
- `R`, `error`: 5,338.
- `c`, `as`: 5,273.
- `Q`, `try`: 5,127.
- `2`, `every`: 5,009.

The rarest types are `<` (`middle`, 7 scripts), `u` (`with transaction`, 8), `8` (`before`, 51), and `9` (`after`, 59).

The beta files use other letters for the same node types. See [File format](file-format.md#node-letters).

For the opcodes, see the counts in [Opcodes](bytecode.md#opcodes).
