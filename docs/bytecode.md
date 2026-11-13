# Bytecode

Every compiled script has bytecode, including scripts that keep a syntax tree. The bytecode is what runs. A run-only script is a normal script without its syntax tree, so the bytecode is the only way to read it. `osacompile -x` and Script Editor's run-only option both produce one. Inside Macintosh describes run-only data as "only the minimum script data required to run the script."

The opcode names come from Jinmo's [applescript-disassembler](https://github.com/Jinmo/applescript-disassembler), which ports AppleScript's own opcode table. The operand sizes on this page were checked against the code blocks of more than 12,000 real scripts.

## Handlers

Each handler is a typed vector of kind 16, or kind 17 for a closure, with seven items: the name, an unused nil, the positional parameters, the labeled parameters, the variable names, the literals, and the code.

- Name: an identifier, or an event code for handlers such as `on run` and `on open`.
- Positional parameters: a kind 4 vector, `{count, {names}}`. A bare name instead is a direct parameter, of an event handler (`on open x`) or a labeled handler (`on mk of x by y`). A kind 123 vector is a direct parameter with a type or a default, as in `on run argv as text`. A list of names is a pattern, as in `on run {a, b}`. AppleScript 1.0 lists positional parameters as plain names.
- Labeled parameters: a binding of labels to the names they bind.
- Variable names: the handler's parameters, then its local variables. Instructions refer to a variable by its index in this list. A variable that is also a term, such as `password`, has an empty name here and its term in the same slot.
- Literals: numbers, strings, terms, handler names, and nested handlers and script objects. Instructions refer to a constant by its index in this list.
- Code: a raw `11` record.

A closure is a handler whose local variables a script object inside it still uses after the handler returns. Both kinds decompile the same way.

### The run handler

The top level of a script is its implicit `run` handler. The compiler puts the implicit `run` handler after every other handler in the table, so a `run` handler anywhere else was an explicit `on run`.

The last handler can be explicit too. Two things show it:

- The code ends with `StoreResult`, `GetResult`, `Return` instead of a bare `Return`. Only a blank line or a comment after the last statement of an explicit `on run` produces this ending. At the top level, trailing blank lines and comments leave no trace.
- The handler is empty: `PushUndefined`, then `Return`. A script with no top-level statements has no `run` handler at all, so an empty one was written as `on run` and `end run`.

### Variable numbers and local declarations

Run-only scripts don't store `local` declarations, but the order of a handler's variable names reveals them. The compiler numbers a handler's variables in the order that its generated code first uses them, after the parameters. The order is the order of evaluation, not of the source text. In `set x to y`, `y` gets the lower number, because the code reads `y` before it stores `x`.

A `local` declaration numbers its variables at the point of the declaration. So when the variable names aren't in first-use order, or a variable is never used at all, the handler declared some of them. scriptview rebuilds the shortest declaration that gives the same order: `local a, b` for the first variables up to the last one out of place.

Some variables must have been declared even when the order fits:

- In the top-level `run` handler, a variable is a global unless it's declared. So every variable in that handler's list was declared `local`, except loop variables (`repeat with i`), `on error` variables, and the names of `script` blocks defined there.
- In other handlers, a variable named like a global, or like a property declared earlier in the script, would be that global or property. A local of the same name must have been declared. Loop and `on error` variables are local anyway. A property declared later in the source doesn't count: a handler sees only the properties above it.

## Instruction encoding

The machine is stack based. Each instruction is one opcode byte, followed by zero, one, or two 16-bit operand words.

The bytes `a0` to `ff` pack a small operand into the opcode itself:

- `a0` to `af`: `PushVariable`, with a variable index in the low 4 bits.
- `b0` to `bf`: `PopVariable`, with a variable index in the low 4 bits.
- `c0` to `cf`: `PushGlobal`, with the index of the literal that names the global in the low 4 bits.
- `d0` to `df`: `PopGlobal`, with the same kind of index.
- `e0` to `ff`: `PushLiteral`, with a literal index in the low 5 bits.

Larger indexes use the `Extended` forms, which take a 16-bit word.

A branch operand is a signed byte offset that counts from the byte right after the opcode, not from the next instruction. `Jump`, `TestIf`, `And`, `Or`, `LinkRepeat`, `ErrorHandler`, and `EndErrorHandler` take one. The words of `Tell`, `Consider`, and `BeginTransaction` are offsets too: they point at the matching end instruction.

`PopVariable`, `PopGlobal`, and `SetData` store the top of the stack without removing it. The compiler follows them with `Pop` when it doesn't need the value.

