# Output quirks

`osadecompile` prints from the syntax tree, but its printer adds things of its own. scriptview copies these rules so that its output matches. Most of them are about parentheses.

## Parentheses

### Parentheses that Script Editor adds

Script Editor adds parentheses in these places, even when the tree has no parentheses wrapper:

- A command inside an expression: `hours of (current date)`.
- `x of y` inside arithmetic: `(x of self) + (x of other)`.
- A handler call on the right side of `*`, `/`, `^`, `div` or `mod`: `2 * (f(x))`. On the left side, or next to `+` and `-`, there are none.
- `my x` and `my f(x)` next to an arithmetic operator: `1 + (my i)`, `2 * (my f(1))`. Comparisons, `&`, `is in`, and command parameters leave them bare: `x is not in my list`, `my f(1) is in L`, `offset of my decPt() in x`.
- A value passed to an Objective-C style call (`x's foo:y`) that uses `+`, `-`, `&`, a comparison, `and` or `or`: `x's foo:(a - b)`. With `*`, `/`, `^` or a minus sign, there are none: `x's foo:a * 2`. References, coercions and `not` always get them.
- A constant or a command passed to an Objective-C style call: `x's foo:(missing value)`, `x's addObject:(do shell script "ls")`. A raw `«constant …»` doesn't get them.
- `my (x's foo:1)` gets parentheses around the call, and none inside.
- A `tell` target that is a `whose` filter, a command, or an Objective-C style call: `tell (first process whose frontmost is true)`, `tell (make new document)`, `tell (NSString's stringWithString:x)`.
- A `whose` filter after `a reference to` or `repeat with x in`: `repeat with t in (every tab whose title is "a")`.
- A command after `not`: `not (exists file "x")`. A call to one of the script's own handlers doesn't get them: `not f()`.
- A reference as an element's index: `column (item i of L) of w`. Without them, the `of` applies to the whole element.

scriptview also keeps parentheses around a coercion that is the direct parameter of a command with an `as` parameter: `do shell script ("x" as string)`. `osadecompile` leaves these out when an old compiler stored the coercion, but without them the text compiles to the `as` parameter instead.

### The `repeat with` rule

Inside a `repeat with` loop, Script Editor puts parentheses around an Objective-C style call or a `whose` filter that is a whole statement, the value of a `set`, or the value of a `return`:

```applescript
repeat with anItem in theList
	set x to (anItem's objectForKey:"a")
end repeat
set y to anItem's objectForKey:"b"
```

The same applies to `set h to (items of L whose it > 1)`. Other expressions, such as `item 1 of L` or `x's y`, don't get them.

The rule applies at any depth inside the loop, even inside a nested `repeat 3 times`. It doesn't apply in `repeat n times`, `repeat while` or `repeat until` loops on their own. The printer probably sets a flag when it prints the `repeat with` line and clears it only when the loop ends. scriptview copies the behavior.

### Parentheses in run-only output

Run-only output has no source parentheses to keep, so scriptview adds the ones that the code needs to compile to the same bytecode:

- A comparison as another comparison's left operand: `(x = "") = false`.
- A handler call with labeled parameters, as a container or a `tell` target: `item 2 of (mk of s by "=")`, `tell (f at 5)`.
- A reference or a call after a minus sign: `-(length of s)`, `-(f(x))`.
- A reference as one end of a range: `text (length of s) thru -1 of s`. An element as an end gets them too: `text (text item 2) thru (text item -1) of p` compiles to different code than the same text without them.
- A reference after `id` or `named`: `«class OOrw» id (item i of L)`.

In a range, `my` follows the same rule as in Script Editor's output: a property stays bare and a call doesn't, as in `text my i thru -1` and `text (my f(1)) thru -1`.

Run-only output also keeps the order of parameters whose labels are raw codes. After a `given` label, it writes a boolean as `«class repl»:true`, not `with «class repl»`, because `with` moves it ahead of the others.

## Objective-C style calls

A handler named `foo_bar_` can print as `foo:x bar:y` or as `foo_bar_(x, y)`. Script Editor uses the first form only when the call has a target, such as `x's foo:1` or `my foo:1`, and in the handler's own `on foo:bar:` line. A plain call prints as `foo_bar_(x, y)`.

## Raw codes

A raw code prints its four bytes as Mac Roman characters, even when they aren't printable. For example, `«constant pSAT Ã »` has the code bytes `00 CC 00 0E`: the NUL byte and the control character go into the output unchanged.

## Line breaks

When a one-line `if` or `tell` has a `¬` break before a block statement, the whole block moves one level in:

```applescript
tell application "System Events" to if (count of processes) > 0 then ¬
	tell application "Finder"
		set x to name of startup disk
	end tell
```

## Words and values

- A `given` label with no dictionary term prints with a class name, or with an AppleScript property name such as `time`.
- An explicit `index` stays: `window index 2`. Without it, `window 2`.
- Inside `|pipes|`, a backslash, a bar and a tab are escaped: `|\\n|`, `|\||`, `|a\tb|`. `app` is reserved, as short for `application`.
- Reals print with at most 12 decimal places and no trailing zeros: `3.141592653589793` prints as `3.14159265359`, and `9.99999999999999` as `10.0`. Exponent notation keeps every digit: `1.23456789012345E+15`. Run-only output keeps every digit everywhere, so that it compiles back to the same value.
- A range can store its end as an element when the source repeats the class: `text 1 thru -5` can come back as `text 1 thru (text -5)`. The class isn't printed twice.
- `front` and `back` are other spellings of `beginning` and `end`, as in `make new folder at front`. A flag on the node keeps the spelling.
- Current compilers rewrite `it's` to `its` and add the comment `-- Grammar Police`. Old scripts keep `it's`, and it prints as written.
