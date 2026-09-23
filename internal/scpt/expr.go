package scpt

import (
	"fmt"
	"strconv"
	"strings"
)

// stmt emits one or more lines for a statement node.
func (dc *decompState) litExpr(n nodeRec) string {
	if len(n.children) == 0 {
		return "<lit>"
	}
	return dc.litRef(n.children[0])
}

// litRef resolves the payload ref of a literal: a negative ref names an
// integer or term; a positive one points at a text, real, date, or app record.
func (dc *decompState) litRef(ch int16) string {
	if o, ok := dc.objects[ch]; ok && o.typ == objList && len(o.refs) == 0 {
		return "{}" // an empty list cell, as AppleScript 1.0 stored {}
	}
	if ch < 0 {
		if v, ok := dc.intByRef[ch]; ok {
			return fmt.Sprintf("%d", v)
		}
		if dc.resolvable(ch) {
			return dc.nameAs(ch, kindValue)
		}
		return fmt.Sprintf("<ref%d>", ch)
	}
	if d, ok := dc.descByID[ch]; ok {
		return descLiteral(d)
	}
	if v, ok := dc.realByID[ch]; ok {
		return formatReal(v)
	}
	if app, ok := dc.appByID[ch]; ok {
		return "application " + quoteAS(displayAppName(app))
	}
	if txt, ok := dc.comments[ch]; ok {
		return quoteAS(txt) // pre-Unicode string literal (0x0c, Mac Roman)
	}
	if dc.wrapType[ch] == wrapAppTerm {
		return dc.nameAs(ch, kindValue)
	}
	if w := dc.wraps[ch]; dc.wrapType[ch] == wrapOptional && len(w) >= 5 {
		// Optional handler parameter: `x as integer : 0`.
		return dc.expr(w[2]) + " : " + dc.expr(w[4])
	}
	if dc.wrapType[ch] == wrapSpecifier {
		// Precompiled object specifier: [container, property/class code].
		w := dc.wraps[ch]
		if len(w) == 2 {
			container := dc.litRef(w[0])
			if app, ok := dc.appByID[w[0]]; ok {
				// Specifiers resolve their application by name, never ".app".
				container = "application " + quoteAS(strings.TrimSuffix(app, ".app"))
			}
			return dc.name(w[1]) + " of " + container
		}
	}
	if txt, ok := dc.wrappedText(ch); ok {
		return quoteAS(txt)
	}
	return fmt.Sprintf("<lit%d>", ch)
}

// callExpr renders an 'I' call node: [name, args, labeled params].
// Builtin (event) commands take a direct parameter and keyword parameters;
// user handlers take a positional list and/or `given label:value` pairs.
func (dc *decompState) callExpr(n nodeRec) string {
	if len(n.children) == 0 {
		return "<call>"
	}
	if w := dc.wraps[n.children[0]]; dc.wrapType[n.children[0]] == wrapAppTerm && len(w) == 2 {
		// A command from a specific application's dictionary.
		if app := dc.appOf(w[1]); app != "" {
			dc.scopes = append(dc.scopes, app)
			defer func() { dc.scopes = dc.scopes[:len(dc.scopes)-1] }()
		}
		n.children = append([]int16{w[0]}, n.children[1:]...)
	}
	name := dc.name(n.children[0])
	if !dc.isBuiltin(n.children[0]) {
		if dc.targeted[dc.curCall] {
			if parts, ok := dc.interleaved(n); ok {
				return parts
			}
		}
		return name + dc.callArgs(n, true)
	}

	saved := dc.cmdLabels
	dc.cmdLabels = map[string]bool{}
	for _, d := range dc.inScope() {
		for label := range d.eventLabels(dc.evCode[n.children[0]]) {
			dc.cmdLabels[label] = true
		}
	}
	defer func() { dc.cmdLabels = saved }()
	if args := dc.commandArgs(n); args != "" {
		if n.flags&flagAltSyntax != 0 {
			if n.flags&3 == 3 {
				// Postfix form: x exists, x count, end run. Keyword
				// parameters follow the name: x «event …» given ….
				direct, rest := dc.commandParts(n)
				if direct != "" && rest != "" {
					return direct + " " + name + " " + rest
				}
				return args + " " + name
			}
			return name + " of " + args // `count of x`, `set eof of f to 0`
		}
		return name + " " + args
	}
	return name
}

// commandArgs renders a command's direct parameter and keyword parameters.
func (dc *decompState) commandArgs(n nodeRec) string {
	direct, rest := dc.commandParts(n)
	if direct == "" {
		return rest
	}
	if rest == "" {
		return direct
	}
	return direct + " " + rest
}

