# The bytecode

Every compiled script has bytecode, including scripts that keep a syntax tree. The bytecode is what runs. A run-only script (`osacompile -x`) is a normal script with the syntax tree removed, so the bytecode is the only way to read it.

The opcode names we use come from Jinmo's [applescript-disassembler](https://github.com/Jinmo/applescript-disassembler). We checked the operand sizes ourselves on 7,598 code blocks from real scripts, and fixed a few along the way.

## Handlers

Each handler is a typed vector of kind 16 or 17. 16 is a plain procedure. 17 is a closure: a handler whose local variables a script object inside it still uses after the handler returns. Both decompile the same way:

```
{name, _, positional parameters, labeled parameters, variable names, literals, code}
```

- The name is an identifier, or an event code for handlers like `on run` and `on open`.
- Positional parameters are `{count, {names}}`. A direct parameter is a bare name: an event handler's (`on open x`), or a labeled handler's (`on mk of x by y`). `on run {a, b}` stores a list of names.
- Instructions refer to variables by their index in the variable names, and to constants by their index in the literals. The literals hold numbers, strings, terms, handler names, and nested code for script objects.
- The code is a raw `11` record.

The top level of a script is the implicit `run` handler.

## Instructions

The machine is stack-based. Most opcodes are one byte. Some take 16-bit operand words after the opcode.

The bytes `a0` to `ff` pack a small operand into the opcode:

| Bytes | Instruction | Operand |
| --- | --- | --- |
| `a0` to `af` | `PushVariable` | 4 bits |
| `b0` to `bf` | `PopVariable` | 4 bits |
| `c0` to `cf` | `PushGlobal` | 4 bits |
| `d0` to `df` | `PopGlobal` | 4 bits |
| `e0` to `ff` | `PushLiteral` | 5 bits |

Larger indexes use the `Extended` forms, which take a 16-bit word.

Branches (`Jump`, `TestIf`, `And`, `Or`, `LinkRepeat`, `ErrorHandler`) take a signed offset. The words of `Tell`, `Consider` and `BeginTransaction` are offsets too: they point at the matching end instruction. The offset counts from the byte right after the opcode. We first counted from the next instruction, and that cost us some time.

Some instructions we had wrong at first:

- `RepeatNTimes`, `RepeatWhile`, `RepeatUntil` and `BeginTimeout` have no operand words. `BeginTransaction` has one.
- `PopVariable`, `PopGlobal` and `SetData` store the top of the stack without removing it. The compiler follows them with a `Pop` when it doesn't need the value.
- `DefineProperty`, `DefineProcedure` and `DefineActor` each take a word.
- `HandleError` takes two words: literal indexes for the error variable and for the labeled parameter bindings.
- `MatchLiteral` takes one word, a literal index. It checks one slot of a list pattern that holds a literal instead of a variable, as in `set {a, 5} to x`.

## Commands

A command call pushes its arguments, then runs `MessageSend` with the event's literal index:

```
direct parameter (or PushIt), then for each labeled parameter: key, value, then the count
```

The count is the number of keys and values together, so it's twice the number of labeled parameters. The direct parameter isn't counted.

`set {year:y, month:m} to current date` passes the command an extra `Krtn` parameter with the record of keys it wants back. The source doesn't write it, so we leave it out. `properties of x` is `every «class prop»`, kind 1 below, and prints as `properties`.

A user handler call with positional arguments, like `f(1, 2)`, uses `PositionalMessageSend` with the handler name as a literal. The stack holds the target (`PushIt` for a plain call, `PushMe` for `my f()`), the arguments, then the count. A user handler with labeled parameters, like `f from 1 to 2` or `f given x:1`, uses `MessageSend` with the handler's name instead of an event. `Continue` and `PositionalContinue` do the same for `continue foo()`, which passes the call on to the parent script. Their stack is laid out differently: the target and the labeled parameters come first, then `PushNext`, then the direct parameter (or `PushIt` when there is none).

