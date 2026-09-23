package scpt

import (
	"fmt"
	"strings"
)

// DecompileBytecode rebuilds source from bytecode alone. It is used for
// run-only scripts; for others it serves to validate the bytecode path.
func DecompileBytecode(f *File) (string, error) {
	dc := f.dc
	if dc == nil || dc.objects == nil {
		return "", fmt.Errorf("no bytecode")
	}
	root, ok := dc.value(0).(*fasBlock)
	if !ok || len(root.items) == 0 {
		return "", fmt.Errorf("no script object")
	}
	b := newBCBuilder(dc)
	items, err := b.scriptItems(root.items[len(root.items)-1], true)
	if err != nil {
		return "", err
	}
	out := b.out
	out.wraps[0] = []int16{-1, b.node('k', b.cons(items))}
	return decompileWith(out, nil), nil
}

// scriptItems decompiles a script object's table
// [_, [names…], entries…] into top-level items.
func (b *bcBuilder) scriptItems(v fasValue, top bool) ([]int16, error) {
	table, ok := v.([]fasValue)
	if ok && len(table) == 0 {
		return nil, nil // an empty script object
	}
	if !ok || len(table) < 2 {
		return nil, fmt.Errorf("malformed script table: %s", b.src.fasString(v)[:min(80, len(b.src.fasString(v)))])
	}
	names, _ := table[1].([]fasValue)
	scopeNames := make([]string, len(names))
	for i, n := range names {
		scopeNames[i] = b.src.fasString(n)
	}
	b.scopes = append(b.scopes, scopeNames)
	defer func() { b.scopes = b.scopes[:len(b.scopes)-1] }()
	var items, uses, props []int16
	var runBody []int16
	// The compiler puts the implicit run handler after every other handler,
	// so a run handler anywhere else was written out: on run.
	lastHandler := -1
	for i, entry := range table[2:] {
		if e, ok := entry.(*fasBlock); ok && (e.kind == 16 || e.kind == 17) {
			lastHandler = i
		}
	}
	globalNames := map[string]bool{}
	if top {
		globalNames = b.globalAccesses(table[2:])
	}
	for i, entry := range table[2:] {
		var name fasValue
		if i < len(names) {
			name = names[i]
		}
		switch e := entry.(type) {
		case *fasBlock:
			switch e.kind {
			case 16, 17:
				h, ok := b.src.handlerFrom(e)
				if !ok {
					continue
				}
				if c, ok := h.name.(fasCode); ok && c.code == "aevtoapp" && top && len(h.params) == 0 && !h.hasPattern && i == lastHandler {
					// The implicit run handler: its body is the script's
					// top-level code.
					body, err := b.handlerBody(h)
					if err != nil {
						return nil, err
					}
					runBody = body
					continue
				}
				def, err := b.handlerDef(h, e)
				if err != nil {
					return nil, err
				}
				items = append(items, def, b.node('l'))
				continue
			case 15:
				// A script object: {name, _, _, table}.
				if len(e.items) >= 4 {
					inner, err := b.scriptItems(e.items[3], false)
					if err != nil {
						return nil, err
					}
					obj := b.node('h', b.keyRef(e.items[0]), b.node('k', b.cons(inner)))
					items = append(items, b.node('l', obj), b.node('l'))
					continue
				}
			}
		}
		if c, ok := name.(fasCode); ok && c.code == "pimr" {
			uses = b.useStatements(entry)
			continue
		}
		if c, ok := name.(fasCode); ok && c.code == "pare" && entry == nil {
			continue // no explicit parent
		}
		// `use O : script "x"` stores O as a property referring to the
		// used script.
		if blk, ok := entry.(*fasBlock); ok && blk.kind == 20 && len(blk.items) == 1 && name != nil {
			if use, ok := b.useTargets[b.useKey(blk.items[0])]; ok {
				n := b.out.nodes[use]
				n.children[0] = b.keyRef(name)
				b.out.nodes[use] = n
				continue
			}
		}
		// A top-level variable's saved value, not a property.
		if n, ok := name.(string); ok && top && globalNames[strings.ToLower(n)] {
			continue
		}
		// Anything else is a property with its initial value.
		if name != nil {
			prop := b.node('j', b.keyRef(name), b.literal(entry))
			props = append(props, b.node('l', prop))
		}
	}
	items = append(props, items...)
	if n := len(items); !top && n > 0 && len(b.out.nodes[items[n-1]].children) == 0 {
		items = items[:n-1] // no blank line before `end script`
	}
	if top {
		if globals := b.globals(table[2:]); len(globals) > 0 {
			items = append([]int16{b.node('l', b.node('p', b.cons(globals)))}, items...)
		}
	}
	if len(runBody) > 0 {
		items = append(items, runBody...)
	}
	if len(uses) > 0 {
		if !b.usesAdditions(uses) && b.callsAdditions() {
			// Standard Additions commands compile only before a `use` that
			// leaves them out, so these came last.
			items = append(append(items, b.node('l')), uses...)
		} else {
			items = append(append(uses, b.node('l')), items...)
		}
	}
	return items, nil
}