// commandParts renders a command's direct parameter and, separately, its
// keyword parameters.
func (dc *decompState) commandParts(n nodeRec) (string, string) {
	ev := dc.evCode[child0(n)]
	var parts []string
	directPart := ""
	if direct := child(n, 1); direct != 0 {
		if dn, ok := dc.nodes[direct]; ok {
			arg := dc.operand(direct)
			if dn.typ == '6' || dn.typ == 'c' && dc.cmdLabels["as"] {
				// click (first item whose …); do shell script (x as string),
				// where `as string` would be the command's own parameter.
				arg = "(" + arg + ")"
			}
			directPart = arg
		} else if args := dc.argChain(direct); len(args) > 0 {
			directPart = strings.Join(args, ", ")
		}
	}
	var given, with, without []string
	cells := dc.cells(child(n, 2))
	hasOf := false
	cmdParts := map[int]string{} // part index → its parameter label
	for _, cell := range cells {
		hasOf = hasOf || dc.paramLabel(ev, cell[0]) == "of"
	}
	for _, cell := range cells {
		label := dc.paramLabel(ev, cell[0])
		if _, user := dc.refName[cell[0]]; user && dc.boolLit(cell[1]) == "" {
			// A user-defined label on a command handler: `given label:value`.
			given = append(given, dc.name(cell[0])+":"+dc.operand(cell[1]))
			continue
		}
		if strings.HasPrefix(label, "«") && dc.boolLit(cell[1]) == "" {
			// No terminology for this parameter: `given label:value`, naming
			// the label code as a term when possible (e.g. string:).
			if code, ok := dc.osCode[cell[0]]; ok {
				if name := dc.anyTerm4(code); name != "" {
					label = name
				}
			}
			given = append(given, label+":"+dc.operand(cell[1]))
			continue
		}
		switch dc.boolLit(cell[1]) {
		case "true":
			with = append(with, label)
		case "false":
			without = append(without, label)
		default:
			value := dc.expr(cell[1])
			if hasOf && dc.isBareOfRef(cell[1]) || dc.isBareOfRef(cell[1]) && strings.HasPrefix(value, label+" ") {
				value = "(" + value + ")" // offset of x in (item 1 of y), output volume (output volume of s)
			}
			if dc.isCommand(cell[1]) && dc.hasArgs(cell[1]) {
				cmdParts[len(parts)] = label
			}
			parts = append(parts, label+" "+value)
		}
	}
	// Boolean parameters merge: `with a and b without c`.
	if len(with) > 0 {
		parts = append(parts, "with "+andList(with))
	}
	if len(without) > 0 {
		parts = append(parts, "without "+andList(without))
	}
	if len(given) > 0 {
		parts = append(parts, "given "+strings.Join(given, ", "))
	}
	for i, label := range cmdParts {
		if i < len(parts)-1 { // default location (path to …) without invisibles
			parts[i] = label + " (" + strings.TrimPrefix(parts[i], label+" ") + ")"
		}
	}
	return directPart, strings.Join(parts, " ")
}

// isIntLiteral reports whether id is an integer literal.
func (dc *decompState) isIntLiteral(id int16) bool {
	n, ok := dc.nodes[id]
	if !ok || n.typ != 'm' {
		return false
	}
	_, isInt := dc.intByRef[child0(n)]
	return isInt
}

// hasArgs reports whether command node id takes any arguments, which would
// run on into the parameters that follow it.
func (dc *decompState) hasArgs(id int16) bool {
	n := dc.nodes[id]
	if n.typ == 'e' {
		return true
	}
	return child(n, 1) > 0 || child(n, 2) > 0
}

// isTargetedCall reports whether id is a handler call, which binds to the
// nearest `'s` target.
func (dc *decompState) isTargetedCall(id int16) bool {
	n, ok := dc.nodes[dc.unwrap(id)]
	return ok && n.typ == 'I' && !dc.isBuiltin(child0(n))
}

// isBareOfRef reports whether id is `x of y`, or a coercion of one, with no
// parentheses: inside `offset of … in …` its `of` would bind to the command.
func (dc *decompState) isBareOfRef(id int16) bool {
	v, ok := dc.nodes[id]
	if ok && v.typ == 'c' {
		v, ok = dc.nodes[child0(v)]
	}
	return ok && v.typ == 'n' && v.flags&flagPossessive == 0
}

// interleaved renders an Objective-C style handler name such as
// "foo_bar_" with arguments (a, b) as "foo:a bar:b", as Script Editor does
// whenever the name's segments line up with the positional arguments.
func (dc *decompState) interleaved(n nodeRec) (string, bool) {
	raw := dc.refName[child0(n)]
	if !strings.HasSuffix(raw, "_") || child(n, 2) != 0 {
		return "", false
	}
	segs := strings.Split(strings.TrimSuffix(raw, "_"), "_")
	var args []int16
	ref := child(n, 1)
	for steps := 0; ref > 0 && steps < 1000; steps++ {
		cell, ok := dc.paramMap[ref]
		if !ok || len(cell) != 2 {
			break
		}
		args = append(args, cell[0])
		ref = cell[1]
	}
	if len(segs) != len(args) {
		return "", false
	}
	parts := make([]string, len(segs))
	for i, seg := range segs {
		arg := dc.expr(args[i])
		if an, ok := dc.nodes[args[i]]; ok {
			switch an.typ {
			case 'm':
				if code, term := dc.osCode[child0(an)]; term && dc.boolLit(args[i]) == "" && code != "yes " && code != "no  " {
					arg = "(" + arg + ")" // constants: (missing value), (null)
				}
			case '1':
				if strings.Contains(arg, " ") && !strings.HasPrefix(arg, "«") {
					arg = "(" + arg + ")"
				}
			case 'n', '4', '5', '2', '6', '7', 'c', 'H': // references, coercions, not
				arg = "(" + arg + ")"
			default:
				if op, ok := binOps[an.typ]; ok && op.prec <= 7 { // + - and weaker
					arg = "(" + arg + ")"
				}
			}
		}
		parts[i] = dc.quoteSeg(seg) + ":" + arg
	}
	return strings.Join(parts, " "), true
}

