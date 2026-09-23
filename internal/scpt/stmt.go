package scpt

import "strings"

// topLevelStmts finds top-level statements (not inside handler bodies).
func (dc *decompState) topLevelStmts(handlerBodyIDs map[int16]bool) []string {
	for _, id := range dc.nodeOrder {
		n := dc.nodes[id]
		if n.typ != 'k' || handlerBodyIDs[id] {
			continue
		}
		if len(n.children) == 0 || n.children[0] <= 0 {
			continue
		}
		lines := dc.block(id, 0)
		if len(lines) > 0 {
			return lines
		}
	}
	return nil
}

// handlerDef emits `on name(params) … end name`. The 'i' node is
// [signature, body]; the 'I' signature is [name, positional chain, given chain].
func (dc *decompState) handlerDef(id int16, depth int) []string {
	indent := strings.Repeat("\t", depth)
	n := dc.nodes[id]
	if len(n.children) < 2 {
		return []string{indent + "-- incomplete handler"}
	}
	sig, ok := dc.nodes[n.children[0]]
	if !ok || sig.typ != 'I' || len(sig.children) == 0 {
		return []string{indent + "-- missing handler signature"}
	}
	name := dc.name(sig.children[0])
	kw := "on "
	if n.flags&flagThe != 0 {
		kw = "to " // `to doIt()` spelling
	}
	head := kw + name
	if dc.isBuiltin(sig.children[0]) {
		// Event handler (on run, on open x, on idle): command-style parameters.
		if args := dc.commandArgs(sig); args != "" {
			if sig.flags&flagAltSyntax != 0 {
				head += " of"
			}
			head += " " + args
		}
	} else if parts, ok := dc.interleaved(sig); ok {
		head = kw + parts
		name = dc.interleavedEnd(dc.refName[sig.children[0]])
	} else {
		head += dc.callArgs(sig, false)
	}
	lines := []string{dc.hdr(indent+head, n.children[1])}
	lines = append(lines, dc.body(n.children[1], depth+1)...)
	return append(lines, indent+"end "+name)
}

// block walks a 'k' node's statement linked list via paramMap.
// Each paramMap entry is a 2-element cons cell [head-stmt-id, tail-ref].
func (dc *decompState) block(kID int16, depth int) []string {
	kNode, ok := dc.nodes[kID]
	if !ok {
		return nil
	}
	if kNode.typ != 'k' {
		if kNode.typ == 'l' && (kNode.flags&flagThe != 0 || dc.isContinuedExpr(kNode)) {
			// A lone expression as the whole body: `the (current date)'s time`.
			return []string{strings.Repeat("\t", depth) + dc.expr(kID)}
		}
		return dc.stmt(kID, depth)
	}

	var lines []string
	ref := child(kNode, 0)
	seen := make(map[int16]bool)
	for ref > 0 && !seen[ref] {
		seen[ref] = true
		entry, ok := dc.paramMap[ref]
		if !ok || len(entry) != 2 {
			break
		}
		if entry[0] > 0 {
			lines = append(lines, dc.stmt(entry[0], depth)...)
		}
		ref = entry[1]
	}
	return lines
}

// tellStmt renders 'O': [body, target]. A body that is a lone statement rather
// than a 'k' block came from the one-line `tell X to stmt` form.
func (dc *decompState) tellStmt(n nodeRec, depth int) []string {
	indent := strings.Repeat("\t", depth)
	if len(n.children) < 2 {
		return []string{indent + "-- tell (incomplete)"}
	}
	target := dc.expr(n.children[1])
	if app := dc.appOf(n.children[1]); app != "" {
		dc.scopes = append(dc.scopes, app)
		defer func() { dc.scopes = dc.scopes[:len(dc.scopes)-1] }()
	}
	if n.flags&flagAltSyntax != 0 {
		dc.targetCall(n.children[0])
		brk := dc.stmtBreak(n.children[0])
		if brk {
			dc.contDepth++
		}
		inner := dc.stmt(n.children[0], 0)
		if brk {
			dc.contDepth--
		}
		if len(inner) == 1 {
			to := " to "
			if dc.stmtBreak(n.children[0]) {
				to = " to" + dc.contMark()
			}
			return []string{indent + "tell " + target + to + inner[0]}
		}
		// `tell a to tell b`, `tell a to repeat …`: a compound statement
		// follows `to` and carries the block.
		if bn, ok := dc.nodes[n.children[0]]; ok && bn.typ != 'k' {
			if brk { // `tell a to ¬` then the compound statement one level in
				return append([]string{indent + "tell " + target + " to ¬"}, dc.stmt(n.children[0], depth+1)...)
			}
			inner := dc.stmt(n.children[0], depth)
			if len(inner) > 0 {
				inner[0] = indent + "tell " + target + " to " + strings.TrimLeft(inner[0], "\t")
			}
			return inner
		}
	}
	lines := []string{dc.hdr(indent+"tell "+target, n.children[0])}
	lines = append(lines, dc.body(n.children[0], depth+1)...)
	return append(lines, indent+"end tell")
}

