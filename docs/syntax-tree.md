# Syntax tree

A compiled script that isn't run-only keeps a full syntax tree of its source. Script Editor and `osadecompile` print the source from this tree each time you open the script. The file doesn't keep the source text itself, which is why a compiled script always comes back with the same indentation.

The tree is language neutral. It stores structure and four-character codes, and the dialect prints them as words. To keep your wording, the tree records it in flags on each node: whether you wrote `is` or `=`, `x's y` or `y of x`, and where you put parentheses and line breaks.

## Node records

Each node is a `0d` record:

| Bytes | Field |
| --- | --- |
| 1 | Node type: one ASCII character |
| 2 | Flags |
| 2 | Code start: the offset of the node's first instruction in its handler's bytecode |
| 2 | Code end |
| 2 each | The children, as references |

The size field of the record is the number of children. The code offsets aren't needed to print the source, but they line the tree up with the bytecode.

## Node types

Statements:

| Type | Statement | Children |
| --- | --- | --- |
| `k` | A block of statements | A list of lines |
| `l` | A source line, or a wrapper around an expression | See [The `l` node](#the-l-node) |
| `r` | `set` | Value, target |
| `s` | `copy` | Value, target |
| `L` | `return` | Value |
| `Z` | `if` | Condition, then block, `else if` chain, else block |
| `O` | `tell` | Body, target |
| `Q` | `try` | Body, `on error` clause, error handler body |
| `R` | `error` | A first child that printing doesn't use, the message, labeled parameters |
| `S` | `exit repeat` | None |
| `T` | `repeat` | Body |
| `U` | `repeat n times` | Body, count |
| `V` | `repeat while` | Body, condition |
| `W` | `repeat until` | Body, condition |
| `X` | `repeat with x in list` | Body, variable, list |
| `Y` | `repeat with x from a to b` | Body, variable, start, end, step |
| `P` | `considering` or `ignoring` | Body, considered attributes, ignored attributes |
| `t` | `with timeout` | Body, seconds |
| `u` | `with transaction` | Body, session |
| `w` | `using terms from` | Body, application |
| `x` | `use` | Name, target, options |
| `i` | A handler definition | Signature, body |
| `h` | A script object | Name, body |
| `j` | `property` | Name, initial value |
| `p` | `global` | A list of names |
| `q` | `local` | A list of names |

Expressions:

| Type | Expression | Children |
| --- | --- | --- |
| `I` | A command or handler call | Name, direct or positional arguments, labeled arguments |
| `M` | `continue` | The call |
| `e` | An explicit `get` | Value |
| `m` | A literal | The value |
| `o` | A variable | Identifier |
| `f` | `me` | None |
| `g` | `it` | None |
| `J` | A list `{…}` | A chain of list cells |
| `v` | A linked list `[…]` | A chain of list cells |
| `K` | A record | A chain of key, value and next cells |
| `c` | `as` | Value, class |
| `d` | Unary minus | Value |
| `H` | `not` | Value |
| `N` | `a reference to` | Value |

References:

| Type | Reference | Children |
| --- | --- | --- |
| `n` | `x of y` | Part, container |
| `1` | A property, or a class used as a value | Term |
| `2` | `every x` | Class |
| `3` | `some x` | Class |
| `4` | `x 3`, by index | Class, index |
| `5` | `x id 3`, `x named "a"`, `x index 3` | Class, value, key form |
| `6` | `x whose …` | Reference, condition |
| `7` | `x 1 thru 3` | Class, start, end |
| `8`, `9` | `before x`, `after x` | Class |
| `:`, `;`, `<` | `beginning`, `end`, `middle` | None, except `middle`: class |

Binary operators, each with two children, left and right:

| Type | Operator | Type | Operator |
| --- | --- | --- | --- |
| `a` | `^` | `A` | `<` |
| `]` | `*` | `B` | `≤` |
| `^` | `/` | `?` | `>` |
| `_` | `div` | `@` | `≥` |
| `` ` `` | `mod` | `=` | `=` |
| `[` | `+` | `>` | `≠` |
| `\` | `-` | `C` | `starts with` |
| `b` | `&` | `D` | `ends with` |
| `F` | `and` | `E` | `contains` |
| `G` | `or` | | |

The operators bind in this order, from tightest to loosest: `^`; `*`, `/`, `div` and `mod`; `+` and `-`; `&`; the comparisons and `starts with`, `ends with` and `contains`; `=` and `≠`; `and`; `or`.

The beta of AppleScript 1.0 uses different letters. See [File format](file-format.md#the-applescript-10b1-beta).

## The `l` node

The `l` node has two jobs, and its position tells you which one.

In a list of statements, an `l` node is a source line, with the children `[statement, comment, comment text]`. An `l` node with no statement and no comment is a blank line. The low three bits of its flags give the kind of comment on the line:

| Value | Comment |
| --- | --- |
| 0 | `(* … *)` |
| 1 | `-- …` |
| 2 | No comment |
| 3 | `-- …` on the header line of the block that follows |
| 4 | `(* … *)` on the header line |
| 6 | `# …` |
| 7 | `# …` on the header line |

Inside an expression, an `l` node is a wrapper that records how you wrote the expression:

| Bit | Meaning |
| --- | --- |
| `0x4` | The expression is in parentheses. |
| `0x8` | A `¬` line break comes before the expression. |
| `0x100` | You wrote the article `the`. |

Bit `0x1` means a line comment on a statement line, but something else on an expression wrapper. To know what an `l` node is, check both its flags and its children.

## Flags

Most of the surface syntax is in the flags. The same node type prints differently depending on them:

| Node | Flags | Prints as |
| --- | --- | --- |
| `=` | none, `0x1`, `0x3` | `=`, `is equal to`, `is` |
| `>` | `0x100`, `0x1` | `is not`, `is not equal to` |
| `A` | `0x1`, `0x101`, `0x100` | `is less than`, `comes before`, `is not greater than or equal to` |
| `C` | `0x100` | `begins with` |
| `n` | `0x2` | `x's y` |
| `n` | `0x1`, with a `me` container | `my y` |
| `n` | `0x100` | `y in x` |
| `4` | `0x2` | `1st item` instead of `item 1` |
| `4` | `0x1` | `first item`, `last item` |
| `4` | `0x100` | `front item`, `back item` |
| `:`, `;` | `0x100` | `front`, `back` instead of `beginning`, `end` |
| `O`, `Z` | `0x1` | The one-line forms, `tell x to …` and `if c then …` |
| `I` | `0x3` | A call written after its argument, as in `x exists` |

The compiler turns a negated comparison into the opposite operator and keeps a flag, so that it can print your words again: `is not greater than or equal to` is a `<` node with the flag `0x100`.

With all flags set to zero, every node prints in AppleScript's default spelling. Bytecode has no flags, so a script rebuilt from bytecode always comes out in the default spelling.

## Line breaks

A `¬` continuation is a flag on an expression wrapper: `0x8` means a break before the expression, and `0x8` with `0x2` means a break after it. The indent of the next line depends on how deep in the expression the break is.

Script Editor indents blank lines inside a block, so a blank line inside a handler is a tab and nothing else.

## Comments

A comment line keeps two copies of the text: a byte string (`0c`) and a UTF-16 copy. Read the UTF-16 copy, because the byte string loses any character that its encoding can't represent.

Some old scripts are irregular:

- Some comments have UTF-16 bytes in their byte string, so every other byte is NUL. `osadecompile` prints the NUL bytes as they are.
- In one file, a comment's byte string reference pointed at an unrelated node. The UTF-16 copy was still correct.

## Older compilers

The tree changed over the years, and old scripts still load:

- Old compilers kept `it's` when you wrote it. Current ones change it to `its`, and add the comment `-- Grammar Police` to the line.
- Old compilers sometimes left out a parentheses wrapper that a current compiler adds. `osadecompile` still prints parentheses in some of those places, because its printer adds them by its own rules. See [Output quirks](quirks.md).
- Old compilers stored some properties as literal term nodes (`m`), where current ones use a property node (`1`).
- A handler body can be a single expression wrapper with no block around it. This happens with records written across several lines, with `¬` breaks before the first line.
