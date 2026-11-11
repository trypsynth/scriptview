# Corpus statistics

This page counts how often each part of the format occurs in real scripts. Use it to judge which structures matter: a record type in every file needs care, and one in three files needs a test fixture more than a fast path.

The counts come from scriptview's own parser, run over four sets of scripts:

| Set | Scripts | Source |
| --- | --- | --- |
| GitHub | 12,150 | Compiled scripts and compiled sources from 4,535 GitHub repositories, compiled on macOS 26.5 |
| macOS | 163 | The compiled scripts in `/Library/Scripts` and `/System/Library` on macOS 26.5 |
| AppleScript 1.0 | 117 | Scripts from Apple's 1993 and 1994 discs, with the versions 0.98 and 1.00 |
| AppleScript 1.1 | 33 | Classic Mac OS scripts from the same discs, with the version 1.10 |

The AppleScript 1.0b1 beta set (26 scripts) appears in the tables where it differs. For how the sets were built, see [Testing](testing.md).

In the tables, each count is the number of scripts that contain the structure at least once. A dash means none.

## Headers and roots

| Header | GitHub | macOS | 1.0 | 1.1 |
| --- | --- | --- | --- | --- |
| `FasdUAS 1.101.10` | 12,150 | 163 | — | 33 |
| `FasdUAS 1.00` | — | — | 116 | — |
| `FasdUAS 0.98` | — | — | 1 | — |

| Root object | GitHub | macOS | 1.0 | 1.1 |
| --- | --- | --- | --- | --- |
| Kind 15 script object, 4 items | 12,146 | 163 | 53 | 33 |
| Kind 16 handler, 7 items (run-only, only a run handler) | — | — | 64 | — |

Run-only scripts: 110 in the GitHub set, 64 in the AppleScript 1.0 set (the PlainTalk speech rules), and none in the others.

## Record types

| Type | Name | GitHub | macOS | 1.0 | 1.1 |
| --- | --- | --- | --- | --- | --- |
| `01` | Symbol | 12,147 | 163 | 117 | 33 |
| `02` | List cell | 12,125 | 163 | 117 | 33 |
| `0e` | Typed vector | 12,146 | 163 | 117 | 33 |
| `10` | Vector | 12,146 | 163 | 117 | 33 |
| `11` | Raw bytes | 12,140 | 163 | 116 | 33 |
| `0a` | Term | 12,076 | 162 | 79 | 33 |
| `0d` | Syntax node | 12,036 | 163 | 53 | 33 |
| `0b` | Identifier | 10,694 | 156 | 77 | 33 |
| `03` | Small integer | 10,424 | 148 | 79 | 32 |
| `0f` | Descriptor | 9,055 | 147 | 29 | 24 |
| `0c` | Byte string | 8,487 | 155 | 65 | 28 |
| `06` | Binding cell | 8,149 | 152 | 32 | 17 |
| `08` | Real | 1,703 | 6 | 11 | 1 |
| `07` | Integer | 306 | 21 | 4 | — |
| `13` | Long raw bytes | 15 | — | — | — |
| `12` | Long descriptor | 3 | — | — | — |

No script uses the types `04` or `09`.

Symbols with a 4-byte payload, which index the runtime's symbol table, occur only in AppleScript 1.0 files: 115 of the 117.

## Typed vector kinds

| Kind | Meaning | GitHub | macOS | 1.0 | 1.1 |
| --- | --- | --- | --- | --- | --- |
| 177 | UTF-16 string | 11,172 | 160 | — | — |
| 16 | Handler | 12,084 | 162 | 116 | 33 |
| 15 | Script object | 12,146 | 163 | 53 | 33 |
| 4 | Positional parameters | 6,134 | 131 | — | 20 |
| 0 | Plain list | 6,125 | 131 | — | 20 |
| 19 | `it` placeholder in a filter | 2,511 | — | 2 | 2 |
| 20 | `a reference to` constant | 2,103 | 4 | 2 | 2 |
| 24 | Index reference constant | 1,692 | 4 | 2 | 2 |
| 22 | `every` constant | 1,563 | — | — | — |
| 17 | Closure | 185 | — | 1 | — |
| 25 | Key reference constant | 171 | — | — | — |
| 21 | Property reference constant | 157 | — | 1 | 2 |
| 122 | Command name with target | 142 | — | — | — |
| 123 | Typed or default parameter | 79 | — | — | — |
| 124 | Not understood | 47 | — | — | — |
| 26 | Filter constant | 5 | — | — | — |
| 33, 36, 40 | Comparison constants | 4, 1, 1 | — | — | — |
| 27 | Range constant | 3 | — | — | — |

AppleScript 1.0 has no kind 177 strings and no kind 4 parameter lists: it stores strings as byte strings and parameters as plain names.

## Term kinds

| Kind | Meaning | GitHub | macOS | 1.0 | 1.1 | Beta |
| --- | --- | --- | --- | --- | --- | --- |
| `0a` | Class, property or constant | 11,340 | 161 | 35 | 33 | 19 |
| `2e` | Event | 11,958 | 162 | 62 | 33 | — |
| `0b` | Typed constant | 7,609 | 109 | 32 | 15 | — |
| `2f` | Class, AppleScript 1.0 | — | — | 67 | — | — |
| `31` | Event, beta | — | — | — | — | 25 |
| `32` | Class, beta | — | — | — | — | 19 |

## Descriptors

| Kind and type | Meaning | GitHub | macOS | 1.0 | 1.1 |
| --- | --- | --- | --- | --- | --- |
| `08` | Application specifier with an alias | 8,705 | 145 | — | — |
| `08`, type `null` | Application specifier | 312 | 2 | 27 | 23 |
| `0d` `ldt ` | Date | 72 | — | 2 | — |
| `0d` `alis` | Alias | 87 | 5 | 3 | 3 |
| `0d` `utxt` | Unicode text | 18 | 2 | — | — |
| `0d` `furl` | File URL | 13 | — | — | — |
| `0d`, image types | `PICT`, `TIFF`, `JPEG`, `sPic` | 6 | — | — | — |

## Syntax node types

The 20 most common node types in the GitHub set, out of the 12,036 scripts that have a syntax tree:

| Type | Meaning | Scripts | Type | Meaning | Scripts |
| --- | --- | --- | --- | --- | --- |
| `l` | Line or wrapper | 12,036 | `O` | `tell` | 9,372 |
| `k` | Block | 11,981 | `4` | By index | 8,739 |
| `m` | Literal | 11,936 | `Z` | `if` | 7,801 |
| `I` | Call | 11,482 | `b` | `&` | 6,779 |
| `o` | Variable | 10,521 | `=` | `=` | 6,749 |
| `r` | `set` | 10,279 | `i` | Handler | 6,332 |
| `n` | `x of y` | 9,973 | `L` | `return` | 6,166 |
| `1` | Property | 9,526 | `J` | List | 5,712 |
| `R` | `error` | 5,338 | `c` | `as` | 5,273 |
| `Q` | `try` | 5,127 | `2` | `every` | 5,009 |

The rarest types are `<` (`middle`, 7 scripts), `u` (`with transaction`, 8), `8` (`before`, 51) and `9` (`after`, 59).

The beta files use two node types that no later version has, `y` and `z`.

For the opcodes, see the Scripts column in [Opcodes](bytecode.md#opcodes).