// tryStmt renders 'Q': [body, R (on error spec), handler body?].
func (dc *decompState) tryStmt(n nodeRec, depth int) []string {
	indent := strings.Repeat("\t", depth)
	lines := []string{dc.hdr(indent+"try", child(n, 0))}
	lines = append(lines, dc.body(child(n, 0), depth+1)...)
	handler := child(n, 2)
	if spec, ok := dc.nodes[child(n, 1)]; ok && spec.typ == 'R' {
		clause := dc.errorClause(spec)
		if handler != 0 || clause != "error" {
			lines = append(lines, dc.hdr(indent+"on "+clause, handler))
		}
	}
	lines = append(lines, dc.body(handler, depth+1)...)
	return append(lines, indent+"end try")
}

// errorClause renders an 'R' node, [error-keyword, message, labeled params],
// shared by the `error` command and the `on error` clause.
func (dc *decompState) errorClause(n nodeRec) string {
	const ev = "ascrerr "
	parts := []string{"error"}
	if msg := child(n, 1); msg != 0 {
		parts = append(parts, dc.operand(msg))
	}
	for _, cell := range dc.cells(child(n, 2)) {
		label := dc.paramLabel(ev, cell[0])
		parts = append(parts, label+" "+dc.operand(cell[1]))
	}
	return strings.Join(parts, " ")
}

// isStmtNode returns true for node types that can head a statement.
func isStmtNode(typ byte) bool {
	switch typ {
	case 'l', 'r', 's', 'L', 'Z', 'O', 'Q', 'T', 'U', 'V', 'W', 'X', 'Y', 'S', 'I', 'k', 'i', 'j', 'p', 'q', 'x', 'P', 'R', 't', 'u', 'w', 'h':
		return true
	}
	return false
}

// maxNesting bounds rendering recursion; well-formed trees are shallow, but
// damaged files can contain reference cycles.
const maxNesting = 300

