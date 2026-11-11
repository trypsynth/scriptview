# Bytecode

Every compiled script has bytecode, including scripts that keep a syntax tree. The bytecode is what runs. A run-only script is a normal script without its syntax tree, so the bytecode is the only way to read it. `osacompile -x` and Script Editor's run-only option both produce one. Inside Macintosh describes run-only data as "only the minimum script data required to run the script."

The opcode names come from Jinmo's [applescript-disassembler](https://github.com/Jinmo/applescript-disassembler), which ports AppleScript's own opcode table. The operand sizes on this page were checked against the code blocks of more than 12,000 real scripts.

## Handlers

Each handler is a typed vector of kind 16, or kind 17 for a closure, with seven items:

```
{name, nil, positional parameters, labeled parameters, variable names, literals, code}
```

- Name: an identifier, or an event code for handlers such as `on run` and `on open`.
- Positional parameters: a kind 4 vector, `{count, {names}}`. A bare name instead is a direct parameter, of an event handler (`on open x`) or a labeled handler (`on mk of x by y`). A list of names is a pattern, as in `on run {a, b}`. AppleScript 1.0 lists positional parameters as plain names.
- Labeled parameters: the labels, and the names they bind.
- Variable names: the handler's parameters and local variables. Instructions refer to a variable by its index in this list.
- Literals: numbers, strings, terms, handler names, and nested handlers and script objects. Instructions refer to a constant by its index in this list.
- Code: a raw `11` record.

A closure is a handler whose local variables a script object inside it still uses after the handler returns. Both kinds decompile the same way.

The top level of a script is its implicit `run` handler. The compiler puts the implicit `run` handler after every other handler in the table, so a `run` handler anywhere else was an explicit `on run`.

## Instruction encoding

The machine is stack based. Each instruction is one opcode byte, followed by zero, one or two 16-bit operand words.

The bytes `a0` to `ff` pack a small operand into the opcode itself:

| Bytes | Instruction | Operand |
| --- | --- | --- |
| `a0` to `af` | `PushVariable` | The low 4 bits: a variable index |
| `b0` to `bf` | `PopVariable` | The low 4 bits: a variable index |
| `c0` to `cf` | `PushGlobal` | The low 4 bits: the index of the literal that names the global |
| `d0` to `df` | `PopGlobal` | The low 4 bits: the index of the literal that names the global |
| `e0` to `ff` | `PushLiteral` | The low 5 bits: a literal index |

Larger indexes use the `Extended` forms, which take a 16-bit word.

A branch operand is a signed byte offset that counts from the byte right after the opcode, not from the next instruction. `Jump`, `TestIf`, `And`, `Or`, `LinkRepeat`, `ErrorHandler` and `EndErrorHandler` take one. The words of `Tell`, `Consider` and `BeginTransaction` are offsets too: they point at the matching end instruction.

`PopVariable`, `PopGlobal` and `SetData` store the top of the stack without removing it. The compiler follows them with `Pop` when it doesn't need the value.

## Opcodes

In the following table, Words is the number of 16-bit operand words. Scripts is the number of scripts in the GitHub corpus, out of 12,150, that use the instruction. A dash means that no script in the corpus uses it.

