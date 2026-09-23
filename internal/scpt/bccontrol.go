package scpt

// body decompiles [lo, hi) into a 'k' block node.
func (bh *bcHandler) body(lo, hi int) (int16, error) {
	stmts, err := bh.stmts(lo, hi)
	if err != nil {
		return 0, err
	}
	return bh.b.node('k', bh.b.cons(stmts)), nil
}

// ifStmt decompiles `cond TestIf else … [Jump end] …`.
func (bh *bcHandler) ifStmt(pc int, stack *[]stackVal) (int16, int, error) {
	b := bh.b
	in := bh.prog[pc]
	s := *stack
	if len(s) == 0 {
		return 0, 0, errUnsupported{in, "missing condition"}
	}
	cond := s[len(s)-1]
	*stack = s[:len(s)-1]
	elseAt := in.jumpTarget(in.args[0])
	thenEnd, end := elseAt, elseAt
	// A Jump just before the else branch skips it at the end of the then branch.
	if j, ok := bh.index[elseAt]; ok && j > 0 {
		if prev := bh.prog[j-1]; prev.name == "Jump" && prev.off > in.off && prev.jumpTarget(prev.args[0]) > elseAt {
			thenEnd = prev.off
			end = prev.jumpTarget(prev.args[0])
		}
	}
	then, err := bh.body(in.off+in.size, thenEnd)
	if err != nil {
		return 0, 0, err
	}
	children := []int16{cond.id, then, b.ref(), b.ref()}
	if end > elseAt {
		// An if statement as the whole else branch prints as `else if`.
		els, err := bh.stmts(elseAt, end)
		if err != nil {
			return 0, 0, err
		}
		if chain, ok := bh.elseIfChain(els); ok {
			children[2], children[3] = chain[0], chain[1]
		} else if len(els) > 0 {
			children[3] = b.node('k', b.cons(els))
		}
	}
	resume := len(bh.prog)
	if j, ok := bh.index[end]; ok {
		resume = j
	}
	return b.node('Z', children...), resume, nil
}

// elseIfChain folds a lone nested if statement into else-if form: it
// returns the chain ([cond, [then, next]] cells) and the final else.
func (bh *bcHandler) elseIfChain(els []int16) ([2]int16, bool) {
	b := bh.b
	if len(els) != 1 {
		return [2]int16{}, false
	}
	inner := b.out.nodes[child0(b.out.nodes[els[0]])]
	if inner.typ != 'Z' || len(inner.children) != 4 {
		return [2]int16{}, false
	}
	// Chain cells: [cond, cell[then, rest]].
	rest := inner.children[2]
	if rest <= 0 {
		rest = b.ref()
	}
	thenCell := b.nextID
	b.nextID++
	b.out.paramMap[thenCell] = []int16{inner.children[1], rest}
	condCell := b.nextID
	b.nextID++
	b.out.paramMap[condCell] = []int16{inner.children[0], thenCell}
	return [2]int16{condCell, inner.children[3]}, true
}

// tellStmt decompiles `target Tell n … EndTell`. When the body leaves a
// value on the stack, the tell is an expression.
func (bh *bcHandler) tellStmt(pc int, stack *[]stackVal) (int16, int, bool, error) {
	b := bh.b
	in := bh.prog[pc]
	s := *stack
	if len(s) == 0 {
		return 0, 0, false, errUnsupported{in, "missing tell target"}
	}
	target := s[len(s)-1]
	*stack = s[:len(s)-1]
	j, err := bh.matching(pc, "Tell", "EndTell")
	if err != nil {
		return 0, 0, false, err
	}
	stmts, rest, err := bh.run(in.off+in.size, bh.prog[j].off, nil)
	if err != nil {
		return 0, 0, false, err
	}
	var value int16
	for _, v := range rest {
		if !v.undefined && !v.assigned && !v.it {
			value = v.id
			if len(stmts) > 0 || j+1 >= len(bh.prog) || !consumesValue(bh.prog[j+1].name) {
				value = b.stmtValue(v)
			}
		}
	}
	if value != 0 && len(stmts) == 0 && j+1 < len(bh.prog) && consumesValue(bh.prog[j+1].name) {
		if bh.isCurrentApp(target.id) {
			// The compiler wraps some Standard Additions calls in scripts
			// that use frameworks in a tell to current application of its
			// own; the source has only the call.
			return value, j + 1, true, nil
		}
		id := b.node('O', value, target.id)
		b.out.nodes[id] = nodeRec{typ: 'O', flags: flagAltSyntax, children: []int16{value, target.id}}
		return id, j + 1, true, nil
	}
	if value != 0 {
		stmts = append(stmts, b.node('l', value))
	}
	stmts = bh.foldDestructuring(stmts)
	if j-2 > pc && bh.blankTail(j) {
		stmts = append(stmts, b.node('l')) // a blank line before end tell
	}
	return b.node('O', b.node('k', b.cons(stmts)), target.id), j + 1, false, nil
}