// stmt emits one or more lines for a statement node.
func (dc *decompState) stmt(id int16, depth int) []string {
	if dc.nesting > maxNesting || dc.activeStmt[id] {
		return []string{"-- (damaged file: cyclic statement)"}
	}
	if dc.activeStmt == nil {
		dc.activeStmt = map[int16]bool{}
	}
	dc.activeStmt[id] = true
	dc.nesting++
	defer func() { dc.nesting--; delete(dc.activeStmt, id) }()
	n, ok := dc.nodes[id]
	if !ok {
		return nil
	}
	indent := strings.Repeat("\t", depth)
	line := func(s string) []string { return []string{indent + s} }

	switch n.typ {
	case 'l': // source line: [statement, comment, comment text]; empty = blank line
		if cn, ok := dc.nodes[child(n, 0)]; ok && dc.isParenExpr(n) && (!isStmtNode(cn.typ) || cn.typ == 'I') {
			return line(dc.expr(id)) // a parenthesized expression statement
		}
		if dc.isContinuedExpr(n) {
			return line(dc.expr(id)) // an expression statement after `¬` breaks
		}
		var out []string
		if ch := child(n, 0); ch != 0 {
			if cn, ok := dc.nodes[ch]; ok && (!isStmtNode(cn.typ) || dc.isParenExpr(cn)) {
				out = line(dc.stmtExpr(ch))
			} else {
				out = dc.stmt(ch, depth)
			}
		}
		if c := dc.comment(n); c != "" && !dc.hoisted[id] {
			if len(out) == 0 {
				return line(c)
			}
			out[len(out)-1] += " " + c
		}
		if len(out) == 0 {
			if dc.hoisted[id] {
				return nil
			}
			return []string{indent} // Script Editor indents blank lines too
		}
		return out

	case 'i':
		return dc.handlerDef(id, depth)

	case 'j': // property: [name, initial value]
		if len(n.children) < 2 {
			return line("-- property (incomplete)")
		}
		return line("property " + dc.name(n.children[0]) + " : " + dc.expr(n.children[1]))

	case 'p': // global: [name chain]
		return line("global " + strings.Join(dc.argChain(child(n, 0)), ", "))

	case 'x': // use: [name, target, labeled params]
		head := "use "
		if nm := child0(n); nm < 0 && dc.resolvable(nm) {
			head += dc.name(nm) + " : "
		}
		head += dc.expr(child(n, 1))
		for _, cell := range dc.cells(child(n, 2)) {
			label := useParams[dc.osCode[cell[0]]]
			if label == "" {
				label = dc.name(cell[0])
			}
			switch dc.boolLit(cell[1]) {
			case "true":
				head += " with " + label
			case "false":
				head += " without " + label
			default:
				head += " " + label + " " + dc.operand(cell[1])
			}
		}
		return line(head)

	case 'q': // local: [name chain]
		return line("local " + strings.Join(dc.argChain(child(n, 0)), ", "))

	case 'R':
		return line(dc.errorClause(n))

	case 't': // with timeout: [body, seconds]
		secs := dc.operand(child(n, 1))
		unit := " seconds"
		if secs == "1" {
			unit = " second"
		}
		out := []string{dc.hdr(indent+"with timeout of "+secs+unit, child(n, 0))}
		out = append(out, dc.body(child(n, 0), depth+1)...)
		return append(out, indent+"end timeout")

	case 'w': // using terms from: [body, application]
		out := []string{dc.hdr(indent+"using terms from "+dc.expr(child(n, 1)), child(n, 0))}
		if app := dc.appOf(child(n, 1)); app != "" {
			dc.scopes = append(dc.scopes, app)
			defer func() { dc.scopes = dc.scopes[:len(dc.scopes)-1] }()
		}
		out = append(out, dc.body(child(n, 0), depth+1)...)
		return append(out, indent+"end using terms from")

	case 'u': // with transaction: [body, session?]
		head := "with transaction"
		if sess := child(n, 1); sess != 0 {
			head += " " + dc.expr(sess)
		}
		out := []string{dc.hdr(indent+head, child(n, 0))}
		out = append(out, dc.body(child(n, 0), depth+1)...)
		return append(out, indent+"end transaction")

	case 'h': // script object: [name, body]
		head := "script"
		if nm := child0(n); dc.resolvable(nm) {
			head += " " + dc.name(nm)
		}
		out := []string{dc.hdr(indent+head, child(n, 1))}
		out = append(out, dc.body(child(n, 1), depth+1)...)
		return append(out, indent+"end script")

	case 'P': // considering/ignoring: [body, considered attrs, ignored attrs]
		var head []string
		if c := dc.argChain(child(n, 1)); len(c) > 0 {
			head = append(head, "considering "+andList(c))
		}
		if ig := dc.argChain(child(n, 2)); len(ig) > 0 {
			if len(head) > 0 {
				head = append(head, "but ignoring "+andList(ig))
			} else {
				head = append(head, "ignoring "+andList(ig))
			}
		}
		if len(head) == 0 {
			head = []string{"considering"}
		}
		end := "end considering"
		if strings.HasPrefix(head[0], "ignoring") {
			end = "end ignoring"
		}
		out := []string{dc.hdr(indent+strings.Join(head, " "), child(n, 0))}
		out = append(out, dc.body(child(n, 0), depth+1)...)
		return append(out, indent+end)

	case 'r': // set: [rhs, lhs]
		if len(n.children) < 2 {
			return line("-- set (incomplete)")
		}
		return line("set " + dc.expr(n.children[1]) + " to " + dc.stmtExpr(n.children[0]))

	case 's': // copy: [src, dst]
		if len(n.children) < 2 {
			return line("-- copy (incomplete)")
		}
		return line("copy " + dc.operand(n.children[0]) + " to " + dc.expr(n.children[1]))

	case 'L': // return
		if ch := child(n, 0); ch != 0 {
			return line("return " + dc.stmtExpr(ch))
		}
		return line("return")

	case 'S':
		return line("exit repeat")

	case 'Z':
		return dc.ifStmt(n, depth)

	case 'O': // tell: [body, target]
		return dc.tellStmt(n, depth)

	case 'Q':
		return dc.tryStmt(n, depth)

	case 'T', 'U', 'V', 'W', 'X', 'Y':
		return dc.repeatStmt(n, depth)

	case 'I':
		dc.curCall = id
		return line(dc.callExpr(n))

	case 'k':
		return dc.block(id, depth)

	default:
		return line(dc.stmtExpr(id))
	}
}

