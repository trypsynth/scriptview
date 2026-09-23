# The syntax tree

A compiled script that isn't run-only keeps a full syntax tree. This is what Script Editor shows you when you open the file. It doesn't keep your source text. It rebuilds the text from the tree each time, which is why a compiled script always comes back with the same indentation.

## Nodes

Each node is a `0d` record:

```
kind (1 byte)  flags (2 bytes)  start (2 bytes)  end (2 bytes)  references
```

The kind is one ASCII character. `start` and `end` are the part of the bytecode that the node compiled to. We don't need them to print the source, but they helped us line up the tree with the bytecode.

These are the kinds we know:

| Kind | Meaning | References |
| --- | --- | --- |
| `k` | A block of statements | a list of statements |
| `l` | A source line, or a wrapper around an expression | see below |
| `r` | `set` | value, target |
| `s` | `copy` | value, target |
| `L` | `return` | value |
| `Z` | `if` | condition, then block, else-if chain, else block |
| `O` | `tell` | body, target |
| `Q` | `try` | body, `on error` clause, handler body |
| `U` | `repeat n times` | body, count |
| `V` | `repeat while` | body, condition |
| `W` | `repeat until` | body, condition |
| `X` | `repeat with x in list` | body, variable, list |
| `Y` | `repeat with x from a to b` | body, variable, from, to, step |
| `P` | `considering` / `ignoring` | body, considered, ignored |
| `t` | `with timeout` | body, seconds |
| `u` | `with transaction` | body, session |
| `w` | `using terms from` | body, application |
| `x` | `use` | name, target, options |
| `i` | A handler definition | signature, body |
| `h` | A script object | name, body |
| `j` | `property` | name, value |
| `p`, `q` | `global`, `local` | names |
| `I` | A call | name, positional arguments, labeled arguments |
| `e` | An explicit `get` | value |
| `n` | `x of y` | part, container |
| `1` | A property or class name | term |
| `2`, `3` | `every x`, `some x` | class |
| `4` | `x 3` (by index) | class, index |
| `5` | `x id 3`, `x named "a"`, `x index 3` | class, value, key form |
| `6` | `x whose ...` | reference, condition |
| `7` | `x 1 thru 3` | class, from, to |
| `c` | `as` | value, class |
| `d` | Unary minus | value |
| `H` | `not` | value |
| `J` | A list | list cells |
| `K` | A record | key/value cells |
| `m` | A literal | the value |
| `o` | A variable | identifier |
| `f` | `my` | |
| `g` | `it` | |

Binary operators use punctuation for their kind: `[` is `+`, `\` is `-`, `]` is `*`, `b` is `&`, `=` is equals, `>` is not equals, `A` is `<`, `?` is `>`, `B` is `≤`, `@` is `≥`, and so on.

## The `l` node

The `l` node does two jobs, and you can only tell them apart from where it is.

In a statement list, an `l` node is a source line: `[statement, comment, comment text]`. An empty one is a blank line. The low three bits of its flags say what kind of comment is on the line:

| Value | Comment |
| --- | --- |
| 0 | `(* ... *)` |
| 1 | `-- ...` |
| 2 | no comment |
| 3 | `-- ...` on the header line of the block that follows |
| 4 | `(* ... *)` on the header line |
| 6 | `# ...` |
| 7 | `# ...` on the header line |

Inside an expression, an `l` node is a wrapper that records how you wrote the expression. Bit 4 means parentheses, bit 8 means a `¬` line break before it, and bit `0x100` means you wrote the word `the`.

Bit 1 on a statement line means "line comment", and on an expression wrapper it can mean something else. We had to check both the flags and the children before we knew what a node was.

## Flags record how you wrote things

Most of the surface syntax is in the flags. The same node kind prints differently depending on them. Some examples:

- `=` with flags 3 prints as `is`, with flag 1 as `is equal to`, and with no flags as `=`.
- `A` (`<`) with flag 1 is `is less than`, with `0x101` it's `comes before`, and with `0x100` it's `is not greater than or equal to`. The compiler turns the negation into the opposite operator and keeps a flag so it can print your words again.
- `n` with flag 2 is `x's y`, and with flag 1 and a `my` container it's `my y`.
- `n` with `0x100` is `y in x` instead of `y of x`.
- `4` with flag 2 is `1st item` instead of `item 1`.
- `O` (`tell`) and `Z` (`if`) with flag 1 are the one-line forms: `tell x to ...` and `if c then ...`.
- A call with flags 3 is written after its argument, as in `x exists`.

If you zero all the flags, you get AppleScript's default spelling for everything. We use that to compare our bytecode decompiler with the tree: the bytecode has no flags, so it can only produce the default spelling.

## Line breaks

A `¬` continuation is a flag on an expression wrapper. `8` means "a line break comes before this", and `8` plus `2` means "a line break comes after this". The indent of the next line depends on how deep the break is. We print a marker while we render an expression and replace it with the break and the indent at the end.

Script Editor indents blank lines inside a block, so a blank line in a handler is a tab and nothing else.

## Comments

A comment line keeps two copies of the text: a Mac Roman `0c` string and a UTF-16 copy. We use the UTF-16 copy, because the Mac Roman one loses any character that Mac Roman doesn't have.

Some old scripts are stranger. We found comments whose Mac Roman record held UTF-16 bytes, so every other byte was NUL. `osadecompile` prints those NUL bytes as they are, and so do we. In one file, the comment's Mac Roman reference pointed at an unrelated node. The UTF-16 copy was still right, so we read that one first.

## Old compilers

The tree changed over the years, and old scripts still load. Some things we found:

- Old compilers kept `it's` when you wrote it. New ones change it to `its` and add the comment `-- Grammar Police` to the line. Really.
- Old compilers sometimes left out a parentheses wrapper that a new compiler would add. `osadecompile` still prints the parentheses in some of those places, because its printer adds them by its own rules. See [Output quirks](quirks.md).
- Old compilers stored some properties as literal term nodes (`m`) where new ones use a property node (`1`).
- A handler body can be a single expression wrapper with no block around it. We saw this with records written across several lines, with `¬` breaks before the first line.