## References

`MakeObjectAlias` builds an object reference. The low bits of the opcode say which kind:

| Kind | Reference |
| --- | --- |
| 0 | property: `name of x` |
| 1 | `every x` |
| 2 | `some x` |
| 3 | by index: `window 1` |
| 4 | by key: `window id 5`, `window named "a"`, `window index 2` |
| 5 | filter: `every window whose ...` |
| 6 | range: `items 1 thru 3` |
| 7, 8 | `before x`, `after x` |
| 9, 10, 11 | `beginning of x`, `end of x`, `middle x`. Only `middle` takes a class. |

Inside a `whose` filter, the thing being tested is a placeholder object (a kind 19 vector) that stands for `it`.

`MakeComp` builds the comparisons in a filter. Kinds 0 to 8 match the order of the plain comparison opcodes (`Equal` to `Contains`), and 9, 10 and 11 are `and`, `or` and `not`. `is in` is `Contains` with the two sides swapped. There's a slot for a kind 12, but we never saw the compiler use it.

## Terms without a dictionary

When an app's dictionary isn't installed, its terms print as raw codes, and a raw code compiles differently depending on where it stands. `«class FCcn»` alone is a class, but the bytecode may read property `FCcn` of the tell target. Run-only output writes `its «class FCcn»` there, which compiles back to the property. With a constant container the compiler goes further: `«class pTrk» of application "iTunes"` becomes one constant reference, not a property read. Run-only output writes `«property pTrk» of application "iTunes"` when the bytecode reads the property, which AppleScript also accepts. `osadecompile` prints a bare `«class FCcn»` either way, so output from the syntax tree does too.

## Shared containers

The compiler evaluates a container once and keeps it on the stack with `Dup` and `GCSwap` when several references use it. In `x's text 1 thru text item -2`, the end of the range is an element of the range's own container, so the output leaves the container out of it: `text 1 thru text item -2 of x`. Writing it out, `text 1 thru (text item -2 of x) of x`, would evaluate `x` twice.

A top-level variable that held a script object when the file was saved, such as `set lib to load script …`, keeps that script in the script's table. It isn't a `script` definition, so the output leaves it out.

## Values that become statements

A statement's value goes to `StoreResult`, which saves it as `result`. The last statement of a handler has no `StoreResult`. Its value stays on the stack for the implicit `Return` at the end. When we rebuild a block, any value left on the stack at the end is the block's last statement.

`GetData` means "evaluate this reference". When the value is a reference and it becomes a statement on its own, the source had an explicit `get`, such as `get window 1`.

## How we decompile it

We simulate the stack. Each instruction pops the expressions it needs and pushes a new expression node. When a statement ends, we add it to the current block. Control flow opcodes (`TestIf`, the `Repeat` forms, `Tell`, `ErrorHandler`, `Consider`) mark regions, and we decompile each region as a nested block.

We build the same kind of syntax tree that the compiler would have saved, with all flags set to zero, and print it with the same code that prints normal scripts.

## What you can't get back

The compiler throws some things away:

- Comments.
- Line breaks and parentheses you added for readability.
- Your choice of words where AppleScript has synonyms, such as `is equal to` compared with `=`. With no flags, everything comes out in the default spelling.
- Where a `using terms from` block ends. The compiler only marks where it starts, so we close it at the end of the enclosing block.
- Where `global` and `use` statements were. We put them at the top, unless Standard Additions commands show that a `use` must have come after them.
- Whether you wrote `on run` or put the statements at the top level.
- The order of statements at the top level, compared with handlers and properties.

To check the result, we compile the decompiled source again and compare the new bytecode with the original (see [How we tested](testing.md)). On our GitHub corpus, about 80% of the older set and 90% of the newer set come back identical. Most of the rest use apps that aren't installed on the test Mac: their terms print as raw codes, and raw codes compile differently than terms from a loaded dictionary.