| Byte | Instruction | Words | Description | Scripts |
| --- | --- | --- | --- | --- |
| `00` | `Equal` | 0 | `=` | 6,089 |
| `01` | `NotEqual` | 0 | `≠` | 2,434 |
| `02` | `GreaterThan` | 0 | `>` | 1,741 |
| `03` | `GreaterThanOrEqual` | 0 | `≥` | 512 |
| `04` | `LessThan` | 0 | `<` | 1,173 |
| `05` | `LessThanOrEqual` | 0 | `≤` | 372 |
| `06` | `StartsWith` | 0 | `starts with` | 608 |
| `07` | `EndsWith` | 0 | `ends with` | 396 |
| `08` | `Contains` | 0 | `contains`. `is in` swaps the operands. | 2,734 |
| `09` | `And` | 1 | `and`, with a branch that skips the right side | 1,140 |
| `0a` | `Or` | 1 | `or`, with a branch that skips the right side | 977 |
| `0b` | `Not` | 0 | `not` | 2,560 |
| `0c` | `MessageSend` | 1 | Sends a command or a labeled handler call. The word is the literal index of the event or handler name. | 11,103 |
| `0d` | `MakeList` | 0 | Builds a `[…]` list from a count and items. AppleScript 1.0 builds every list this way. | 130 |
| `0e` | `MakeRecord` | 0 | Builds a record from a count and key–value pairs | 1,564 |
| `0f` | `Return` | 0 | Returns the top of the stack | 11,866 |
| `10` | `Continue` | 1 | `continue`: passes a labeled call to the parent script | 40 |
| `11` | `ObjectAliasQuote` | 0 | `a reference to` | 100 |
| `12` | `Tell` | 1 | Starts a `tell` block. The word points at the `EndTell`. | 9,342 |
| `13` | `Consider` | 1 | Starts a `considering` or `ignoring` block | 250 |
| `14` | `ErrorHandler` | 1 | Starts a `try` block. The word points at the `on error` code. | 5,008 |
| `15` | `Error` | 0 | `error` | 2,031 |
| `16` | `Exit` | 0 | `exit repeat` | 1,133 |
| `17` | `LinkRepeat` | 1 | Starts a loop. The word points past its end. | 4,896 |
| `18` | `RepeatNTimes` | 0 | `repeat n times` | 600 |
| `19` | `RepeatWhile` | 0 | `repeat while` | 527 |
| `1a` | `RepeatUntil` | 0 | `repeat until` | 421 |
| `1b` | `RepeatInCollection` | 1 | `repeat with x in list`. The word is the loop variable. | 3,063 |
| `1c` | `RepeatInRange` | 1 | `repeat with x from a to b`. The word is the loop variable. | 1,634 |
| `1d` | `TestIf` | 1 | Branches when the top of the stack is false | 7,666 |
| `1e` | `Add` | 0 | `+` | 2,070 |
| `1f` | `Subtract` | 0 | `-` | 1,447 |
| `20` | `Multiply` | 0 | `*` | 930 |
| `21` | `Divide` | 0 | `/` | 580 |
| `22` | `Quotient` | 0 | `div` | 271 |
| `23` | `Remainder` | 0 | `mod` | 280 |
| `24` | `Power` | 0 | `^` | 64 |
| `25` | `Concatenate` | 0 | `&` | 6,660 |
| `26` | `Coerce` | 0 | `as` | 5,519 |
| `27` | `Negate` | 0 | Unary minus | 170 |
| `28` | `GetData` | 0 | Evaluates a reference | 10,056 |
| `29` | `PushMe` | 0 | Pushes `me` | 4,422 |
| `2a` | `PushIt` | 0 | Pushes `it`: the `tell` target, or the default target | 10,115 |
| `2b` | `PositionalMessageSend` | 1 | Calls a handler with positional arguments, `f(x, y)`. The word is the literal index of the name. | 5,350 |
| `2c` to `37` | `MakeObjectAlias` | 0 | Builds an object reference. The low bits give the form. See [References](#references). | 10,429 |
| `38` to `44` | `MakeComp` | 0 | Builds a comparison inside a `whose` filter. See [References](#references). | 2,444 |
| `45` | `GetData` | 0 | A second opcode with the same name as `28` | — |
| `46` | `SetData` | 0 | `set` a reference to a value | 4,225 |
| `47` | `CopyData` | 0 | `copy` a value to a reference | 255 |
| `4a` | `PositionalContinue` | 1 | `continue f(x)` | — |
| `4b` | `DefineActor` | 1 | Starts a `script` object definition | 264 |
| `4c` | `DefineProcedure` | 1 | Defines a handler | — |
| `4d` | `DefineClosure` | 0 | Defines a closure | — |
| `4e` | `DefineProperty` | 1 | Defines a property | — |
| `4f` | `StoreResult` | 0 | Stores the top of the stack as `result`, after a statement | 10,882 |
| `50` | `GetResult` | 0 | Pushes `result` | 3,027 |
| `51` | `Clone` | 0 | Copies a value, for `copy` | 172 |
| `52` | `Of` | 0 | Unknown | — |
| `53` | `EndDefineActor` | 0 | Ends a `script` object definition | 264 |
| `54` | `EndOf` | 0 | Unknown | — |
| `55` | `EndTell` | 0 | Ends a `tell` block | 9,342 |
| `56` | `EndConsider` | 0 | Ends a `considering` or `ignoring` block | 250 |
| `57` | `EndErrorHandler` | 1 | Ends the body of a `try` block. The word points past the `on error` code. | 5,008 |
| `58` | `HandleError` | 2 | Starts the `on error` code. The words are the literal indexes of the error variable and of the labeled parameter bindings. | 5,008 |
| `59` | `Jump` | 1 | Branches | 8,418 |
| `5a` | `Pop` | 0 | Drops the top of the stack | 4,206 |
| `5b` | `Dup` | 0 | Duplicates the top of the stack | 6,752 |
| `5c` | `GCSwap` | 0 | Swaps the top two values | 3,567 |
| `5d` | `PushVariableExtended` | 1 | `PushVariable` with a 16-bit index | 333 |
| `5e` | `PopVariableExtended` | 1 | `PopVariable` with a 16-bit index | 306 |
| `5f` | `PushGlobalExtended` | 1 | `PushGlobal` with a 16-bit index | 4,364 |
| `60` | `PopGlobalExtended` | 1 | `PopGlobal` with a 16-bit index | 4,019 |
| `61` | `PushLiteralExtended` | 1 | `PushLiteral` with a 16-bit index | 7,369 |
| `62` | `PushParentVariable` | 2 | Pushes a variable of an enclosing script object. The words are the scope level and the index. | 2,282 |
| `63` | `PopParentVariable` | 2 | Stores into a variable of an enclosing script object | 457 |
| `64` | `PushNext` | 0 | Marks where `continue` passes its arguments | 40 |
| `65` | `PushTrue` | 0 | Pushes `true` | 4,771 |
| `66` | `PushFalse` | 0 | Pushes `false` | 3,939 |
| `67` | `PushEmpty` | 0 | Pushes an empty list | 242 |
| `68` | `PushUndefined` | 0 | Pushes no value, as an empty result slot | 8,081 |
| `69` | `PushMinus1` | 0 | Pushes -1 | 1,182 |
| `6a` | `Push0` | 0 | Pushes 0 | 10,853 |
| `6b` | `Push1` | 0 | Pushes 1 | 9,365 |
| `6c` | `Push2` | 0 | Pushes 2 | 7,893 |
| `6d` | `Push3` | 0 | Pushes 3 | 3,285 |
| `6e` | `BeginTimeout` | 0 | Starts a `with timeout` block | 202 |
| `6f` | `EndTimeout` | 0 | Ends a `with timeout` block | 202 |
| `70` | `BeginTransaction` | 1 | Starts a `with transaction` block | 8 |
| `71` | `EndTransaction` | 0 | Ends a `with transaction` block | 8 |
| `75` | `MatchLiteral` | 1 | Checks one slot of a pattern that holds a literal, as in `set {a, 5} to x`. The word is a literal index. | 4 |
| `76` | `MakeVector` | 0 | Builds a `{…}` list from a count and items | 5,357 |

The bytes `48`, `49`, `72` to `74` and `77` to `9f` aren't instructions.

## Commands

A command pushes its arguments, then runs `MessageSend` with the literal index of the event:

```
direct parameter (or PushIt), then for each labeled parameter: key, value, then the count
```

The count is the number of keys and values together, which is twice the number of labeled parameters. It doesn't include the direct parameter.

A call to a handler with positional arguments, such as `f(1, 2)`, runs `PositionalMessageSend` with the handler's name as the literal. The stack holds the target (`PushIt` for a plain call, `PushMe` for `my f()`), the arguments, then the count. A call to a handler with labeled parameters, such as `f from 1 to 2` or `f given x:1`, uses `MessageSend` with the handler's name in place of an event.

`Continue` and `PositionalContinue` do the same for `continue f()`, which passes the call on to the parent script. Their stack is laid out differently: the target and the labeled parameters come first, then `PushNext`, then the direct parameter or `PushIt`.

`set {year:y, month:m} to current date` passes the command an extra parameter, `Krtn`, with the record of keys that it wants back. The source doesn't contain this parameter.

## References

`MakeObjectAlias` builds an object reference from a container and its parts. The opcode's low bits give the form:

| Opcode | Form | Example |
| --- | --- | --- |
| `2c` | Property | `name of x` |
| `2d` | Every element | `every window`. `properties of x` is every `«class prop»` of `x`. |
| `2e` | Some element | `some window` |
| `2f` | By index | `window 1` |
| `30` | By key | `window id 5`, `window named "a"`, `window index 2` |
| `31` | Filter | `every window whose …` |
| `32` | Range | `items 1 thru 3` |
| `33`, `34` | Before, after | `before x`, `after x` |
| `35`, `36`, `37` | Beginning, end, middle | `beginning of x`, `end of x`, `middle item of x` |

Inside a `whose` filter, the object under test is a placeholder, a kind 19 vector, that stands for `it`.

`MakeComp` builds a comparison inside a filter. The opcodes `38` to `40` are the plain comparisons in order, from `Equal` to `Contains`, and `41`, `42` and `43` are `and`, `or` and `not`. No script in the corpus uses `44`.

When every part of a reference is a constant, the compiler evaluates the reference ahead of time and stores the result as a literal: a typed vector of kind 21 to 32, the form above plus 21. See [Typed vectors](file-format.md#typed-vectors).

`GetData` evaluates a reference. Where the compiler wouldn't evaluate a reference on its own, a `GetData` means an explicit `get` in the source: `f(get name of me)`, `item 1 of (get name of every process whose …)`, `repeat with x in (get every folder of …)`. A plain `f(name of me)` passes the reference without a `GetData`.

`POSIX file "/x"` with a literal path compiles with `me` as its container (`PushMe`), although the source has no `of me`.

## Shared values

The compiler evaluates a value once and keeps it on the stack with `Dup` and `GCSwap` when several references use it:

- In `x's text 1 thru text item -2`, the end of the range is an element of the range's own container. Both share one copy of `x`.
- `set {a, b} to x` compiles to one assignment for each item, `set a to item 1 of x` and `set b to item 2 of x`, which share one copy of `x`, and then discards `x`. In a nested pattern, `set {{a, b}, c} to x`, the inner value `item 1 of x` is discarded with `Pop`.

## Values that become statements

A statement's value goes to `StoreResult`, which saves it as `result`. The last statement of a handler has no `StoreResult`: its value stays on the stack for the implicit `Return` at the end.

## Terms without a dictionary

When an app's dictionary isn't installed, its terms appear as raw codes, and a raw code compiles differently depending on where it stands:

- `«class FCcn»` alone is a class, but the bytecode can read the property `FCcn` of the `tell` target. `its «class FCcn»` compiles back to the property read.
- `«class pTrk» of application "iTunes"` has only constant parts, so the compiler turns it into one constant reference. `«property pTrk» of application "iTunes"` compiles to a property read, which is what the original did when the dictionary was installed.

## Decompiling bytecode

scriptview decompiles bytecode by simulating the stack. Each instruction pops the expressions that it needs and pushes a new expression. When a statement ends, it joins the current block. The control flow instructions (`TestIf`, the repeat forms, `Tell`, `ErrorHandler`, `Consider`) mark regions, and each region becomes a nested block.

The result is the same kind of syntax tree that the compiler saves, with every flag set to zero. The same printer prints it, with a few rules of its own that keep the output compiling to the same bytecode. See [Output quirks](quirks.md).

## What bytecode doesn't keep

The compiler discards some things, and no decompiler can recover them:

- Comments.
- Line breaks and parentheses that you added for readability.
- Your choice among synonyms, such as `is equal to` instead of `=`. Every node comes out in the default spelling.
- Where a `using terms from` block ends. The compiler marks only where it starts, so scriptview closes it at the end of the enclosing block.
- Where `global` and `use` statements were. scriptview puts them at the top, unless Standard Additions commands show that a `use` came after them.
- Whether you wrote `on run`, or put the statements at the top level.
- The order of top-level statements relative to handlers and properties.

To check the result, compile the decompiled source again and compare the new bytecode with the original. See [Testing](testing.md).