// quoteSeg renders one segment of an interleaved handler name, piping it
// when it is reserved or spells a term.
func (dc *decompState) quoteSeg(seg string) string {
	if c, ok := dc.canonical[strings.ToLower(seg)]; ok {
		seg = c
	}
	if dc.isTermWord(seg) && isValidIdent(seg) {
		return "|" + seg + "|"
	}
	return quoteIdent(seg)
}

// interleavedEnd renders the `end` line name for an interleaved handler.
func (dc *decompState) interleavedEnd(raw string) string {
	segs := strings.Split(strings.TrimSuffix(raw, "_"), "_")
	for i, seg := range segs {
		segs[i] = dc.quoteSeg(seg) + ":"
	}
	return strings.Join(segs, "")
}

// reservedWords are AppleScript keywords that must be written |quoted| when
// used as identifiers.
var reservedWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`about above after against and application around as at back
		before beginning behind below beneath beside between but by considering contain contains
		continue copy div does eighth else end equal equals error every exit false fifth first for
		fourth from front get given global if ignoring in into is it its last local me middle mod my
		ninth not of on onto or over prop property put ref reference repeat return returning script
		second set seventh since sixth some tell tenth that the then third through thru timeout
		times to transaction true try until where while whose with without`) {
		m[w] = true
	}
	return m
}()

// quoteIdent wraps name in |bars| when it is not a plain identifier.
func quoteIdent(name string) string {
	if reservedWords[strings.ToLower(name)] || !isValidIdent(name) || (name[0] >= '0' && name[0] <= '9') {
		return "|" + name + "|"
	}
	return name
}

// callArgs renders a user handler's argument list, from either a call or a
// signature: "(a, b)", " given k:v", or both.
func (dc *decompState) callArgs(n nodeRec, isCall bool) string {
	var args []string
	direct := ""
	if a := child(n, 1); a != 0 {
		if _, ok := dc.nodes[a]; ok {
			direct = " of " + dc.operand(a) // `on doIt of x` labeled direct parameter
		} else {
			args = dc.argChain(a)
		}
	}
	var given, withs, withouts []string
	for _, cell := range dc.cells(child(n, 2)) {
		if prep, ok := prepositions[dc.osCode[cell[0]]]; ok {
			value := dc.expr(cell[1])
			if dc.isCommand(cell[1]) && dc.hasArgs(cell[1]) {
				value = "(" + value + ")" // on (path to desktop), but on current date
			}
			direct += " " + prep + " " + value
			continue
		}
		switch dc.boolLit(cell[1]) {
		case "true":
			withs = append(withs, dc.name(cell[0]))
		case "false":
			withouts = append(withouts, dc.name(cell[0]))
		default:
			given = append(given, dc.name(cell[0])+":"+dc.operand(cell[1]))
		}
	}
	out := direct
	if direct == "" && (len(args) > 0 || len(given)+len(withs)+len(withouts) == 0) {
		out = "(" + strings.Join(args, ", ") + ")"
	}
	if len(withs) > 0 {
		out += " with " + andList(withs)
	}
	if len(withouts) > 0 {
		out += " without " + andList(withouts)
	}
	if len(given) > 0 {
		out += " given " + strings.Join(given, ", ")
	}
	return out
}

// cells walks a chain of 3-slot [key, value, next] cells (records, labeled
// parameters) and returns each cell.
func (dc *decompState) cells(ref int16) [][]int16 {
	var out [][]int16
	seen := make(map[int16]bool)
	for ref > 0 && !seen[ref] {
		seen[ref] = true
		cell, ok := dc.paramMap[ref]
		if !ok || len(cell) != 3 {
			break
		}
		out = append(out, cell)
		ref = cell[2]
	}
	return out
}

// boolLit returns "true"/"false" if id is a boolean literal, else "".
func (dc *decompState) boolLit(id int16) string {
	n, ok := dc.nodes[id]
	if !ok || n.typ != 'm' || len(n.children) == 0 {
		return ""
	}
	switch dc.osCode[n.children[0]] {
	case "true":
		return "true"
	case "fals":
		return "false"
	}
	return ""
}

// prepositions are the labels AppleScript allows for user handler
// parameters, keyed by code: `on rangeSum from a to b`.
var prepositions = map[string]string{
	"abou": "about", "abve": "above", "agst": "against", "aprt": "apart from",
	"arnd": "around", "asdf": "aside from", "at  ": "at", "belw": "below",
	"bnth": "beneath", "bsid": "beside", "btwn": "between", "by  ": "by",
	"for ": "for", "from": "from", "isto": "instead of", "into": "into",
	"of  ": "of", "on  ": "on", "onto": "onto", "outo": "out of", "over": "over",
	"snce": "since", "thru": "thru", "undr": "under", "to  ": "to", "in  ": "in",
}

