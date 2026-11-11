# Terminology

A compiled script doesn't store words like `window` or `display dialog`. It stores four-character codes: `cwin` and `sysodlog`. To print the script, you need the dictionary that maps each code back to a name. On a Mac, `osadecompile` asks the apps for their dictionaries while it runs. We can't do that on Linux, so we build the dictionaries into scriptview ahead of time.

## Where the words come from

`tools/genterms` reads these on a Mac and writes them to `internal/scpt/terms.tsv`:

- AppleScript's own words. These aren't in an `.sdef` file. They're in an old `aeut` resource inside `AppleScript.component`, in the classic Mac resource fork format. We wrote a small parser for it.
- Standard Additions (`display dialog`, `do shell script` and so on), from its `.sdef` file.
- The apps that ship with macOS, from their `.sdef` files. Many of these include the Cocoa standard suite with `xi:include`, so we follow those includes.
- The SOAP and XML-RPC commands (`call soap`, `call xmlrpc`). These are built into AppleScript's code, so there's no file. We got the codes by compiling a test script and wrote them down by hand.

## Classic Mac OS terms

Scripts from classic Mac OS often use scripting additions and apps that macOS dropped long ago: Jon's Commands, PlainTalk's speech commands (`acknowledge`, `click`), Scriptable Text Editor, the Finder Scripting Extension of System 7. `tools/genclassic` reads their dictionaries, `aete` resources from Apple's 1993 and 1994 discs, and writes them to `internal/scpt/classic.tsv`. We can't include the discs, so the tool reads `aete` files you extract yourself; its comment says how.

These terms are only used for scripts from classic Mac OS: AppleScript 1.0 files, and scripts read from a resource fork. For any other script, `osadecompile` on a current Mac doesn't know them either and prints raw codes, so we do too. Even for a classic script, a classic term only names a code that the current dictionaries leave raw, so a word that prints one way today doesn't change.

Scripting additions, and PlainTalk's SR Monitor, which runs speech rules without a `tell`, are in scope everywhere. The Finder Scripting Extension's terms belong to the Finder, and an app's terms to the app. Scripts name an app by its file name, which often has a version in it (`Progress Bar 1.0.1`), so the terms are under both names.

## When two dictionaries disagree

Codes clash all the time. Some rules we found by testing:

- Inside a `tell` block, the target app's dictionary wins.
- Outside of that, the kind of word decides. In a place where a property can go, a property beats a class, and a class beats a constant. In a place where a class goes, the class wins first.
- In a spot that can only be a property, such as `x of y`, a constant is never used. `«class extn» of x` stays raw, even though Standard Additions has a constant `extensions folder` with the same code.
- Inside one dictionary, a later definition replaces an earlier one. Parameters and AppleScript's own properties are the exception: the first one wins.
- Classes and properties that only the old QuickDraw Graphics, Table and Macintosh Connectivity suites define stay raw: `«class ccel»`, not `cell`, and `«class prot»`, not `rotation`. AppleScript's `aeut` still lists these suites, but nothing switches them on (see Pipes below), so AppleScript never prints those names, not even at the top level. `tools/genterms` leaves them out.
- Hidden terms and synonyms are the last choice. We only use them if nothing else names the code.
- `use application "Finder"` makes Finder's words work in the whole script, as if everything were inside a `tell`.

If no dictionary has the code, the output uses the raw form: `«class abcd»`, `«event abcdefgh»` or `«constant enumcode»`. AppleScript accepts these when you compile, so the output still works.

## Pipes

AppleScript lets you use a reserved word as a variable name if you put pipes around it: `|name|`. The compiler keeps the variable as an identifier, and the printer has to decide again whether it needs pipes. It adds them when the name matches a word in scope. So getting the pipes right means knowing exactly which words are in scope at each point in the script.

Rules we found:

- Only the innermost app counts. Inside `tell application "Finder"` inside `tell application "System Events"`, a variable named like a System Events property doesn't need pipes. This is true even when the inner app isn't installed and has no dictionary.
- Inside a command's arguments, that command's parameter labels count too. In `display alert "a" message (|message| of r)` the variable needs pipes, because `message` is a label of `display alert`. In `display dialog (message of r)` it doesn't.
- Inside an app's `tell` block, `type`, `specifier` and `any` need pipes (`r's |type|`), though no dictionary has a term with those names. They're value types in the app's scripting definition: the Cocoa standard suite declares parameters with `type="type"` and `type="specifier"`. Outside a `tell`, `r's type` is fine. An app with no dictionary doesn't count.
- Standard Additions words only count when Standard Additions is loaded. A script with `use` statements but no `use scripting additions` doesn't load it.
- Words from a few old suites don't need pipes: QuickDraw Graphics (`pixel`, `oval`, `polygon`), Table and Macintosh Connectivity. These are left over from the 1990s. AppleScript's dictionary still lists them, but nothing switches them on, so the compiler doesn't treat them as words.
- The old Type Names suite (`point`, `fixed`, `menu`, `null`, `rotation`, `fixed point` and more) is switched on by Standard Additions. Standard Additions has an old `aete` resource next to its `.sdef`, and that `aete` names the suite `tpnm`. Inside Macintosh says that naming a standard suite in an `aete` turns on the whole suite from AppleScript's `aeut`. So these words need pipes when Standard Additions is loaded, and don't when it isn't. Without Standard Additions, `fixed point` is even a syntax error.

We first found the pipe words by testing every word, and ended up with a list of eight. The research on the Type Names suite explained five of them. The other three, `AppleTalk`, `IP` and `port`, are Standard Additions terms of their own.

## Spelling

AppleScript identifiers ignore case. Script Editor prints every use of a variable in the spelling of its first appearance in the file, so `myVar` and `MyVar` in the same script both print as whichever came first.