// repeatStmt decompiles `LinkRepeat end, setup, Repeat…, body, Dup StoreResult Jump back`.
func (bh *bcHandler) repeatStmt(pc int) (int16, int, error) {
	b := bh.b
	in := bh.prog[pc]
	end := in.jumpTarget(in.args[0])
	endIdx, ok := bh.index[end]
	if !ok {
		endIdx = len(bh.prog)
	}
	// The back jump closes the loop; the body runs up to its Dup StoreResult.
	back := -1
	for j := endIdx - 1; j > pc; j-- {
		if bh.prog[j].name == "Jump" && bh.prog[j].jumpTarget(bh.prog[j].args[0]) < bh.prog[j].off {
			back = j
			break
		}
	}
	if back < 0 {
		return 0, 0, errUnsupported{in, "loop without back jump"}
	}
	loopTop := bh.index[bh.prog[back].jumpTarget(bh.prog[back].args[0])]
	bodyEnd := bh.prog[back].off
	if back >= 2 && bh.prog[back-1].name == "StoreResult" && bh.prog[back-2].name == "Dup" {
		bodyEnd = bh.prog[back-2].off
	}
	top := bh.prog[loopTop]
	var stmt int16
	switch top.name {
	case "Pop": // plain repeat: LinkRepeat PushUndefined [Pop …]
		body, err := bh.body(bh.prog[loopTop+1].off, bodyEnd)
		if err != nil {
			return 0, 0, err
		}
		stmt = b.node('T', body)
	case "RepeatNTimes", "RepeatInRange", "RepeatInCollection":
		// Setup pushes the loop parameters, then an undefined result slot.
		setup, err := bh.stackAt(in.off+in.size, top.off)
		if err != nil {
			return 0, 0, err
		}
		body, err := bh.body(top.off+top.size, bodyEnd)
		if err != nil {
			return 0, 0, err
		}
		switch top.name {
		case "RepeatNTimes":
			if len(setup) < 1 {
				return 0, 0, errUnsupported{top, "missing count"}
			}
			stmt = b.node('U', body, setup[0].id)
		case "RepeatInRange":
			if len(setup) < 3 {
				return 0, 0, errUnsupported{top, "missing range"}
			}
			v := bh.varRef(top.args[0])
			children := []int16{body, v, setup[0].id, setup[1].id}
			if setup[2].count != 1 {
				children = append(children, setup[2].id)
			}
			stmt = b.node('Y', children...)
		case "RepeatInCollection":
			if len(setup) < 1 {
				return 0, 0, errUnsupported{top, "missing collection"}
			}
			v := bh.varRef(top.args[0])
			stmt = b.node('X', body, v, setup[0].id)
		}
	default:
		// repeat while/until: the condition is evaluated at the loop top.
		cond := -1
		for j := loopTop; j < back; j++ {
			if n := bh.prog[j].name; n == "RepeatWhile" || n == "RepeatUntil" {
				cond = j
				break
			}
		}
		if cond < 0 {
			return 0, 0, errUnsupported{top, "unknown loop form"}
		}
		c, err := bh.expr(top.off, bh.prog[cond].off)
		if err != nil {
			return 0, 0, err
		}
		body, err := bh.body(bh.prog[cond].off+bh.prog[cond].size, bodyEnd)
		if err != nil {
			return 0, 0, err
		}
		typ := byte('V')
		if bh.prog[cond].name == "RepeatUntil" {
			typ = 'W'
		}
		stmt = b.node(typ, body, c)
	}
	// Skip the result bookkeeping after the loop.
	resume := endIdx
	return stmt, resume, nil
}