// usesAdditions reports whether use statements include `use scripting additions`.
func (b *bcBuilder) usesAdditions(uses []int16) bool {
	for _, u := range uses {
		x := b.out.nodes[child0(b.out.nodes[u])]
		if t, ok := b.out.nodes[child(x, 1)]; ok && t.typ == '2' && b.out.osCode[child0(t)] == "osax" {
			return true
		}
	}
	return false
}

// callsAdditions reports whether the script calls a Standard Additions command.
func (b *bcBuilder) callsAdditions() bool {
	for _, h := range b.src.allHandlers() {
		for _, lit := range h.literals {
			if c, ok := lit.(fasCode); ok && c.kind == osKindEvent && dicts[""].events[c.code] != "" {
				return true
			}
		}
	}
	return false
}

// globalAccesses returns the (lowercase) names that any handler reaches
// with PushGlobal or PopGlobal.
func (b *bcBuilder) globalAccesses(entries []fasValue) map[string]bool {
	out := map[string]bool{}
	for _, entry := range entries {
		h, ok := b.src.handlerFrom(entry)
		if !ok {
			continue
		}
		prog, _ := disassemble(h.code)
		for _, in := range prog {
			if strings.HasPrefix(in.name, "PushGlobal") || strings.HasPrefix(in.name, "PopGlobal") {
				if idx := in.args[len(in.args)-1]; idx >= 0 && idx < len(h.literals) {
					if name, ok := h.literals[idx].(string); ok {
						out[strings.ToLower(name)] = true
					}
				}
			}
		}
	}
	return out
}

// globals returns the variables that handlers reach as globals, which the
// source must have declared (`global x`): top-level code sees its variables
// as globals without one, but a handler only with a declaration.
func (b *bcBuilder) globals(entries []fasValue) []int16 {
	seen := map[string]bool{"result": true, "applescript": true}
	var out []int16
	for _, entry := range entries {
		h, ok := b.src.handlerFrom(entry)
		if !ok {
			continue
		}
		if c, ok := h.name.(fasCode); ok && c.code == "aevtoapp" {
			continue // a run handler's variables are globals anyway
		}
		prog, _ := disassemble(h.code)
		for _, in := range prog {
			if !strings.HasPrefix(in.name, "PushGlobal") && !strings.HasPrefix(in.name, "PopGlobal") {
				continue
			}
			idx := in.args[len(in.args)-1]
			if idx < 0 || idx >= len(h.literals) {
				continue
			}
			name, ok := h.literals[idx].(string)
			if !ok || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			out = append(out, b.node('o', b.nameRef(name)))
		}
	}
	return out
}

// handlerBody decompiles a handler's code into statements.
func (b *bcBuilder) handlerBody(h handlerCode) ([]int16, error) {
	prog, err := disassemble(h.code)
	if err != nil {
		return nil, err
	}
	bh := &bcHandler{b: b, h: h, prog: prog, index: map[int]int{}}
	for i, in := range prog {
		bh.index[in.off] = i
	}
	bh.index[len(h.code)] = len(prog)
	stmts, err := bh.stmts(0, len(h.code))
	if n := len(prog); err == nil && n >= 3 && prog[n-1].name == "Return" && bh.blankTail(n-1) {
		// A blank line (or comment) after the last statement compiles to
		// this tail instead of a bare Return.
		stmts = append(stmts, b.node('l'))
	}
	return stmts, err
}