## Opcodes

Each item gives the opcode byte, the instruction's name, what it does, and how many scripts in the GitHub corpus use it, out of 12,150. An instruction takes no operand words unless the item says so.

### Comparisons and logic

- `00` `Equal`: `=`. 6,089 scripts.
- `01` `NotEqual`: `≠`. 2,434 scripts.
- `02` `GreaterThan`: `>`. 1,741 scripts.
- `03` `GreaterThanOrEqual`: `≥`. 512 scripts.
- `04` `LessThan`: `<`. 1,173 scripts.
- `05` `LessThanOrEqual`: `≤`. 372 scripts.
- `06` `StartsWith`: `starts with`. 608 scripts.
- `07` `EndsWith`: `ends with`. 396 scripts.
- `08` `Contains`: `contains`. `is in` swaps the operands. 2,734 scripts.
- `09` `And`: `and`. One word, a branch that skips the right side. 1,140 scripts.
- `0a` `Or`: `or`. One word, a branch that skips the right side. 977 scripts.
- `0b` `Not`: `not`. 2,560 scripts.

### Arithmetic and text

- `1e` `Add`: `+`. 2,070 scripts.
- `1f` `Subtract`: `-`. 1,447 scripts.
- `20` `Multiply`: `*`. 930 scripts.
- `21` `Divide`: `/`. 580 scripts.
- `22` `Quotient`: `div`. 271 scripts.
- `23` `Remainder`: `mod`. 280 scripts.
- `24` `Power`: `^`. 64 scripts.
- `25` `Concatenate`: `&`. 6,660 scripts.
- `26` `Coerce`: `as`. 5,519 scripts.
- `27` `Negate`: unary minus. 170 scripts.

### Constants

- `65` `PushTrue`: `true`. 4,771 scripts.
- `66` `PushFalse`: `false`. 3,939 scripts.
- `67` `PushEmpty`: an empty list. 242 scripts.
- `68` `PushUndefined`: no value, as an empty result slot. 8,081 scripts.
- `69` `PushMinus1`: -1. 1,182 scripts.
- `6a` to `6d` `Push0` to `Push3`: 0 to 3. `Push0` is in 10,853 scripts, `Push1` in 9,365, `Push2` in 7,893, and `Push3` in 3,285.
- `29` `PushMe`: `me`. 4,422 scripts.
- `2a` `PushIt`: `it`, which is the `tell` target or the default target. 10,115 scripts.

### Variables and the stack

- `5d` `PushVariableExtended` and `5e` `PopVariableExtended`: the variable forms with a 16-bit index word. 333 and 306 scripts.
- `5f` `PushGlobalExtended` and `60` `PopGlobalExtended`: the global forms with a 16-bit index word. 4,364 and 4,019 scripts.
- `61` `PushLiteralExtended`: `PushLiteral` with a 16-bit index word. 7,369 scripts.
- `62` `PushParentVariable`: a variable of an enclosing script object. Two words: the scope level and the index. 2,282 scripts.
- `63` `PopParentVariable`: stores into such a variable. 457 scripts.
- `5a` `Pop`: drops the top of the stack. 4,206 scripts.
- `5b` `Dup`: duplicates the top of the stack. 6,752 scripts.
- `5c` `GCSwap`: swaps the top two values. 3,567 scripts.
- `4f` `StoreResult`: stores the top of the stack as `result`, after a statement. 10,882 scripts.
- `50` `GetResult`: pushes `result`. 3,027 scripts.

### Lists, records, and references

