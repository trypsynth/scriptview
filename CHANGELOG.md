# Changelog

## 0.3.1

- Run-only output compiles back to the same bytecode for 11,887 of 12,150 scripts from GitHub, up from 11,632
- Rebuilt `local` declarations, which run-only scripts don't store, from the order of each handler's variables
- Run-only output keeps `use` statements and `property parent` inside script objects, and refers by name to a script object or handler that another property holds, instead of writing it twice
- Objective-C calls in run-only scripts with no `use` statement print as `foo_bar_(x, y)`, since `foo:x bar:y` needs one
- Fixed more run-only parentheses: an explicit `get` or a labeled call passed as a parameter, a labeled call used as an index or operand, `(a reference to x)` as a container, and `do shell script ("x" & password)`
- Run-only output keeps `property parent`, an explicit `on run` handler that is empty or ends in a blank line, `on run argv as text`, `application (get "X")`, applications given by URL, and `my POSIX file x` inside a `tell` block
- Fixed run-only output of `set {a} to x` and `set {text:t} to x`, and of `[a, b] of x`
- Fixed parentheses around an Objective-C call or a command used as a container, as in `item 2 of (x's foo:y)` and `(make x)'s f()`, and around `(x ^ 2) ^ 0.5`

## 0.3.0

- Old Japanese scripts decode as Shift JIS instead of printing garbled text
- Scripts from classic Mac OS get the terms of about 80 classic scripting additions and apps, such as the Finder, HyperCard, and the Scriptable Text Editor, which cuts their raw codes from 257 to 23
- AppleScript 1.0 scripts use Apple's own symbol table, read from the AppleScript 1.0 extension, which fixes 5 symbols that were guessed wrong, including `true` and `false`
- Added scripts from the AppleScript 1.0 beta (`FasdUAS 0.95`), which use other node letters, layouts, and symbols. The 16 beta scripts that came with plain-text copies decompile to the same code, apart from wording such as `actor` for `script`
- Run-only AppleScript 1.0 scripts decompile, including handler parameters, list literals, and script objects
- Run-only output compiles back to the same bytecode for 11,632 of 12,150 scripts from GitHub. Fixed `get` on `repeat with` loops, handler arguments, and `whose` containers, `copy`, properties in their original order, labeled handler parameters, nested destructuring such as `set {{a, b}, c} to x`, `its «class x»`, `«property x» of application "A"`, `id` values, and the ends of element ranges
- Fixed `-- copy (incomplete)` lines in run-only scripts from early compilers, which keep a `copy` statement's value as the script's result
- Output matches `osadecompile` on 12,105 of 12,143 scripts from GitHub and on all 163 scripts that ship with macOS 26
- Fixed parentheses around `not (exists …)`, `a reference to (… whose …)`, comparisons inside comparisons, `my x` in arithmetic, a minus sign before a reference or a call, and a reference after `id`
- Fixed escaping inside `|pipes|`, and `app` used as a name
- `front` and `back` print as written instead of as `beginning` and `end`
- Reals print with at most 12 decimal places, like Script Editor. Run-only output keeps every digit
- Long text stored in one block decodes correctly
- `AppleScript Editor` gets Script Editor's terms
- Classic scripts print `window` and `application`, not `Window` and `Application`, which one old scripting addition used
- Terms that only old Mac OS versions define, in the QuickDraw, table, and accessory suites, no longer replace newer names
- The docs are now a reference for the format: every record, typed vector kind, syntax tree node, and opcode, with how often each occurs in real scripts

## 0.2.0

- Added classic Mac OS scripts: scriptview finds the script in a resource fork, in `._name` files and `__MACOSX` folders, and in MacBinary and BinHex files
- Added AppleScript 1.0 scripts (`FasdUAS 1.00`), with their old headers, symbol table, class codes, and empty lists
- `scriptview file.scpt` decompiles without a command, and `-h` and `--help` print the help
- macOS release binaries are signed and notarized
- Release binaries are tested before they're published

## 0.1.0

- Initial release
