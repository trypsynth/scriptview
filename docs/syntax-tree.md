# Syntax tree

A compiled script that isn't run-only keeps a full syntax tree of its source. Script Editor and `osadecompile` print the source from this tree each time you open the script. The file doesn't keep the source text itself, which is why a compiled script always comes back with the same indentation.

The tree is language neutral. It stores structure and four-character codes, and the dialect prints them as words. To keep your wording, the tree records it in flags on each node: whether you wrote `is` or `=`, `x's y` or `y of x`, and where you put parentheses and line breaks.

## Node records

Each node is a `0d` record. After the usual record header, it holds:

- The node type: one ASCII character.
- The flags: 2 bytes.
- The code start: 2 bytes, the offset of the node's first instruction in its handler's bytecode.
- The code end: 2 bytes.
- The children: 2 bytes each, as references.

The size field of the record is the number of children. The code offsets aren't needed to print the source, but they line the tree up with the bytecode.

## Node types

### Statements

- `k`: a block of statements. Its child is a list of lines.
- `l`: a source line, or a wrapper around an expression. See [The `l` node](#the-l-node).
- `r`: `set`, with the value, then the target.
- `s`: `copy`, with the value, then the target.
- `L`: `return`, with the value.
- `Z`: `if`, with the condition, the then block, the `else if` chain, and the else block. The chain is a list that alternates conditions and blocks.
- `O`: `tell`, with the body, then the target.
- `Q`: `try`, with the body, the `on error` clause (an `R` node), and the error handler's body.
- `R`: `error`, with a first child that printing doesn't use, the message, and the labeled parameters.
- `S`: `exit repeat`. No children.
- `T`: `repeat`, with the body.
- `U`: `repeat n times`, with the body and the count.
- `V`: `repeat while`, with the body and the condition.
- `W`: `repeat until`, with the body and the condition.
- `X`: `repeat with x in list`, with the body, the variable, and the list.
- `Y`: `repeat with x from a to b`, with the body, the variable, the start, the end, and the step.
- `P`: `considering` or `ignoring`, with the body, the considered attributes, and the ignored attributes.
- `t`: `with timeout`, with the body and the seconds.
- `u`: `with transaction`, with the body and the session.
- `w`: `using terms from`, with the body and the application.
- `x`: `use`, with the name, the target, and the options.
- `i`: a handler definition, with the signature (an `I` node) and the body.
- `h`: a script object, with the name and the body.
- `j`: `property`, with the name and the initial value.
- `p`: `global`, with a list of names.
- `q`: `local`, with a list of names.

### Expressions

- `I`: a command or handler call, with the name, the direct or positional arguments, and the labeled arguments.
- `M`: `continue`, with the call.
- `e`: an explicit `get`, with the value.
- `m`: a literal, with the value.
- `o`: a variable, with its identifier.
- `f`: `me`. No children.
- `g`: `it`. No children.
- `J`: a list, `{…}`, with a chain of list cells.
- `v`: a linked list, `[…]`, with a chain of list cells.
- `K`: a record, with a chain of cells that each hold a key, a value, and the next cell.
- `c`: `as`, with the value and the class.
- `d`: unary minus, with the value.
- `H`: `not`, with the value.
- `N`: `a reference to`, with the value.

### References

- `n`: `x of y`, with the part, then the container.
- `1`: a property, or a class used as a value, with the term.
- `2`: `every x`, with the class.
- `3`: `some x`, with the class.
- `4`: an element by index, `x 3`, with the class and the index.
- `5`: an element by key, `x id 3`, `x named "a"`, or `x index 3`, with the class, the value, and the key form.
- `6`: a filter, `x whose …`, with the reference and the condition.
- `7`: a range, `x 1 thru 3`, with the class, the start, and the end.
- `8` and `9`: `before x` and `after x`, with the class.
- `:`, `;`, and `<`: `beginning`, `end`, and `middle`. Only `middle` has a child, the class.

### Operators

Each operator node has two children, the left and right operands:

- `a`: `^`
- `]`: `*`
- `^`: `/`
- `_`: `div`
- `` ` ``: `mod`
- `[`: `+`
- `\`: `-`
- `b`: `&`
- `A`: `<`
- `B`: `≤`
- `?`: `>`
- `@`: `≥`
- `=`: `=`
- `>`: `≠`
- `C`: `starts with`
- `D`: `ends with`
- `E`: `contains`
- `F`: `and`
- `G`: `or`

The operators bind in this order, from tightest to loosest: `^`; `*`, `/`, `div`, and `mod`; `+` and `-`; `&`; the comparisons and `starts with`, `ends with`, and `contains`; `=` and `≠`; `and`; `or`. `as` binds tighter than `*` but looser than `^`.

`^` groups to the right: `2 ^ 3 ^ 2` means `2 ^ (3 ^ 2)`. The other operators group to the left. A printer that adds parentheses has to know both, because the tree stores the grouping, not the parentheses.

The beta of AppleScript 1.0 uses different letters for the same nodes. See [Node letters](file-format.md#node-letters).

## The `l` node

The `l` node has two jobs, and its position tells you which one.

In a list of statements, an `l` node is a source line, with the children `[statement, comment, comment text]`. An `l` node with no statement and no comment is a blank line. The low three bits of its flags give the kind of comment on the line:

- 0: a `(* … *)` comment.
- 1: a `-- …` comment.
- 2: no comment.
- 3: a `-- …` comment on the header line of the block that follows.
- 4: a `(* … *)` comment on the header line.
- 6: a `# …` comment.
- 7: a `# …` comment on the header line.

Inside an expression, an `l` node is a wrapper that records how you wrote the expression:

- `0x4`: the expression is in parentheses.
- `0x8`: a `¬` line break comes before the expression.
- `0x100`: you wrote the article `the`.

Bit `0x1` means a line comment on a statement line, but something else on an expression wrapper. To know what an `l` node is, check both its flags and its children.

## Flags

Most of the surface syntax is in the flags. The same node type prints differently depending on them:

- `=`: with no flags, `=`. With `0x1`, `is equal to`. With `0x3`, `is`.
- `>`: with `0x100`, `is not`. With `0x1`, `is not equal to`.
- `A`: with `0x1`, `is less than`. With `0x101`, `comes before`. With `0x100`, `is not greater than or equal to`.
- `C`: with `0x100`, `begins with`.
- `n`: with `0x2`, `x's y`. With `0x1` and a `me` container, `my y`. With `0x100`, `y in x`.
- `4`: with `0x2`, `1st item` instead of `item 1`. With `0x1`, `first item` or `last item`. With `0x100`, `front item` or `back item`.
- `:` and `;`: with `0x100`, `front` and `back` instead of `beginning` and `end`.
- `O` and `Z`: with `0x1`, the one-line forms, `tell x to …` and `if c then …`.
- `I`: with `0x3`, a call written after its argument, as in `x exists`.

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