// stmtBreak reports whether statement wrapper id follows a `¬` break, as in
// `tell x to ¬` or `if c then ¬`.
func (dc *decompState) stmtBreak(id int16) bool {
	n, ok := dc.nodes[id]
	return ok && n.typ == 'l' && n.flags&flagContinuation != 0
}

// isContinuedExpr reports whether statement 'l' node n, which has no comment,
// holds an expression that starts after `¬` line breaks.
func (dc *decompState) isContinuedExpr(n nodeRec) bool {
	c, ok := dc.nodes[child0(n)]
	return ok && n.flags&flagContinuation != 0 && c.typ == 'l' && c.flags&flagContinuation != 0 && child(n, 1) <= 0
}

// comment returns the formatted source comment carried by 'l' node n.
func (dc *decompState) comment(n nodeRec) string {
	kind := n.flags & commentKindMask
	c, ok := dc.comments[child(n, 1)]
	if u, wrapped := dc.wrappedText(child(n, 2)); wrapped && child(n, 2) != 0 {
		c, ok = u, true // UTF-16 copy; the 0x0c text is Mac Roman
	}
	if !ok {
		if n.flags&flagContinuation != 0 {
			return "" // a continued statement, not a comment
		}
		switch {
		case kind == commentHash || kind == commentHeaderHash:
			return "#" // empty # comment
		case kind == commentLine || (kind == commentHeader && child(n, 0) != 0):
			return "--" // empty -- comment
		}
		return ""
	}
	c = strings.ReplaceAll(strings.ReplaceAll(c, "\r\n", "\n"), "\r", "\n")
	switch kind {
	case commentHash, commentHeaderHash:
		return "#" + c
	case commentBlock, commentHeaderBlk:
		return "(*" + c + "*)"
	}
	return "--" + c
}

// hdr appends to a compound statement's header line the comment that the
// compiler attached to the first line of its body: a comment-only line, or
// one flagged as preceding its statement.
func (dc *decompState) hdr(header string, bodyID int16) string {
	first := bodyID
	if n, ok := dc.nodes[bodyID]; ok && n.typ == 'k' {
		cell, ok := dc.paramMap[child(n, 0)]
		if !ok || len(cell) != 2 {
			return header
		}
		first = cell[0]
	}
	n, ok := dc.nodes[first]
	if !ok || n.typ != 'l' {
		return header
	}
	c := dc.comment(n)
	if k := n.flags & commentKindMask; c == "" || (k != commentHeader && k != commentHeaderBlk && k != commentHeaderHash) {
		return header
	}
	if dc.hoisted == nil {
		dc.hoisted = map[int16]bool{}
	}
	dc.hoisted[first] = true
	return header + " " + c
}

// body renders a statement or block child at the given depth.
func (dc *decompState) body(id int16, depth int) []string {
	if id <= 0 {
		return nil
	}
	return dc.block(id, depth)
}