// handlerDef builds an 'i' handler definition node.
func (b *bcBuilder) handlerDef(h handlerCode, blk *fasBlock) (int16, error) {
	stmts, err := b.handlerBody(h)
	if err != nil {
		return 0, err
	}
	body := b.node('k', b.cons(stmts))
	var params []int16
	for _, p := range h.params {
		if name, ok := p.(string); ok {
			params = append(params, b.node('o', b.nameRef(name)))
		} else {
			params = append(params, b.literal(p))
		}
	}
	nameRef := b.keyRef(h.name)
	var sig int16
	if _, event := h.name.(fasCode); event {
		// Event handler: `on open theFiles` — the direct parameter first.
		direct := b.ref()
		if len(params) > 0 {
			direct = params[0]
		}
		if h.hasPattern {
			var names []int16
			for _, p := range h.pattern {
				names = append(names, b.node('o', b.nameRef(p)))
			}
			direct = b.node('J', b.cons(names))
		}
		sig = b.node('I', nameRef, direct, b.labeledParams(blk))
	} else {
		plist := b.ref()
		if len(params) > 0 {
			plist = b.cons(params)
		}
		sig = b.node('I', nameRef, plist, b.labeledParams(blk))
	}
	return b.node('i', sig, body), nil
}

// labeledParams builds the given/preposition cells from a handler block's
// labeled-parameter binding (label → variable name).
func (b *bcBuilder) labeledParams(blk *fasBlock) int16 {
	bind, ok := blk.items[3].(fasBinding)
	if !ok || len(bind.keys) == 0 {
		return b.ref()
	}
	var keys, values []int16
	for i := range bind.keys {
		keys = append(keys, b.keyRef(bind.keys[i]))
		if name, ok := bind.values[i].(string); ok {
			values = append(values, b.node('o', b.nameRef(name)))
		} else {
			values = append(values, b.literal(bind.values[i]))
		}
	}
	return b.cells(keys, values)
}

// parentName resolves a variable of an enclosing script object: level 1 is
// the script defining the current handler.
func (bh *bcHandler) parentName(level, index int) string {
	if level >= 1 && level <= len(bh.b.scopes) {
		names := bh.b.scopes[len(bh.b.scopes)-level]
		if index >= 0 && index < len(names) {
			return names[index]
		}
	}
	return fmt.Sprintf("parent%d_%d", level, index)
}

// specBlock builds a precompiled reference literal. Block kinds mirror the
// MakeObjectAlias forms offset by 21: 20 a reference to, 21 property,
// 22 every, 23 some, 24 index, 25 key, 27 range, 30 beginning, 31 end,
// 32 middle. Items start with the container (nil for none).
func (b *bcBuilder) specBlock(x *fasBlock) (int16, bool) {
	it := x.items
	withContainer := func(part int16) int16 {
		if len(it) == 0 || it[0] == nil {
			return part
		}
		container := it[0]
		if c, ok := container.(*fasBlock); ok && c.kind == 15 {
			return part // the script itself: use scripting additions
		}
		if c, ok := container.(*fasBlock); ok && c.kind == 20 && len(c.items) == 1 {
			container = c.items[0] // only the outermost reference says "a reference to"
		}
		return b.node('n', part, b.literal(container))
	}
	key := func(v fasValue) int16 { return b.keyRef(v) }
	switch {
	case x.kind == 20 && len(it) == 1:
		return b.node('N', b.literal(it[0])), true
	case x.kind == 21 && len(it) == 2:
		part := b.node('1', key(it[1]))
		if _, ident := it[1].(string); ident {
			part = b.node('o', key(it[1]))
		}
		if c, ok := it[0].(fasCode); ok && c.code == "cura" {
			id := b.node('n', part, b.literal(it[0]))
			b.out.nodes[id] = nodeRec{typ: 'n', flags: flagPossessive, children: b.out.nodes[id].children}
			return id, true
		}
		return withContainer(part), true
	case x.kind == 22 && len(it) == 2:
		return withContainer(b.node('2', key(it[1]))), true
	case x.kind == 23 && len(it) == 2:
		return withContainer(b.node('3', key(it[1]))), true
	case x.kind == 24 && len(it) == 3:
		return withContainer(b.node('4', key(it[1]), b.literal(it[2]))), true
	case x.kind == 25 && len(it) >= 3:
		form := fasValue(fasCode{kind: osKindCode, code: "name"})
		if len(it) >= 4 {
			form = it[3]
		}
		return withContainer(b.node('5', key(it[1]), b.literal(it[2]), key(form))), true
	case x.kind == 27 && len(it) == 4:
		n := b.node('7', key(it[1]), b.literal(it[2]), b.literal(it[3]))
		b.out.nodes[n] = nodeRec{typ: '7', flags: flagThe, children: b.out.nodes[n].children}
		return withContainer(n), true
	case x.kind == 30 && len(it) == 1:
		return withContainer(b.node(':')), true
	case x.kind == 31 && len(it) == 1:
		return withContainer(b.node(';')), true
	case x.kind == 32 && len(it) == 1:
		return withContainer(b.node('<')), true
	case x.kind == 123 && len(it) == 3:
		// An optional/typed handler parameter: name [as class] [: default].
		v := b.node('o', key(it[0]))
		if it[1] != nil {
			v = b.node('c', v, b.literal(it[1]))
		}
		if it[2] != nil {
			return b.node('~', v, b.literal(it[2])), true
		}
		return v, true
	}
	return 0, false
}

