# Terminology

A compiled script doesn't store words such as `window` or `display dialog`. It stores four-character codes, such as `cwin` and `sysodlog`. To print the script, a decompiler needs the dictionary that maps each code back to a name. On a Mac, `osadecompile` asks the apps for their dictionaries while it runs. scriptview can't do that on other systems, so it carries the dictionaries with it.

## Where the terms come from

`tools/genterms` runs on a Mac and writes the current terms to `internal/scpt/terms.tsv`. It reads these sources:

- AppleScript's own terms. These aren't in an `.sdef` file, but in an `aeut` resource inside `AppleScript.component`, in the classic Mac OS resource format. The `internal/aete` package parses it.
- Standard Additions (`display dialog`, `do shell script`, and others), from its `.sdef` file.
- The apps that ship with macOS, from their `.sdef` files. Many of these include the Cocoa standard suite with `xi:include`, and the tool follows those includes.
- The SOAP and XML-RPC commands (`call soap`, `call xmlrpc`). These are built into AppleScript, so there's no file to read. Their codes come from a compiled test script.

## Classic Mac OS terms

Scripts from classic Mac OS often use scripting additions and apps that macOS dropped long ago, such as Jon's Commands, PlainTalk's speech commands (`acknowledge`, `click`), Scriptable Text Editor, and the Finder Scripting Extension of System 7. `tools/genclassic` reads their dictionaries, the `aete` resources from Apple's 1993 and 1994 discs, and writes them to `internal/scpt/classic.tsv`. The discs can't be part of the repository, so the tool reads `aete` files that you extract yourself. Its package comment explains how.

scriptview uses these terms only for scripts from classic Mac OS: AppleScript 1.0 files, and scripts read from a resource fork. For any other script, `osadecompile` on a current Mac doesn't know them either and prints raw codes, so scriptview does too. Even for a classic script, a classic term only names a code that the current dictionaries leave raw. A word that prints one way today doesn't change.

The terms have these scopes:

- Scripting additions, and PlainTalk's SR Monitor, which runs speech rules without a `tell`, are in scope everywhere.
- The Finder Scripting Extension's terms belong to the Finder.
- An app's terms belong to the app. Scripts name an app by its file name, which often includes a version (`Progress Bar 1.0.1`), so the terms are under both names.

## When dictionaries disagree

Codes often have more than one name. scriptview follows these rules, which match `osadecompile`:

- Inside a `tell` block, the target app's dictionary wins.
- Outside a `tell` block, the kind of term decides. Where a property can go, a property beats a class, and a class beats a constant. Where a class goes, the class comes first.
- Where only a property can go, such as `x of y`, a constant is never used. `«class extn» of x` stays raw, although Standard Additions has a constant `extensions folder` with the same code.
- Inside one dictionary, a later definition replaces an earlier one. Parameters and AppleScript's own properties are exceptions: the first definition wins.
- Classes and properties that only the old QuickDraw Graphics, Table and Macintosh Connectivity suites define stay raw: `«class ccel»`, not `cell`, and `«class prot»`, not `rotation`. AppleScript's `aeut` still lists these suites, but nothing switches them on (see [Pipes](#pipes)), so AppleScript never prints those names, not even at the top level. `tools/genterms` leaves them out.
- Hidden terms and synonyms come last. They're used only when nothing else names the code.
- `use application "Finder"` makes the Finder's terms available in the whole script, as if everything were inside a `tell` block.

If no dictionary has a code, the output uses the raw form: `«class abcd»`, `«event abcdefgh»` or `«constant enumcode»`. AppleScript accepts these forms when it compiles, so the output still works.

## Pipes

AppleScript lets you use a reserved word as a variable name if you put pipes around it, as in `|name|`. The compiler keeps the variable as an identifier, and the printer decides again whether it needs pipes: it adds them when the name matches a term in scope. Getting the pipes right means knowing exactly which terms are in scope at each point of the script.

These rules decide whether a name needs pipes:

- Only the innermost app counts. Inside `tell application "Finder"` inside `tell application "System Events"`, a variable named like a System Events property doesn't need pipes. This is true even when the inner app isn't installed and has no dictionary.
- Inside a command's arguments, that command's parameter labels count too. In `display alert "a" message (|message| of r)`, the variable needs pipes, because `message` is a label of `display alert`. In `display dialog (message of r)`, it doesn't.
- Inside an app's `tell` block, `type`, `specifier` and `any` need pipes, as in `r's |type|`, although no dictionary has a term with those names. They're value types in the app's scripting definition: the Cocoa standard suite declares parameters with `type="type"` and `type="specifier"`. Outside a `tell` block, `r's type` is fine. An app with no dictionary doesn't count.
- Standard Additions terms count only when Standard Additions is loaded. A script with `use` statements but no `use scripting additions` doesn't load it.
- Words from a few old suites don't need pipes: QuickDraw Graphics (`pixel`, `oval`, `polygon`), Table, and Macintosh Connectivity. AppleScript's dictionary still lists these suites, but nothing switches them on, so the compiler doesn't treat their words as terms.
- The old Type Names suite (`point`, `fixed`, `menu`, `null`, `rotation`, `fixed point`, and more) is switched on by Standard Additions. Standard Additions has an old `aete` resource next to its `.sdef` file, and that `aete` names the suite `tpnm`. Inside Macintosh says that naming a standard suite in an `aete` turns on the whole suite from AppleScript's `aeut`. So these words need pipes when Standard Additions is loaded, and don't when it isn't. Without Standard Additions, `fixed point` is a syntax error.

## Spelling

AppleScript identifiers ignore case. Script Editor prints every use of a variable in the spelling of its first appearance in the file, so `myVar` and `MyVar` in the same script both print as whichever comes first.