// ifStmt renders 'Z': [cond, then, else-if chain, else]. The else-if chain is
// a cons list alternating condition and then-branch.
func (dc *decompState) ifStmt(n nodeRec, depth int) []string {
	indent := strings.Repeat("\t", depth)
	if len(n.children) < 2 {
		return []string{indent + "-- if (incomplete)"}
	}
	if n.flags&flagAltSyntax != 0 {
		brk := dc.stmtBreak(n.children[1])
		if brk {
			dc.contDepth++
		}
		body := dc.stmt(n.children[1], 0)
		if brk {
			dc.contDepth--
		}
		then := " then "
		if brk {
			then = " then" + dc.contMark()
		}
		if len(body) == 1 {
			return []string{indent + "if " + dc.expr(n.children[0]) + then + body[0]}
		}
		if bn, ok := dc.nodes[n.children[1]]; ok && bn.typ != 'k' && len(body) > 0 {
			// `if c then repeat …`: a compound statement follows `then`.
			if brk { // `if c then ¬` then the compound statement one level in
				return append([]string{indent + "if " + dc.expr(n.children[0]) + " then ¬"}, dc.stmt(n.children[1], depth+1)...)
			}
			body = dc.stmt(n.children[1], depth)
			body[0] = indent + "if " + dc.expr(n.children[0]) + then + strings.TrimLeft(body[0], "\t")
			return body
		}
	}
	lines := []string{dc.hdr(indent+"if "+dc.expr(n.children[0])+" then", n.children[1])}
	lines = append(lines, dc.body(n.children[1], depth+1)...)

	// Else-if chain.
	ref := child(n, 2)
	seen := make(map[int16]bool)
	for ref > 0 && !seen[ref] {
		seen[ref] = true
		condCell, ok := dc.paramMap[ref]
		if !ok || len(condCell) != 2 {
			break
		}
		thenCell, ok := dc.paramMap[condCell[1]]
		if !ok || len(thenCell) != 2 {
			break
		}
		lines = append(lines, dc.hdr(indent+"else if "+dc.expr(condCell[0])+" then", thenCell[0]))
		lines = append(lines, dc.body(thenCell[0], depth+1)...)
		ref = thenCell[1]
	}

	if els := child(n, 3); els != 0 {
		lines = append(lines, dc.hdr(indent+"else", els))
		lines = append(lines, dc.body(els, depth+1)...)
	}
	return append(lines, indent+"end if")
}

// repeatStmt renders every repeat form. Children always start with the body:
//
//	T [body]                      repeat
//	U [body, count]               repeat N times
//	V [body, cond]                repeat while
//	W [body, cond]                repeat until
//	X [body, var, list]           repeat with var in list
//	Y [body, var, from, to, by?]  repeat with var from … to … by …
func (dc *decompState) repeatStmt(n nodeRec, depth int) []string {
	indent := strings.Repeat("\t", depth)
	header := "repeat"
	switch n.typ {
	case 'U':
		header += " " + dc.expr(child(n, 1)) + " times"
	case 'V':
		header += " while " + dc.expr(child(n, 1))
	case 'W':
		header += " until " + dc.expr(child(n, 1))
	case 'X':
		if len(n.children) >= 3 {
			header += " with " + dc.name(n.children[1]) + " in " + dc.expr(n.children[2])
		}
	case 'Y':
		if len(n.children) >= 4 {
			header += " with " + dc.name(n.children[1]) + " from " + dc.expr(n.children[2]) + " to " + dc.expr(n.children[3])
			if by := child(n, 4); by != 0 {
				header += " by " + dc.expr(by)
			}
		}
	}
	lines := []string{dc.hdr(indent+header, child(n, 0))}
	if n.typ == 'X' || n.typ == 'Y' {
		dc.repeatWith++
		defer func() { dc.repeatWith-- }()
	}
	lines = append(lines, dc.body(child(n, 0), depth+1)...)
	return append(lines, indent+"end repeat")
}

// stmtExpr renders an expression that a statement consists of or assigns.
func (dc *decompState) stmtExpr(id int16) string {
	s := dc.expr(id)
	n, ok := dc.nodes[id]
	if !ok || dc.repeatWith == 0 {
		return s
	}
	if n.typ == '6' || n.typ == 'n' && n.flags&(flagPossessive|flagAltSyntax) != 0 && dc.isInterleavedCall(child0(n)) {
		return "(" + s + ")" // (x's foo:y), (items of x whose …)
	}
	return s
}