// stackAt simulates [lo, hi) (loop setup code) and returns the pushed
// values, without the trailing undefined result slot.
func (bh *bcHandler) stackAt(lo, hi int) ([]stackVal, error) {
	_, stack, err := bh.run(lo, hi, nil)
	if err != nil {
		return nil, err
	}
	if n := len(stack); n > 0 && stack[n-1].undefined {
		stack = stack[:n-1]
	}
	return stack, nil
}

// tryStmt decompiles `ErrorHandler handler, body, EndErrorHandler end,
// handler: HandleError msg num, …`.
func (bh *bcHandler) tryStmt(pc int) (int16, int, error) {
	b := bh.b
	in := bh.prog[pc]
	handlerAt := in.jumpTarget(in.args[0])
	hIdx, ok := bh.index[handlerAt]
	if !ok || hIdx == 0 || bh.prog[hIdx-1].name != "EndErrorHandler" {
		return 0, 0, errUnsupported{in, "try without EndErrorHandler"}
	}
	endTry := bh.prog[hIdx-1]
	end := endTry.jumpTarget(endTry.args[0])
	body, err := bh.body(in.off+in.size, endTry.off)
	if err != nil {
		return 0, 0, err
	}
	he := bh.prog[hIdx]
	spec := b.node('R', b.ref(), b.ref(), b.ref())
	handlerStart := handlerAt
	if he.name == "HandleError" {
		handlerStart = he.off + he.size
		// Operands are literal indices: the variable receiving the message,
		// and a binding of labels (number, from, …) to variables.
		var keys, values []int16
		msg := b.ref()
		if len(he.args) >= 1 {
			switch v := bh.lit(he.args[0]).(type) {
			case string:
				msg = b.node('o', b.nameRef(v))
			case nil:
			default:
				msg = b.literal(v) // on error "x": a message to match
			}
		}
		if len(he.args) >= 2 {
			if bind, ok := bh.lit(he.args[1]).(fasBinding); ok {
				for i := range bind.keys {
					keys = append(keys, b.keyRef(bind.keys[i]))
					if name, ok := bind.values[i].(string); ok {
						values = append(values, b.node('o', b.nameRef(name)))
					} else {
						values = append(values, b.literal(bind.values[i])) // on error number -128
					}
				}
			}
		}
		labels := b.ref()
		if len(keys) > 0 {
			labels = b.cells(keys, values)
		}
		spec = b.node('R', b.ref(), msg, labels)
	}
	handler := b.ref()
	if end > handlerStart {
		stmts, err := bh.stmts(handlerStart, end)
		if err != nil {
			return 0, 0, err
		}
		if len(stmts) > 0 {
			handler = b.node('k', b.cons(stmts))
		}
	}
	resume := len(bh.prog)
	if j, ok := bh.index[end]; ok {
		resume = j
	}
	return b.node('Q', body, spec, handler), resume, nil
}

// considerStmt decompiles `considering … Consider n … EndConsider`.
func (bh *bcHandler) considerStmt(pc int, considering, ignoring stackVal) (int16, int, error) {
	b := bh.b
	in := bh.prog[pc]
	depth := 0
	for j := pc + 1; j < len(bh.prog); j++ {
		switch bh.prog[j].name {
		case "Consider":
			depth++
		case "EndConsider":
			if depth > 0 {
				depth--
				continue
			}
			body, err := bh.body(in.off+in.size, bh.prog[j].off)
			if err != nil {
				return 0, 0, err
			}
			return b.node('P', body, bh.attrChain(considering.id), bh.attrChain(ignoring.id)), j + 1, nil
		}
	}
	return 0, 0, errUnsupported{in, "unterminated considering"}
}

