# The file format

A compiled script starts with the 16 bytes `FasdUAS 1.101.10`. That reads like one string, but it's four 4-byte fields: `Fasd`, `UAS `, and two versions, `1.10` and `1.10`. `UAS` is the prefix Apple's runtime uses for its own names. Nobody has written down what `Fasd` means. The AppleScript team came from Lisp, where compiled files are "fast load" files written by a "fast dump", so our guess is "fast dump".

The file ends with a trailer that starts with `ascr` and ends with the bytes `FA DE DE AD`. This is the Open Scripting Architecture's storage trailer: `ascr` names the scripting component that made the data, then come a 16-bit version, a 16-bit length and `FADEDEAD`. The length can count a pad byte before `ascr`. Everything between the magic and the trailer is the script.

Scripts copied through non-Mac systems sometimes have junk bytes after `FADEDEAD`. Deleting everything after it repairs them.

Some `.scpt` files hold other things:

- A JavaScript for Automation script starts with `JsOsaDAS`. After that comes a binary property list with the source text in it.
- Some files are only plain text with a `.scpt` name. `osadecompile` compiles those first, so its output can look different from the file.
- A script bundle (`.scptd`) or an applet (`.app`) keeps the compiled script at `Contents/Resources/Scripts/main.scpt`.

## Records

The body is a list of records. Each record starts with the same five bytes:

```
type (1 byte)  reference (2 bytes)  size (2 bytes)  payload
```

All numbers are big-endian. The records together make a graph of objects, and the reference number is how one object points at another:

- A negative reference names an object that is stored inline. It is the next record in the stream the first time it's used.
- A zero or positive reference names a shared object. Many objects can point at it.

The loader reads objects depth first, in the order they are first used. So the stream is every object, in that order, with nothing between them.

## Record types

| Type | What it is |
| --- | --- |
| `01` | A symbol. If the size isn't zero, an 8-byte value follows. |
| `02` | A list cell: `[head, tail]`. Lists are chains of these. |
| `03` | A small integer. The value is the size field itself. |
| `04`, `0e` | A typed vector: a kind byte, then references. |
| `06` | A record or labeled-parameter cell: `[key, value, next]`. |
| `07` | A 32-bit integer. |
| `08` | A double. |
| `09` | A boolean. The value is the size field. |
| `0a` | A term: a kind byte, then a four-character code. |
| `0b` | An identifier: a kind byte, then two names. |
| `0c` | A Mac Roman string: a length, then the bytes. |
| `0d` | A syntax tree node: a kind byte, flags, two code offsets, then references. |
| `0f` | A descriptor: a kind byte, then raw bytes. |
| `10` | A vector of references with no kind byte. |
| `11` | Raw bytes. UTF-16 text and bytecode use this. |
| `12`, `13` | Like `0f` and `11`, with a 32-bit length for large data. |

A few of these need more detail.

### Terms (`0a`)

The kind byte says what the code is:

- `0a`: a class, property or constant, with one four-character code such as `cwin` for `window`.
- `0b`: a typed constant, with two codes: the enumeration and the value.
- `2e`: an event (a command), such as `sysodlog` for `display dialog`. The record is 24 bytes: the suite code, the event code, then the reply type, two flag words and the direct parameter type. Only the first eight bytes matter for printing.
- `2f`: a second kind of class code, also four bytes.

Codes aren't always printable. We found codes with a NUL byte in them.

### Identifiers (`0b`)

The kind byte is `30`. Then come two names, each with a 16-bit length: the name in lowercase, then the name as you wrote it. AppleScript identifiers ignore case, so the lowercase copy is what the machine compares.

If the second name is empty, the identifier was written with pipes, like `|my var|`.

The names are Mac Roman in old scripts and UTF-8 in newer ones. We try UTF-8 first.

### Strings

Newer scripts keep string literals as UTF-16 in a raw `11` record. The record is wrapped in a `0e` vector of kind `b1`. Older scripts use a Mac Roman `0c` record. Comments keep both: a Mac Roman copy and a UTF-16 copy.

### Descriptors (`0f`)

A descriptor is an Apple Event value. Kind `0d` holds a four-character type, then the data. We see these types:

- `ldt `: a date, as seconds since 1904.
- `alis`: an alias record. See [Applications](applications.md).
- Anything else prints as `«data TYPE0123ABCD»`.

Application specifiers are descriptors too. They hold either an alias record or an `aprl` URL.

## The root

Object 0 is the root of the script. It is a typed vector of kind 15 with four items:

```
{nil, syntax tree, parent, table}
```

- The second item is the top block of the syntax tree. A run-only script has nil there, and that is the whole difference: the bytecode is still in the file.
- The third item is an empty kind 15 vector. The loader swaps in a shared empty script object. We think it's the parent script, which isn't saved with the file.
- The table is a list: `[_, [handler names], handler, handler, ...]`. Each handler is a kind 16 or 17 vector. See [The bytecode](bytecode.md) for what's inside.

A script object inside the script looks the same as the root: a kind 15 vector with a name and its own table.