// useParams names the labeled parameters of `use` statements.
var useParams = map[string]string{"minv": "version", "impt": "importing"}

// numericOrdinal renders 1 as "1st", 2 as "2nd", 11 as "11th", …
func numericOrdinal(v int) string {
	suffix := "th"
	if v%100 < 11 || v%100 > 13 {
		switch v % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(v) + suffix
}

// andList joins items as "a, b and c".
func andList(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// isCommand reports whether id is a builtin command invocation, which needs
// parentheses when nested inside another expression.
func (dc *decompState) isCommand(id int16) bool {
	n, ok := dc.nodes[id]
	if ok && n.typ == 'e' {
		return true
	}
	return ok && n.typ == 'I' && len(n.children) > 0 && dc.isBuiltin(n.children[0])
}

// operand renders id as an expression, parenthesizing builtin commands.
func (dc *decompState) operand(id int16) string {
	if dc.isCommand(id) {
		return "(" + dc.expr(id) + ")"
	}
	return dc.expr(id)
}

// joinItems renders a cons chain as a comma-separated list, keeping `¬`
// breaks the source placed before a comma.
func (dc *decompState) joinItems(ref int16) string {
	var sb strings.Builder
	seen := make(map[int16]bool)
	for i := 0; ref > 0 && !seen[ref]; i++ {
		seen[ref] = true
		cell, ok := dc.paramMap[ref]
		if !ok || len(cell) != 2 {
			break
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		if cell[0] > 0 {
			sb.WriteString(dc.expr(cell[0]))
		} else if dc.resolvable(cell[0]) {
			sb.WriteString(dc.name(cell[0]))
		}
		ref = cell[1]
	}
	return sb.String()
}

// argChain walks a paramMap linked list collecting argument expressions.
func (dc *decompState) argChain(ref int16) []string {
	var args []string
	seen := make(map[int16]bool)
	for ref > 0 && !seen[ref] {
		seen[ref] = true
		entry, ok := dc.paramMap[ref]
		if !ok || len(entry) != 2 {
			break
		}
		headID, tail := entry[0], entry[1]
		if headID > 0 {
			if a := dc.expr(headID); a != "" {
				args = append(args, a)
			}
		} else if headID < 0 {
			if dc.resolvable(headID) {
				args = append(args, dc.name(headID))
			}
		}
		if tail <= 0 {
			break
		}
		ref = tail
	}
	return args
}

// binOp describes an infix operator node: its source spelling and binding
// precedence (higher binds tighter), used to decide where parentheses go.
type binOp struct {
	tok  string
	prec int
}

// binOps maps FASD node types to infix operators. Arithmetic and comparison
// opcodes are allocated sequentially by the compiler.
var binOps = map[byte]binOp{
	'a':  {"^", 9},
	']':  {"*", 8},
	'^':  {"/", 8},
	'_':  {"div", 8},
	'`':  {"mod", 8},
	'[':  {"+", 7},
	'\\': {"-", 7},
	'b':  {"&", 6},
	'A':  {"<", 4},
	'B':  {"≤", 4},
	'?':  {">", 4},
	'@':  {"≥", 4},
	'C':  {"starts with", 4},
	'D':  {"ends with", 4},
	'E':  {"contains", 4},
	'=':  {"=", 3},
	'>':  {"≠", 3},
	'F':  {"and", 1},
	'G':  {"or", 0},
}

// opSynonyms maps (operator node type, header flags) to the English spelling
// the source used, e.g. "is" or "comes before" instead of a symbol.
var opSynonyms = map[byte]map[uint16]string{
	'=': {0x0003: "is", 0x0001: "is equal to"},
	'>': {0x0100: "is not", 0x0001: "is not equal to"},
	'A': {0x0001: "is less than", 0x0101: "comes before", 0x0100: "is not greater than or equal to"},
	'?': {0x0001: "is greater than", 0x0101: "comes after", 0x0100: "is not less than or equal to"},
	'B': {0x0001: "is less than or equal to", 0x0100: "is not greater than", 0x0101: "does not come after"},
	'@': {0x0001: "is greater than or equal to", 0x0100: "is not less than", 0x0101: "does not come before"},
	'C': {0x0100: "begins with"},
}

// ordinals names the element indexes AppleScript prints as words.
var ordinals = map[int]string{
	1: "first", 2: "second", 3: "third", 4: "fourth", 5: "fifth",
	6: "sixth", 7: "seventh", 8: "eighth", 9: "ninth", 10: "tenth", -1: "last",
}

// plural returns the plural form of a class name, as used by "windows" or
// "text items".
func plural(name string) string {
	switch {
	case strings.HasSuffix(name, "text"), strings.HasPrefix(name, "«"):
		return name
	case strings.HasSuffix(name, "s"), strings.HasSuffix(name, "x"),
		strings.HasSuffix(name, "ch"), strings.HasSuffix(name, "sh"):
		return name + "es"
	case strings.HasSuffix(name, "y") && !strings.ContainsAny(name[len(name)-2:len(name)-1], "aeiou"):
		return name[:len(name)-1] + "ies"
	}
	return name + "s"
}

const (
	precAs   = 5
	precNot  = 2
	precAtom = 10
)

// expr returns an inline expression string (no indent, no newline).
func (dc *decompState) expr(id int16) string {
	if dc.nesting > maxNesting || dc.active[id] {
		return "«…»" // damaged file: cyclic references
	}
	if dc.active == nil {
		dc.active = map[int16]bool{}
	}
	dc.active[id] = true
	dc.nesting++
	defer func() { dc.nesting--; delete(dc.active, id) }()
	n, ok := dc.nodes[id]
	if !ok {
		return dc.litRef(id)
	}

	if op, ok := binOps[n.typ]; ok {
		if len(n.children) < 2 {
			return "<" + op.tok + ">"
		}
		if n.typ == 'E' && n.flags&flagAltSyntax != 0 {
			container := dc.sub(n.children[0], op.prec+1)
			if dc.isMyCall(n.children[0]) {
				container = dc.expr(n.children[0]) // x is in my list
			}
			return dc.sub(n.children[1], op.prec) + " is in " + container
		}
		if (n.typ == '=' || n.typ == '>') && n.flags == flagPossessive {
			// `x is running` compiles to `running of x = true`.
			if p, ok := dc.nodes[n.children[0]]; ok && p.typ == 'n' && len(p.children) == 2 {
				verb := " is "
				if n.typ == '>' {
					verb = " is not "
				}
				return dc.expr(p.children[1]) + verb + dc.expr(p.children[0])
			}
		}
		if alt, ok := opSynonyms[n.typ][n.flags]; ok {
			op.tok = alt
		}
		// Script Editor parenthesizes `my f()` only as the right operand of
		// arithmetic.
		left, right := dc.sub(n.children[0], op.prec), dc.sub(n.children[1], op.prec+1)
		if op.prec >= 7 { // Script Editor parenthesizes references in arithmetic
			if dc.isArithRef(n.children[0]) {
				left = "(" + left + ")"
			}
			if dc.isArithRef(n.children[1]) {
				right = "(" + right + ")"
			}
		}
		if r, ok := dc.nodes[n.children[1]]; ok && op.prec >= 8 && r.typ == 'I' && !dc.isBuiltin(child0(r)) {
			right = "(" + right + ")" // 2 * (f(x))
		}
		if dc.isMyCall(n.children[0]) && op.prec < 7 {
			left = dc.expr(n.children[0])
		}
		if dc.isMyCall(n.children[1]) && op.prec < 7 {
			right = dc.expr(n.children[1])
		}
		if n.typ == 'C' || n.typ == 'D' || n.typ == 'E' { // coercions tested for containment
			if c, ok := dc.nodes[n.children[0]]; ok && c.typ == 'c' {
				left = "(" + dc.expr(n.children[0]) + ")"
			}
			if c, ok := dc.nodes[n.children[1]]; ok && c.typ == 'c' {
				right = "(" + dc.expr(n.children[1]) + ")"
			}
		}
		return left + " " + op.tok + " " + right
	}

	switch n.typ {
	case 'o': // variable reference
		if len(n.children) == 0 {
			return "<var>"
		}
		return dc.name(n.children[0])

	case 'm':
		return dc.litExpr(n)

	case 'I':
		dc.curCall = id
		return dc.callExpr(n)

	case 'l': // source-level wrapper: parentheses and/or the article "the"
		ch := firstPositive(n)
		if ch == 0 {
			return "()"
		}
		// Breaks inside a continued expression indent one level deeper.
		deepen := n.flags&(flagContinuation|flagPossessive) == flagContinuation
		if deepen {
			dc.contDepth++
		}
		inner := dc.expr(ch)
		if deepen {
			dc.contDepth--
		}
		if n.flags&flagThe != 0 {
			inner = "the " + inner
		}
		if n.flags&flagParens != 0 {
			inner = "(" + inner + ")"
		}
		if n.flags&flagContinuation != 0 {
			if n.flags&flagPossessive != 0 {
				inner += dc.contMark() // `¬` after this expression
			} else {
				inner = dc.contMark() + inner // `¬` before it
			}
		}
		return inner

	case 'H': // not — folded into "does not contain", "is not in", …
		if inner, ok := dc.nodes[child(n, 0)]; ok && len(inner.children) == 2 {
			a, b := inner.children[0], inner.children[1]
			verb := ""
			switch {
			case inner.typ == 'E' && inner.flags&flagAltSyntax != 0:
				a, b, verb = b, a, "is not in"
			case inner.typ == 'E':
				verb = "does not contain"
			case inner.typ == 'D':
				verb = "does not end with"
			case inner.typ == 'C' && inner.flags == 0x100:
				verb = "does not begin with"
			case inner.typ == 'C':
				verb = "does not start with"
			}
			if verb != "" {
				p := binOps[inner.typ].prec
				left := dc.sub(a, p)
				if dc.isMyCall(a) {
					left = dc.expr(a)
				}
				return left + " " + verb + " " + dc.sub(b, p+1)
			}
		}
		if dc.isMyCall(child(n, 0)) || dc.isCommand(child(n, 0)) {
			return "not " + dc.expr(child(n, 0)) // not exists x
		}
		return "not " + dc.sub(child(n, 0), precNot)

	case 'd': // unary minus
		return "-" + dc.sub(child(n, 0), precAtom)

	case 'c': // coercion: [value, class]
		if len(n.children) < 2 {
			return "<as>"
		}
		left := dc.sub(n.children[0], precAs)
		if dc.isMyCall(n.children[0]) {
			left = dc.expr(n.children[0])
		}
		return left + " as " + dc.expr(n.children[1])

	case 'n': // property/element "of": [part, container]
		if len(n.children) < 2 {
			return "<of>"
		}
		if n.flags&(flagPossessive|flagAltSyntax) != 0 {
			dc.targetCall(n.children[0]) // a call with a target: x's foo:y, my foo:y
		}
		if c, ok := dc.nodes[dc.unwrapBreak(n.children[1])]; ok && c.typ == 'f' && n.flags&flagAltSyntax != 0 {
			// A `¬` break before `my` wraps `me`; Script Editor drops it.
			p, ok := dc.nodes[n.children[0]]
			parens := ok && (binOps[p.typ].tok != "" || (p.typ == 'n' && dc.isInterleavedCall(child0(p))))
			saved := dc.inMy
			dc.inMy = parens
			part := dc.expr(n.children[0])
			dc.inMy = saved
			if parens {
				part = "(" + part + ")"
			}
			return "my " + part
		}
		if n.flags&flagPossessive != 0 {
			if c, ok := dc.nodes[n.children[1]]; ok && c.typ == 'g' && n.flags&flagImplicitIts != 0 {
				if code := dc.osCode[child0(dc.nodes[n.children[0]])]; dc.className4(code) == "" {
					return dc.expr(n.children[0]) // name: a property of the tell target
				}
				return "its " + dc.expr(n.children[0]) // its file: file alone is the class
			}
			if c, ok := dc.nodes[n.children[1]]; ok && c.typ == 'g' {
				if n.flags&flagAltSyntax != 0 {
					return "it's " + dc.expr(n.children[0]) // kept by old compilers
				}
				return "its " + dc.expr(n.children[0])
			}
			inMy := dc.inMy
			dc.inMy = false
			container := dc.expr(n.children[1])
			dc.inMy = inMy
			if c, ok := dc.nodes[n.children[1]]; ok && (c.typ == 'c' || binOps[c.typ].tok != "" || dc.isOfRef(n.children[1]) && c.flags&flagPossessive == 0 && dc.isTargetedCall(n.children[0])) {
				container = "(" + container + ")" // (a & b)'s length, (class "X" of y)'s f()
			} else if ok && !inMy && c.typ != 'l' && dc.endsInInterleavedCall(n.children[1]) {
				container = "(" + container + ")" // (x's foo:y)'s bar
			}
			return container + "'s " + dc.expr(n.children[0])
		}
		if p, ok := dc.nodes[n.children[0]]; ok && (p.typ == '8' || p.typ == '9') {
			// Insertion location: `before x`, `after x`, `front of x`, `back of x`.
			words := map[byte]string{'8': "before ", '9': "after "}
			if p.flags&flagThe != 0 {
				words = map[byte]string{'8': "front of ", '9': "back of "}
			}
			prefix := ""
			if c := child0(p); dc.osCode[c] != "" && dc.osCode[c] != "insl" {
				prefix = dc.className(c) + " " // insertion point before x
			}
			return prefix + words[p.typ] + dc.expr(n.children[1])
		}
		container := dc.expr(n.children[1])
		if c, ok := dc.nodes[n.children[1]]; dc.isCommand(n.children[1]) || ok && (c.typ == 'c' || c.typ == '6' || binOps[c.typ].tok != "") {
			container = "(" + container + ")" // hours of (current date), x of (a & b)
		}
		part := dc.expr(n.children[0])
		if p, ok := dc.nodes[n.children[0]]; ok && p.typ == 'm' && dc.osCode[child0(p)] != "" {
			part = dc.nameAs(child0(p), kindProperty) // old compilers: a literal property code
		}
		if n.flags&flagThe != 0 {
			return part + " in " + container
		}
		return part + " of " + container

	case 'r': // assignment in expression context — its value
		return dc.expr(child(n, 0))

	case '1': // property or class name
		if len(n.children) == 0 {
			return "<ref>"
		}
		return dc.name(n.children[0])

	case '2': // every <class>
		if len(n.children) == 0 {
			return "every <>"
		}
		if n.flags&flagAltSyntax != 0 {
			return dc.pluralOf(n.children[0])
		}
		return "every " + dc.className(n.children[0])

	case '3': // some <class>
		return "some " + dc.className(child0(n))

	case '<': // middle <class>
		return "middle " + dc.className(child0(n))

	case '4': // <class> <index>
		if len(n.children) < 2 {
			return "<elem>"
		}
		if dc.osCode[n.children[0]] == "capp" {
			if lit, ok := dc.nodes[n.children[1]]; ok && lit.typ == 'm' {
				if t, ok := dc.wrappedText(child0(lit)); ok {
					if renamed, ok := scopeAliases[t]; ok {
						return "application " + quoteAS(renamed)
					}
				}
			}
		}
		if n.flags&flagNumericOrdinal != 0 {
			if idx, ok := dc.nodes[n.children[1]]; ok && idx.typ == 'm' && len(idx.children) > 0 {
				if v, ok := dc.intByRef[idx.children[0]]; ok && v > 0 {
					return numericOrdinal(v) + " " + dc.className(n.children[0])
				}
			}
		}
		if n.flags&(flagAltSyntax|flagThe) != 0 {
			if idx, ok := dc.nodes[n.children[1]]; ok && idx.typ == 'm' && len(idx.children) > 0 {
				if v, ok := dc.intByRef[idx.children[0]]; ok {
					words := ordinals
					if n.flags&flagThe != 0 {
						words = map[int]string{1: "front", -1: "back"}
					}
					if w := words[v]; w != "" {
						return w + " " + dc.className(n.children[0])
					}
				}
			}
		}
		class := dc.className(n.children[0])
		if dc.osCode[n.children[0]] == "****" {
			class = "any" // any button: anything, indexed by a class
		}
		return class + " " + dc.sub(n.children[1], precAtom)

	case '7': // <class> <from> thru <to>
		if len(n.children) < 3 {
			return "<range>"
		}
		cls := dc.pluralOf(n.children[0])
		if strings.HasPrefix(cls, "every «") {
			cls = strings.TrimPrefix(cls, "every ") // raw codes: «class x» 1 thru 2
		}
		if n.flags&flagThe == 0 && !(dc.isIntLiteral(n.children[1]) && dc.isIntLiteral(n.children[2])) {
			// `characters from x to y`; with two numbers it prints as thru.
			return cls + " from " + dc.sub(n.children[1], precAtom) + " to " + dc.sub(n.children[2], precAtom)
		}
		thru := " thru "
		if n.flags&flagAltSyntax != 0 {
			thru = " through "
		}
		return cls + " " + dc.sub(n.children[1], precAtom) + thru + dc.sub(n.children[2], precAtom)

	case ';':
		return "end"

	case 'f':
		return "me"

	case 'O': // bytecode decompiler: a tell used as a value
		return "(tell " + dc.expr(child(n, 1)) + " to " + dc.expr(child(n, 0)) + ")"

	case '~': // bytecode decompiler: optional parameter `x as class : default`
		return dc.expr(child(n, 0)) + " : " + dc.expr(child(n, 1))

	case 'g':
		return "it"

	case 'M': // continue: pass a call to the parent script
		dc.targetCall(child(n, 0)) // continue initWithFrame:frame
		return "continue " + dc.expr(child(n, 0))

	case 'e': // get
		return "get " + dc.expr(child(n, 0))

	case ':':
		return "beginning"

	case '5': // <class> id <value>
		if len(n.children) < 2 {
			return "<by-id>"
		}
		form := " id "
		if len(n.children) > 2 {
			switch dc.osCode[n.children[2]] {
			case "name":
				form = " named "
			case "indx":
				form = " index "
			}
		}
		return dc.className(n.children[0]) + form + dc.sub(n.children[1], precAtom)

	case 'N':
		return "a reference to " + dc.expr(child(n, 0))

	case '6': // filter: every X whose condition
		if len(n.children) < 2 {
			return "<filter>"
		}
		kw := " whose "
		switch {
		case n.flags&flagAltSyntax != 0:
			kw = " where "
		case n.flags&flagPossessive != 0:
			kw = " that "
		}
		return dc.expr(n.children[0]) + kw + dc.expr(n.children[1])

	case 'J': // list literal: children[0] is a cons chain
		return "{" + dc.joinItems(child(n, 0)) + "}"

	case 'v': // list literal written with brackets
		return "[" + dc.joinItems(child(n, 0)) + "]"

	case 'K': // record literal: children[0] is a chain of [key, value, next] cells
		var fields []string
		ref := child(n, 0)
		seen := make(map[int16]bool)
		for ref > 0 && !seen[ref] {
			seen[ref] = true
			cell, ok := dc.paramMap[ref]
			if !ok || len(cell) != 3 {
				break
			}
			fields = append(fields, dc.name(cell[0])+":"+dc.expr(cell[1]))
			ref = cell[2]
		}
		return "{" + strings.Join(fields, ", ") + "}"

	default:
		return fmt.Sprintf("<%c%d>", n.typ, id)
	}
}

// sub renders child id, parenthesizing it if it is an operator binding
// looser than minPrec. Explicit source parentheses ('l') are kept as-is.
func (dc *decompState) sub(id int16, minPrec int) string {
	if dc.isCommand(id) {
		return "(" + dc.expr(id) + ")"
	}
	s := dc.expr(id)
	if dc.prec(id) < minPrec {
		return "(" + s + ")"
	}
	return s
}

// prec returns the binding precedence of the expression rooted at id.
func (dc *decompState) prec(id int16) int {
	if dc.helperDepth > maxNesting {
		return precAtom // damaged file: cyclic references
	}
	dc.helperDepth++
	defer func() { dc.helperDepth-- }()
	n, ok := dc.nodes[id]
	if !ok {
		return precAtom
	}
	if op, ok := binOps[n.typ]; ok {
		return op.prec
	}
	switch n.typ {
	case 'I': // interleaved calls (`x's foo:y`) are parenthesized as operands
		if _, ok := dc.interleaved(n); ok && dc.targeted[id] {
			return -1
		}
	case 'n':
		if dc.isMyCall(id) {
			return -1
		}
		if n.flags&flagPossessive != 0 {
			return dc.prec(child0(n))
		}
	case 'H':
		return precNot
	case 'c':
		return precAs
	case '6': // a whose clause runs to the end of the expression
		return 0
	}
	return precAtom
}

// targetCall marks id, if it is a user handler call, as having an explicit
// target so that it renders with interleaved syntax.
func (dc *decompState) targetCall(id int16) {
	n, ok := dc.nodes[id]
	for steps := 0; ok && n.typ == 'l' && steps < 64; steps++ {
		id = firstPositive(n)
		n, ok = dc.nodes[id]
	}
	if ok && n.typ == 'I' {
		if dc.targeted == nil {
			dc.targeted = map[int16]bool{}
		}
		dc.targeted[id] = true
	}
}

// isOfRef reports whether id is a bare `x of y` reference (not my/its/'s).
func (dc *decompState) isOfRef(id int16) bool {
	n, ok := dc.nodes[id]
	if !ok || n.typ != 'n' || n.flags&flagThe != 0 || dc.isMyCall(id) {
		return false
	}
	if p, ok := dc.nodes[child0(n)]; ok && (p.typ == '8' || p.typ == '9') {
		return false
	}
	return true
}

// isArithRef reports whether id is a reference that Script Editor puts in
// parentheses inside arithmetic: `(x of y) + 1`, `(x's y) + 1`, `(item v) - 2`.
func (dc *decompState) isArithRef(id int16) bool {
	if n, ok := dc.nodes[id]; ok && n.typ == '4' {
		return true
	}
	return dc.isOfRef(id)
}

// unwrapBreak skips a wrapper that only records a `¬` break before id.
func (dc *decompState) unwrapBreak(id int16) int16 {
	if n, ok := dc.nodes[id]; ok && n.typ == 'l' && n.flags&(flagParens|flagThe) == 0 && n.flags&flagContinuation != 0 {
		return firstPositive(n)
	}
	return id
}

// unwrap skips 'l' wrappers (parentheses, the, continuations) around id.
func (dc *decompState) unwrap(id int16) int16 {
	for i := 0; i < 16; i++ {
		n, ok := dc.nodes[id]
		if !ok || n.typ != 'l' {
			break
		}
		id = firstPositive(n)
	}
	return id
}

// endsInInterleavedCall reports whether id renders as a targeted interleaved
// call (x's foo:y), which needs parentheses before another 's.
func (dc *decompState) endsInInterleavedCall(id int16) bool {
	if dc.helperDepth > maxNesting {
		return false // damaged file: cyclic references
	}
	dc.helperDepth++
	defer func() { dc.helperDepth-- }()
	n, ok := dc.nodes[id]
	if !ok {
		return false
	}
	if n.typ == 'n' && n.flags&(flagPossessive|flagAltSyntax) != 0 {
		return dc.endsInInterleavedCall(child0(n))
	}
	return n.typ == 'I' && dc.targeted[id] && dc.isInterleavedCall(id)
}

// isParenExpr reports whether an 'l' node is a parenthesized expression used
// as a statement, rather than a (possibly commented) source line.
func (dc *decompState) isParenExpr(n nodeRec) bool {
	_, comment := dc.comments[child(n, 1)]
	return n.typ == 'l' && n.flags&flagParens != 0 && !comment
}

// isInterleavedCall reports whether id (possibly parenthesized) is a user
// handler call that renders with interleaved syntax.
func (dc *decompState) isInterleavedCall(id int16) bool {
	n, ok := dc.nodes[id]
	for steps := 0; ok && n.typ == 'l' && steps < 64; steps++ {
		id = firstPositive(n)
		n, ok = dc.nodes[id]
	}
	if !ok || n.typ != 'I' || dc.isBuiltin(child0(n)) {
		return false
	}
	_, inter := dc.interleaved(n)
	return inter
}

// isMyCall reports whether id is `my handler(…)`.
func (dc *decompState) isMyCall(id int16) bool {
	n, ok := dc.nodes[id]
	if !ok || n.typ != 'n' {
		return false
	}
	c, ok := dc.nodes[dc.unwrapBreak(child(n, 1))]
	return ok && c.typ == 'f' && n.flags&flagAltSyntax != 0
}