// attrChain turns a list literal node of attributes into a name cons chain.
func (bh *bcHandler) attrChain(listID int16) int16 {
	b := bh.b
	n := b.out.nodes[listID]
	if n.typ != 'J' {
		return b.ref()
	}
	var names []int16
	for ref := child0(n); ref > 0; {
		cell, ok := b.out.paramMap[ref]
		if !ok || len(cell) != 2 {
			break
		}
		names = append(names, bh.keyOf(cell[0]))
		ref = cell[1]
	}
	if len(names) == 0 {
		return b.ref()
	}
	return b.cons(names)
}

// matching finds the instruction closing the block opened at pc.
func (bh *bcHandler) matching(pc int, open, close string) (int, error) {
	depth := 0
	for j := pc + 1; j < len(bh.prog); j++ {
		switch bh.prog[j].name {
		case open:
			depth++
		case close:
			if depth == 0 {
				return j, nil
			}
			depth--
		}
	}
	return 0, errUnsupported{bh.prog[pc], "unterminated " + open}
}

// foldDestructuring rewrites the compiler's expansion of
// `set {a, b} to x` — `set a to item 1 of x`, `set b to item 2 of x`, …,
// then x as a discarded expression — back into one statement.
func (bh *bcHandler) foldDestructuring(stmts []int16) []int16 {
	b := bh.b
	// itemOf returns (k, x) when stmt is `set target to item k of x`.
	itemOf := func(stmt int16) (int, int16, int16, bool) {
		set := b.out.nodes[child0(b.out.nodes[stmt])]
		if (set.typ != 'r' && set.typ != 's') || len(set.children) != 2 {
			return 0, 0, 0, false
		}
		ref := b.out.nodes[set.children[0]]
		if ref.typ != 'n' || len(ref.children) != 2 {
			return 0, 0, 0, false
		}
		idx := b.out.nodes[ref.children[0]]
		if idx.typ != '4' || len(idx.children) != 2 || b.out.osCode[idx.children[0]] != "cobj" {
			return 0, 0, 0, false
		}
		k, ok := b.out.intByRef[child0(b.out.nodes[idx.children[1]])]
		return k, ref.children[1], set.children[1], ok
	}
	// propOf returns (property ref, x, target) for `set target to prop of x`.
	propOf := func(stmt int16) (int16, int16, int16, bool) {
		set := b.out.nodes[child0(b.out.nodes[stmt])]
		if set.typ != 'r' || len(set.children) != 2 {
			return 0, 0, 0, false
		}
		ref := b.out.nodes[set.children[0]]
		if ref.typ != 'n' || len(ref.children) != 2 {
			return 0, 0, 0, false
		}
		prop := b.out.nodes[ref.children[0]]
		if prop.typ != '1' && prop.typ != 'o' {
			return 0, 0, 0, false
		}
		return child0(prop), ref.children[1], set.children[1], true
	}
	var out []int16
	for i := 0; i < len(stmts); i++ {
		if _, x, _, ok := propOf(stmts[i]); ok {
			var keys, targets []int16
			j := i
			for ; j < len(stmts); j++ {
				key, xj, t, ok := propOf(stmts[j])
				if !ok || xj != x {
					break
				}
				keys, targets = append(keys, key), append(targets, t)
			}
			if len(keys) >= 2 && j < len(stmts) && bh.isValue(stmts[j], x) {
				// set {year:y, month:m} to d
				out = append(out, b.node('l', b.node('r', x, b.node('K', b.cells(keys, targets)))))
				i = j
				continue
			}
		}
		k, x, _, ok := itemOf(stmts[i])
		if !ok || k != 1 {
			out = append(out, stmts[i])
			continue
		}
		var targets []int16
		j := i
		for ; j < len(stmts); j++ {
			kj, xj, t, ok := itemOf(stmts[j])
			if !ok || xj != x || kj != len(targets)+1 {
				break
			}
			targets = append(targets, t)
		}
		// The duplicated value itself is left over as an expression statement.
		if len(targets) < 2 || j >= len(stmts) || !bh.isValue(stmts[j], x) {
			out = append(out, stmts[i])
			continue
		}
		// copy x to {a, b} when the parts were copied.
		verb := b.out.nodes[child0(b.out.nodes[stmts[i]])].typ
		out = append(out, b.node('l', b.node(verb, x, b.node('J', b.cons(targets)))))
		i = j
	}
	return out
}