- `76` `MakeVector`: builds a `{…}` list from a count and items. 5,357 scripts.
- `0d` `MakeList`: builds a `[…]` linked list. AppleScript 1.0 builds every list this way. 130 scripts.
- `0e` `MakeRecord`: builds a record from a count and key–value pairs. 1,564 scripts.
- `2c` to `37` `MakeObjectAlias`: builds an object reference. The low bits give the form. See [References](#references). 10,429 scripts.
- `38` to `44` `MakeComp`: builds a comparison inside a `whose` filter. See [References](#references). 2,444 scripts.
- `11` `ObjectAliasQuote`: `a reference to`. 100 scripts.
- `28` `GetData`: evaluates a reference. 10,056 scripts. Byte `45` has the same name, and no script uses it.
- `46` `SetData`: `set` a reference to a value. 4,225 scripts.
- `47` `CopyData`: `copy` a value to a reference. 255 scripts.
- `51` `Clone`: copies a value, for `copy`. 172 scripts.
- `75` `MatchLiteral`: checks one slot of a pattern that holds a literal, as in `set {a, 5} to x`. One word, a literal index. 4 scripts.

### Commands and calls

- `0c` `MessageSend`: sends a command or a labeled handler call. One word, the literal index of the event or handler name. 11,103 scripts.
- `2b` `PositionalMessageSend`: calls a handler with positional arguments, `f(x, y)`. One word, the literal index of the name. 5,350 scripts.
- `10` `Continue`: `continue`, which passes a labeled call to the parent script. One word. 40 scripts.
- `4a` `PositionalContinue`: `continue f(x)`. One word. No script uses it.
- `64` `PushNext`: marks where `continue` passes its arguments. 40 scripts.
- `0f` `Return`: returns the top of the stack. 11,866 scripts.
- `15` `Error`: `error`. 2,031 scripts.

### Control flow

- `1d` `TestIf`: branches when the top of the stack is false. One word. 7,666 scripts.
- `59` `Jump`: branches. One word. 8,418 scripts.
- `12` `Tell` and `55` `EndTell`: a `tell` block. The word points at the `EndTell`. 9,342 scripts.
- `13` `Consider` and `56` `EndConsider`: a `considering` or `ignoring` block. 250 scripts.
- `14` `ErrorHandler`: starts a `try` block. The word points at the `on error` code. 5,008 scripts.
- `57` `EndErrorHandler`: ends the body of a `try` block. The word points past the `on error` code.
- `58` `HandleError`: starts the `on error` code. Two words: the literal indexes of the error variable's name and of a binding of labels, such as `number`, to variable names.
- `17` `LinkRepeat`: starts a loop. The word points past its end. 4,896 scripts.
- `18` `RepeatNTimes`: `repeat n times`. 600 scripts.
- `19` `RepeatWhile`: `repeat while`. 527 scripts.
- `1a` `RepeatUntil`: `repeat until`. 421 scripts.
- `1b` `RepeatInCollection`: `repeat with x in list`. The word is the loop variable. 3,063 scripts.
- `1c` `RepeatInRange`: `repeat with x from a to b`. The word is the loop variable. 1,634 scripts.
- `16` `Exit`: `exit repeat`. 1,133 scripts.
- `6e` `BeginTimeout` and `6f` `EndTimeout`: a `with timeout` block. 202 scripts.
- `70` `BeginTransaction` and `71` `EndTransaction`: a `with transaction` block. 8 scripts.

### Definitions

- `4b` `DefineActor` and `53` `EndDefineActor`: a `script` object defined inside a handler. One word. 264 scripts.
- `4c` `DefineProcedure`: defines a handler, inside a script object's init code. One word.
- `4d` `DefineClosure`: defines a closure.
- `4e` `DefineProperty`: defines a property. One word: the index of the literal that names the property, which is almost always `parent`. It isn't a variable index.
- `52` `Of` and `54` `EndOf`: unknown. No script uses them.

The bytes `48`, `49`, `72` to `74`, and `77` to `9f` aren't instructions.

## Commands

A command pushes its direct parameter, or `PushIt` when it has none, then a key and a value for each labeled parameter, then a count. Then it runs `MessageSend` with the literal index of the event.

The count is the number of keys and values together, which is twice the number of labeled parameters. It doesn't include the direct parameter.

A call to a handler with positional arguments, such as `f(1, 2)`, runs `PositionalMessageSend` with the handler's name as the literal. The stack holds the target (`PushIt` for a plain call, `PushMe` for `my f()`), the arguments, then the count. A call to a handler with labeled parameters, such as `f from 1 to 2` or `f given x:1`, uses `MessageSend` with the handler's name in place of an event.

`Continue` and `PositionalContinue` do the same for `continue f()`, which passes the call on to the parent script. Their stack is laid out differently: the target and the labeled parameters come first, then `PushNext`, then the direct parameter or `PushIt`.

`set {year:y, month:m} to current date` passes the command an extra parameter, `Krtn`, with the record of keys that it wants back. The source doesn't contain this parameter.

## References

`MakeObjectAlias` builds an object reference from a container and its parts. The opcode's low bits give the form:

- `2c`: a property, as in `name of x`.
- `2d`: every element, as in `every window`. `properties of x` is every `«class prop»` of `x`.
- `2e`: some element, as in `some window`.
- `2f`: an element by index, as in `window 1`.
- `30`: an element by key, as in `window id 5`, `window named "a"`, or `window index 2`.
- `31`: a filter, as in `every window whose …`.
- `32`: a range, as in `items 1 thru 3`.
- `33` and `34`: before and after, as in `before x`.
- `35`, `36`, and `37`: beginning, end, and middle, as in `end of x` or `middle item of x`.

Inside a `whose` filter, the object under test is a placeholder, a kind 19 vector, that stands for `it`.

`MakeComp` builds a comparison inside a filter. The opcodes `38` to `40` are the plain comparisons in order, from `Equal` to `Contains`, and `41`, `42`, and `43` are `and`, `or`, and `not`. No script in the corpus uses `44`.

When every part of a reference is a constant, the compiler evaluates the reference ahead of time and stores the result as a literal: a typed vector of kind 21 to 32, the form above plus 21. See [Typed vectors](file-format.md#typed-vectors).

`GetData` evaluates a reference. Where the compiler wouldn't evaluate a reference on its own, a `GetData` means an explicit `get` in the source: `f(get name of me)`, `item 1 of (get name of every process whose …)`, `repeat with x in (get every folder of …)`. A plain `f(name of me)` passes the reference without a `GetData`.

### Applications

`application "Finder"` with a literal name is a constant: the compiler stores an application specifier as a literal. An application built at run time is an element reference instead, `MakeObjectAlias` by index on class `capp`. When its name is a literal string, the source must have kept the compiler from folding it, as in `application (get "Finder")`. An application of a remote machine, `application "Finder" of machine "eppc://…"`, is always built at run time.

An application given by URL, such as a SOAP endpoint, is a literal descriptor that holds an `aprl` URL instead of an alias.

### POSIX file and me

`POSIX file "/x"` with a literal path compiles with `me` as its container (`PushMe`), although the source has no `of me`. With a path that isn't a literal, `POSIX file x` takes the default container: `me` at the top level, and the `tell` target inside a `tell` block. So inside a `tell` block, a `me` container on a non-literal path means the source said `my POSIX file x`.

## Script objects

A `script` block at the top level of a script is a kind 15 vector in the property table, with its own table inside. A `script` block inside a handler is built at run time: `DefineActor` takes an init handler, whose code defines the object's properties and handlers one by one.

In the init handler:

- An assignment to one of the handler's own variables defines a property.
- `DefineProperty` defines `property parent : …`. Its operand is the literal that names the property, not a variable.
- `DefineProcedure` defines a handler.
- `use framework "AppKit"` sets the property `required import items` (`pimr`) to a list of records, `{{item:a reference to framework "AppKit"}}`. A second `use` appends to it: `set required import items to required import items & {…}`.

A declared parent in a top-level script leaves an entry named `pare` with no value in the property table, and the parent itself in item 2 of the script object. An empty kind 15 vector there stands for AppleScript.

A property can hold a script object or a handler that is also defined by name, as in `property suite : ASUnitSentinel` or `property sort : mergeSort`. Both entries then point at the same vector.

## Shared values

The compiler evaluates a value once and keeps it on the stack with `Dup` and `GCSwap` when several references use it:

- In `x's text 1 thru text item -2`, the end of the range is an element of the range's own container. Both share one copy of `x`.
- `{name, artist} of current track` builds the list from one copy of the container. With brackets, `[name, artist] of current track`, the list is built with `MakeList`.
- `set {a, b} to x` compiles to one assignment for each item, `set a to item 1 of x` and `set b to item 2 of x`, which share one copy of `x`, and then discards `x`. In a nested pattern, `set {{a, b}, c} to x`, the inner value `item 1 of x` is discarded with `Pop`. A record pattern, `set {text:t} to x`, does the same with properties. Only the discarded copy of `x` tells a one-item pattern, `set {a} to x`, from `set a to item 1 of x`.

## Values that become statements

A statement's value goes to `StoreResult`, which saves it as `result`. The last statement of a handler has no `StoreResult`: its value stays on the stack for the implicit `Return` at the end.

AppleScript 0.98 compiles a `copy` whose value is also the result as `Clone`, `Dup`, then the store. The `Dup` leaves a second copy of the value for `StoreResult`.

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
- Where a `local` declaration was, and any part of it that the variable order doesn't reveal. scriptview puts the declaration at the start of the handler. See [Variable numbers and local declarations](#variable-numbers-and-local-declarations).
- Whether you wrote `on run`, except in the cases in [The run handler](#the-run-handler).
- The order of top-level statements relative to handlers and properties.

To check the result, compile the decompiled source again and compare the new bytecode with the original. See [Testing](testing.md).
