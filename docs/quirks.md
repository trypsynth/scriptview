# Output quirks

`osadecompile` prints from the syntax tree, but it adds some things on its own. Matching its output meant finding these rules one by one. Most of them are about parentheses.

## Parentheses

Script Editor adds parentheses in these places, even when the tree has no parentheses node:

- A command used inside an expression: `hours of (current date)`.
- `x of y` inside arithmetic: `(x of self) + (x of other)`.
- A handler call on the right side of `*`, `/`, `^`, `div` or `mod`: `2 * (f(x))`. On the left side, or next to `+` and `-`, there are none.
- A value passed to an Objective-C style call (`x's foo:y`), when the value uses `+`, `-`, `&`, a comparison, `and` or `or`: `x's foo:(a - b)`. With `*`, `/`, `^` or a minus sign there are none: `x's foo:a * 2`. References, coercions and `not` always get them.
- A constant passed to an Objective-C style call: `x's foo:(missing value)`.
- `my (x's foo:1)` gets parentheses around the call, and nothing inside them.

### The `repeat with` rule

This one surprised us. Inside a `repeat with` loop, Script Editor puts parentheses around an Objective-C style call or a `whose` filter when it is a whole statement, the value of a `set`, or the value of a `return`:

```applescript
repeat with anItem in theList
	set x to (anItem's objectForKey:"a")
end repeat
set y to anItem's objectForKey:"b"
```

The same goes for `set h to (items of L whose it > 1)`. Other expressions, such as `item 1 of L` or `x's y`, don't get them.

It happens at any depth inside the loop, even inside a nested `repeat 3 times`. It doesn't happen in `repeat n times`, `repeat while` or `repeat until` loops on their own. Our guess is that the printer sets a flag when it prints the `repeat with` line and doesn't clear it until the loop ends. We copy the behavior.

## Objective-C style calls

A handler named `foo_bar_` can print as `foo:x bar:y` or as `foo_bar_(x, y)`. Script Editor uses the first form only when the call has a target, such as `x's foo:1` or `my foo:1`, and in the handler's own `on foo:bar:` line. A plain call prints as `foo_bar_(x, y)`.

## Raw codes

A raw code prints its four bytes as Mac Roman characters, even when they aren't printable. We saw `«constant pSAT Ã »` where the code bytes are `00 CC 00 0E`: the NUL byte and the control character go into the output as they are.

## Line breaks

When a one-line `if` or `tell` has a `¬` break before a block statement, the whole block moves one level in:

```applescript
tell application "System Events" to if (count of processes) > 0 then ¬
	tell application "Finder"
		set x to name of startup disk
	end tell
```

## Words

- `given` labels with no dictionary term print with a class name, or with an AppleScript property name such as `time`.
- An explicit `index` stays: `window index 2`. Without it, `window 2`.
- New compilers rewrite `it's` to `its` and add a `-- Grammar Police` comment. Old scripts keep `it's`, and it prints as written.