// useKey identifies a use target regardless of its container, which is the
// script itself in the use statement and absent in the named property.
func (b *bcBuilder) useKey(v fasValue) string {
	if blk, ok := v.(*fasBlock); ok && len(blk.items) > 0 {
		return fmt.Sprintf("%d:%s", blk.kind, b.src.fasString(blk.items[1:]))
	}
	return b.src.fasString(v)
}

// useStatements builds `use …` statements from the required-imports
// property: block4{count, block0{records…}}.
func (b *bcBuilder) useStatements(v fasValue) []int16 {
	outer, ok := v.(*fasBlock)
	if !ok || len(outer.items) < 2 {
		return nil
	}
	inner, ok := outer.items[1].(*fasBlock)
	if !ok {
		return nil
	}
	var out []int16
	for _, rec := range inner.items {
		bind, ok := rec.(fasBinding)
		if !ok {
			continue
		}
		var target int16
		var targetValue fasValue
		var keys, values []int16
		for i, k := range bind.keys {
			kc, _ := k.(fasCode)
			switch {
			case kc.code == "cobj" || b.src.fasString(k) == "item":
				val := bind.values[i]
				if blk, ok := val.(*fasBlock); ok && blk.kind == 20 && len(blk.items) == 1 {
					val = blk.items[0] // the reference, not `a reference to`
				}
				target, targetValue = b.literal(val), val
				if t := b.out.nodes[target]; t.typ == '2' {
					t.flags = flagAltSyntax // use scripting additions (plural)
					b.out.nodes[target] = t
				}
			case kc.code == "vers" || b.src.fasString(k) == "version":
				keys = append(keys, b.termRef(fasCode{kind: osKindCode, code: "minv"}))
				values = append(values, b.literal(bind.values[i]))
			}
		}
		if target == 0 {
			target = b.node('1', b.termRef(fasCode{kind: osKindCode, code: "ascr"}))
		}
		labels := b.ref()
		if len(keys) > 0 {
			labels = b.cells(keys, values)
		}
		use := b.node('x', b.ref(), target, labels)
		if b.useTargets == nil {
			b.useTargets = map[string]int16{}
		}
		b.useTargets[b.useKey(targetValue)] = use
		out = append(out, b.node('l', use))
	}
	return out
}

// scriptInit decompiles a script object's init handler into its body:
// assignments to the object's own variables become properties.
func (b *bcBuilder) scriptInit(blk *fasBlock) ([]int16, error) {
	h, ok := b.src.handlerFrom(blk)
	if !ok {
		return nil, fmt.Errorf("malformed script init")
	}
	own := map[string]bool{}
	for _, v := range h.vars {
		own[v] = true
	}
	stmts, err := b.handlerBody(h)
	if err != nil {
		return nil, err
	}
	var props, rest []int16
	for _, st := range stmts {
		n := b.out.nodes[child0(b.out.nodes[st])]
		if n.typ == 'r' && len(n.children) == 2 {
			if t := b.out.nodes[n.children[1]]; t.typ == 'o' && own[b.out.refName[child0(t)]] {
				props = append(props, b.node('l', b.node('j', child0(t), n.children[0])))
				continue
			}
		}
		if n.typ == 'j' { // property parent : …, in declaration order
			props = append(props, st)
			continue
		}
		rest = append(rest, st)
	}
	return append(props, rest...), nil
}
