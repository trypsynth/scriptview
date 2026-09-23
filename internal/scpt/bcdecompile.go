package scpt

import (
	"fmt"
	"strings"
)

// Bytecode decompilation: rebuild a syntax tree from each handler's
// instructions by simulating the VM's operand stack, then render it with the
// same code that renders compiled syntax trees. This is the only way to read
// run-only scripts, which carry no syntax tree.
//
// The rebuilt tree lives in a fresh decompState whose node and term tables
// are synthesized here; flags are left zero, so the output uses AppleScript's
// default spellings.

// bcBuilder synthesizes nodes and term references.
type bcBuilder struct {
	src     *decompState // the parsed file (for literal values)
	out     *decompState // the synthetic tree being built
	nextID  int16
	nextRef int16
	// scopes holds the name tables of the script objects enclosing the
	// code being decompiled, outermost first.
	scopes [][]string
}

func newBCBuilder(src *decompState) *bcBuilder {
	out := &decompState{
		nodes:     map[int16]nodeRec{},
		refName:   map[int16]string{},
		intByRef:  map[int16]int{},
		textByID:  map[int16]string{},
		paramMap:  map[int16][]int16{},
		wraps:     map[int16][]int16{},
		wrapType:  map[int16]byte{},
		appByID:   map[int16]string{},
		evCode:    map[int16]string{},
		realByID:  map[int16]float64{},
		descByID:  map[int16]descRec{},
		comments:  map[int16]string{},
		osCode:    map[int16]string{},
		osEnum:    map[int16]string{},
		piped:     map[int16]bool{},
		canonical: src.canonical,
	}
	return &bcBuilder{src: src, out: out, nextID: 1, nextRef: -1}
}

// node adds a synthetic syntax node.
func (b *bcBuilder) node(typ byte, children ...int16) int16 {
	id := b.nextID
	b.nextID++
	b.out.nodes[id] = nodeRec{typ: typ, children: children}
	return id
}

// ref allocates a synthetic negative reference.
func (b *bcBuilder) ref() int16 {
	r := b.nextRef
	b.nextRef--
	return r
}

// cons builds a cons chain of ids (for blocks, lists, parameter lists).
func (b *bcBuilder) cons(ids []int16) int16 {
	tail := b.ref() // terminator
	for i := len(ids) - 1; i >= 0; i-- {
		cell := b.nextID
		b.nextID++
		b.out.paramMap[cell] = []int16{ids[i], tail}
		tail = cell
	}
	if len(ids) == 0 {
		return tail
	}
	return tail
}

// cells builds a chain of [key, value, next] cells.
func (b *bcBuilder) cells(keys, values []int16) int16 {
	tail := b.ref()
	for i := len(keys) - 1; i >= 0; i-- {
		cell := b.nextID
		b.nextID++
		b.out.paramMap[cell] = []int16{keys[i], values[i], tail}
		tail = cell
	}
	return tail
}

// nameRef returns a reference naming an identifier.
func (b *bcBuilder) nameRef(name string) int16 {
	r := b.ref()
	b.out.refName[r] = name
	return r
}

// termRef returns a reference for a term code.
func (b *bcBuilder) termRef(c fasCode) int16 {
	r := b.ref()
	switch c.kind {
	case osKindEvent:
		b.out.evCode[r] = c.code
	case osKindConstant:
		b.out.osCode[r] = c.code
		b.out.osEnum[r] = c.enum
	default:
		b.out.osCode[r] = c.code
	}
	return r
}

// keyRef returns a reference for a literal used as a name: a term or an
// identifier (record keys, parameter labels, handler names).
func (b *bcBuilder) keyRef(v fasValue) int16 {
	switch x := v.(type) {
	case fasCode:
		return b.termRef(x)
	case string:
		return b.nameRef(x)
	}
	return b.nameRef(b.src.fasString(v))
}

// constantList returns the items of a list literal that the compiler stored
// as a value: block4{count, block0{items…}}.
func constantList(x *fasBlock) ([]fasValue, bool) {
	if x.kind != 4 || len(x.items) != 2 {
		return nil, false
	}
	n, ok := x.items[0].(int)
	if ok && n == 0 && x.items[1] == nil {
		return []fasValue{}, true
	}
	items, ok2 := x.items[1].(*fasBlock)
	if !ok || !ok2 || items.kind != 0 || len(items.items) < n {
		return nil, false
	}
	return items.items[:n], true
}

// literal builds an expression node for a literal value.
func (b *bcBuilder) literal(v fasValue) int16 {
	switch x := v.(type) {
	case int:
		r := b.ref()
		b.out.intByRef[r] = x
		return b.node('m', r)
	case float64:
		id := b.nextID
		b.nextID++
		b.out.realByID[id] = x
		return b.node('m', id)
	case bool:
		code := "fals"
		if x {
			code = "true"
		}
		return b.node('m', b.termRef(fasCode{kind: osKindConstant, enum: "boov", code: code}))
	case fasText:
		textID := b.nextID
		b.nextID++
		b.out.textByID[textID] = string(x)
		wrap := b.nextID
		b.nextID++
		b.out.wraps[wrap] = []int16{textID}
		b.out.wrapType[wrap] = wrapText
		return b.node('m', wrap)
	case fasCode:
		if x.kind == osKindCode {
			return b.node('1', b.termRef(x))
		}
		return b.node('m', b.termRef(x))
	case fasApp:
		id := b.nextID
		b.nextID++
		b.out.appByID[id] = string(x)
		return b.node('m', id)
	case fasDesc:
		id := b.nextID
		b.nextID++
		b.out.descByID[id] = descRec(x)
		return b.node('m', id)
	case string:
		return b.node('o', b.nameRef(x))
	case []fasValue:
		items := make([]int16, len(x))
		for i, it := range x {
			items[i] = b.literal(it)
		}
		return b.node('J', b.cons(items))
	case nil:
		return b.node('m', b.termRef(fasCode{kind: osKindConstant, enum: "enum", code: "msng"}))
	case *fasBlock:
		if items, ok := constantList(x); ok {
			return b.literal(items)
		}
		if id, ok := b.specBlock(x); ok {
			return id
		}
	}
	return b.node('o', b.nameRef(fmt.Sprintf("«%s»", b.src.fasString(v))))
}

// bcHandler decompiles one handler's code.
type bcHandler struct {
	b     *bcBuilder
	h     handlerCode
	prog  []instr
	index map[int]int // offset → instruction index
}

// stackVal is an operand-stack entry: an expression node, or a marker.
type stackVal struct {
	id        int16
	undefined bool // PushUndefined
	it        bool // PushIt: no explicit target/direct parameter
	assigned  bool // already stored by a set statement (Pop*/SetData peek)
	scriptDef bool // a script object definition (DefineActor)
	next      bool // PushNext marker for continue
	gotten    bool // evaluated by GetData (an explicit get if used as a container)
	count     int  // literal integer value, when the entry is one (-1 otherwise)
}

func (bh *bcHandler) lit(i int) fasValue {
	if i >= 0 && i < len(bh.h.literals) {
		return bh.h.literals[i]
	}
	return nil
}

// varRef returns a name reference for variable i, which is a term when the
// variable's name spells one.
func (bh *bcHandler) varRef(i int) int16 {
	if c, ok := bh.h.varTerms[i]; ok {
		return bh.b.termRef(c)
	}
	return bh.b.nameRef(bh.varName(i))
}

func (bh *bcHandler) varName(i int) string {
	if i >= 0 && i < len(bh.h.vars) {
		return bh.h.vars[i]
	}
	return fmt.Sprintf("var%d", i)
}

// errUnsupported marks bytecode the decompiler does not handle yet.
type errUnsupported struct {
	in  instr
	why string
}

func (e errUnsupported) Error() string {
	return fmt.Sprintf("unsupported %s at %05x: %s", e.in.name, e.in.off, e.why)
}

// stmtValue returns the expression for a value used as a statement: an
// evaluated reference is an explicit get (`get every mailbox of …`).
func (b *bcBuilder) stmtValue(v stackVal) int16 {
	if n := b.out.nodes[v.id]; v.gotten && isReference(n) && !(n.typ == 'n' && n.flags&flagAltSyntax != 0) {
		return b.node('e', v.id)
	}
	return v.id
}

// stmts decompiles instructions [lo, hi) into statement nodes.
func (bh *bcHandler) stmts(lo, hi int) ([]int16, error) {
	out, stack, err := bh.run(lo, hi, nil)
	if err != nil {
		return out, err
	}
	out = bh.foldDestructuring(out)
	// Leftover values at a block end are the block's result expression.
	for _, v := range stack {
		if !v.undefined && !v.it && !v.assigned {
			out = append(out, bh.b.node('l', bh.b.stmtValue(v)))
		}
	}
	return out, nil
}

// run simulates instructions [lo, hi) starting from stack, returning the
// statements completed and the final stack.
func (bh *bcHandler) run(lo, hi int, stack []stackVal) ([]int16, []stackVal, error) {
	var out []int16
	b := bh.b
	push := func(id int16) { stack = append(stack, stackVal{id: id, count: -1}) }
	pop := func(in instr) (stackVal, error) {
		if len(stack) == 0 {
			return stackVal{}, errUnsupported{in, "stack underflow"}
		}
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return v, nil
	}
	emit := func(stmt int16) {
		out = append(out, b.node('l', stmt))
	}
	pc := bh.index[lo]
	for pc < len(bh.prog) && bh.prog[pc].off < hi {
		in := bh.prog[pc]
		next := pc + 1
		switch name := in.name; name {
		case "Push0", "Push1", "Push2", "Push3", "PushMinus1":
			v := map[string]int{"Push0": 0, "Push1": 1, "Push2": 2, "Push3": 3, "PushMinus1": -1}[name]
			stack = append(stack, stackVal{id: b.literal(v), count: v})
		case "PushTrue", "PushFalse":
			push(b.literal(name == "PushTrue"))
		case "PushLiteral", "PushLiteralExtended":
			v := bh.lit(in.args[len(in.args)-1])
			if blk, ok := v.(*fasBlock); ok && blk.kind == 19 {
				// The object a whose clause examines: `it`.
				stack = append(stack, stackVal{id: b.node('g'), it: true, count: -1})
				break
			}
			sv := stackVal{id: b.literal(v), count: -1}
			if n, ok := v.(int); ok {
				sv.count = n
			}
			stack = append(stack, sv)
		case "PushGlobal", "PushGlobalExtended":
			push(b.literal(bh.lit(in.args[len(in.args)-1])))
		case "PushVariable", "PushVariableExtended":
			push(b.node('o', bh.varRef(in.args[len(in.args)-1])))
		case "PushParentVariable", "PopParentVariable":
			// Operands: scope level and index into that script object's names.
			name := bh.parentName(in.args[0], in.args[1])
			target := b.node('o', b.nameRef(name))
			if in.name == "PushParentVariable" {
				push(target)
				break
			}
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			emit(b.node('r', v.id, target))
			stack = append(stack, stackVal{id: v.id, assigned: true, count: -1})
		case "ObjectAliasQuote":
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			push(b.node('N', v.id))
		case "MakeComp":
			r, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			kind := in.args[0]
			if kind == 11 { // not
				push(b.node('H', r.id))
				break
			}
			l, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			ops := []string{"Equal", "NotEqual", "GreaterThan", "GreaterThanOrEqual", "LessThan",
				"LessThanOrEqual", "StartsWith", "EndsWith", "Contains"}
			switch {
			case kind < len(ops):
				push(b.node(binaryNodeType[ops[kind]], l.id, r.id))
			case kind == 9:
				push(b.node('F', l.id, r.id))
			case kind == 10:
				push(b.node('G', l.id, r.id))
			default:
				return out, stack, errUnsupported{in, fmt.Sprintf("comparison kind %d", kind)}
			}
		case "Consider":
			ignoring, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			considering, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			stmt, resume, err := bh.considerStmt(pc, considering, ignoring)
			if err != nil {
				return out, stack, err
			}
			emit(stmt)
			next = resume
		case "BeginTimeout", "BeginTransaction":
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			endName := map[string]string{"BeginTimeout": "EndTimeout", "BeginTransaction": "EndTransaction"}[name]
			j, err := bh.matching(pc, name, endName)
			if err != nil {
				return out, stack, err
			}
			body, err := bh.body(in.off+in.size, bh.prog[j].off)
			if err != nil {
				return out, stack, err
			}
			if name == "BeginTimeout" {
				emit(b.node('t', body, v.id))
			} else {
				session := b.ref()
				if !v.undefined {
					session = v.id
				}
				emit(b.node('u', body, session))
			}
			next = j + 1
		case "PushMe":
			push(b.node('f'))
		case "PushIt":
			stack = append(stack, stackVal{id: b.node('g'), it: true, count: -1})
		case "CopyData": // copy value to reference
			ref, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			emit(b.node('s', v.id, ref.id))
			stack = append(stack, stackVal{id: v.id, assigned: true, count: -1})
		case "DefineActor":
			// [name, parent] DefineActor script-literal … EndDefineActor
			if _, err := pop(in); err != nil { // parent
				return out, stack, err
			}
			nameVal, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			var inner []int16
			blk, _ := bh.lit(in.args[0]).(*fasBlock)
			switch {
			case blk != nil && blk.kind == 15 && len(blk.items) >= 4:
				inner, err = b.scriptItems(blk.items[3], false)
			case blk != nil && (blk.kind == 16 || blk.kind == 17):
				// An init handler builds the object: property assignments and
				// DefineProcedure for its handlers.
				inner, err = b.scriptInit(blk)
			default:
				err = errUnsupported{in, "script object literal"}
			}
			if err != nil {
				return out, stack, err
			}
			name := bh.keyOf(nameVal.id)
			if n := b.out.nodes[nameVal.id]; n.typ == 'm' && b.out.osCode[child0(n)] == "msng" {
				name = b.ref() // an anonymous script object
			}
			obj := b.node('h', name, b.node('k', b.cons(inner)))
			stack = append(stack, stackVal{id: obj, scriptDef: true, count: -1})
		case "EndDefineActor":
		case "DefineProperty": // property <var> : value (the unnamed var is parent)
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			emit(b.node('j', bh.varRef(in.args[0]), v.id))
			stack = append(stack, stackVal{id: v.id, assigned: true, count: -1})
		case "DefineProcedure":
			blk, ok := bh.lit(in.args[0]).(*fasBlock)
			h, ok2 := b.src.handlerFrom(blk)
			if !ok || !ok2 {
				return out, stack, errUnsupported{in, "handler literal"}
			}
			def, err := b.handlerDef(h, blk)
			if err != nil {
				return out, stack, err
			}
			out = append(out, def, b.node('l'))
		case "PushNext":
			stack = append(stack, stackVal{next: true, count: -1})
		case "Continue", "PositionalContinue":
			// [target, args…, count, PushNext, PushIt] → continue handler(…);
			// a direct parameter takes PushIt's place.
			var direct *stackVal
			if n := len(stack); n >= 2 && stack[n-2].next && !stack[n-1].it && !stack[n-1].next {
				direct = &stack[n-1]
				stack = stack[:n-1]
			}
			for len(stack) > 0 && (stack[len(stack)-1].it || stack[len(stack)-1].next) {
				stack = stack[:len(stack)-1]
			}
			var call int16
			var err error
			fake := instr{off: in.off, name: map[string]string{"Continue": "MessageSend", "PositionalContinue": "PositionalMessageSend"}[name], args: in.args}
			stack = append(stack[:len(stack):len(stack)], stack...)[:len(stack)]
			if name == "Continue" {
				call, err = bh.messageSend(fake, &stack)
			} else {
				call, err = bh.positionalCall(fake, &stack)
			}
			if err != nil {
				return out, stack, err
			}
			if cn := b.out.nodes[call]; cn.typ == 'I' && len(cn.children) > 1 {
				if d := b.out.nodes[cn.children[1]]; direct != nil {
					cn.children[1] = direct.id
					b.out.nodes[call] = cn
				} else if d.typ == 'f' {
					cn.children[1] = b.ref() // the implicit `me` target
					b.out.nodes[call] = cn
				}
			}
			if cn := b.out.nodes[call]; cn.typ == 'n' {
				call = child0(cn) // drop `of me`
			}
			push(b.node('M', call))
		case "GetResult":
			// The value of the last statement (`result`), mostly consumed by
			// the implicit return at the end of a handler.
			stack = append(stack, stackVal{id: b.node('o', b.nameRef("result")), undefined: true, count: -1})
		case "PushUndefined":
			stack = append(stack, stackVal{undefined: true, count: -1})
		case "PushEmpty":
			push(b.node('J', b.ref()))
		case "GetData":
			// Evaluating a reference: transparent in source, unless the value
			// then serves as a container (an explicit `get`).
			if n := len(stack); n > 0 {
				if stack[n-1].gotten {
					// Evaluated twice: set x to get properties.
					stack[n-1].id = b.node('e', stack[n-1].id)
				}
				stack[n-1].gotten = true
			}
		case "Equal", "NotEqual", "GreaterThan", "GreaterThanOrEqual", "LessThan", "LessThanOrEqual",
			"StartsWith", "EndsWith", "Contains", "Add", "Subtract", "Multiply", "Divide", "Quotient",
			"Remainder", "Power", "Concatenate", "Coerce":
			r, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			l, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			push(b.node(binaryNodeType[name], l.id, r.id))
		case "Not", "Negate":
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			typ := byte('H')
			if name == "Negate" {
				typ = 'd'
			}
			push(b.node(typ, v.id))
		case "And", "Or":
			left, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			end := in.jumpTarget(in.args[0])
			right, err := bh.expr(in.off+in.size, end)
			if err != nil {
				return out, stack, err
			}
			typ := byte('F')
			if name == "Or" {
				typ = 'G'
			}
			push(b.node(typ, left.id, stripBoolCoerce(b, right)))
			next = bh.index[end]
		case "MakeVector", "MakeList":
			n, err := pop(in)
			if err != nil || n.count < 0 || n.count > len(stack) {
				return out, stack, errUnsupported{in, "list size"}
			}
			items := make([]int16, n.count)
			for i := n.count - 1; i >= 0; i-- {
				v, _ := pop(in)
				items[i] = v.id
			}
			push(b.node('J', b.cons(items)))
		case "MakeRecord":
			n, err := pop(in)
			if err != nil || n.count < 0 || n.count > len(stack) || n.count%2 != 0 {
				return out, stack, errUnsupported{in, "record size"}
			}
			keys := make([]int16, n.count/2)
			values := make([]int16, n.count/2)
			for i := n.count/2 - 1; i >= 0; i-- {
				v, _ := pop(in)
				k, _ := pop(in)
				values[i] = v.id
				keys[i] = bh.keyOf(k.id)
			}
			push(b.node('K', b.cells(keys, values)))
		case "MakeObjectAlias":
			ref, err := bh.objectAlias(in, &stack)
			if err != nil {
				return out, stack, err
			}
			push(ref)
		case "PopGlobal", "PopGlobalExtended", "PopVariable", "PopVariableExtended":
			// Stores the top of the stack without popping it.
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			stack = append(stack, stackVal{id: v.id, assigned: true, count: -1})
			var target int16
			if strings.HasPrefix(name, "PopGlobal") {
				target = b.literal(bh.lit(in.args[len(in.args)-1]))
			} else {
				target = b.node('o', bh.varRef(in.args[len(in.args)-1]))
			}
			if v.scriptDef {
				emit(v.id) // script name … end script
			} else if n, ok := b.out.nodes[v.id]; ok && n.typ == 's' { // copy x to …
				emit(b.node('s', child0(n), target))
			} else {
				emit(b.node('r', v.id, target))
			}
		case "MatchLiteral":
			// A literal in a destructuring pattern: set {a, 5} to x.
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			stack = append(stack, stackVal{id: v.id, assigned: true, count: -1})
			emit(b.node('r', v.id, b.literal(bh.lit(in.args[0]))))
		case "Clone":
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			push(b.node('s', v.id)) // completed by the Pop that follows
		case "SetData":
			ref, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			v, err := pop(in)
			if err != nil {
				return out, stack, err
			}
			emit(b.node('r', v.id, ref.id))
			stack = append(stack, stackVal{id: v.id, assigned: true, count: -1})
		case "StoreResult":
			if len(stack) > 0 {
				v, _ := pop(in)
				if !v.undefined && !v.assigned {
					emit(b.stmtValue(v))
				}
			}
		case "Pop":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case "GCSwap":
			if n := len(stack); n >= 2 {
				stack[n-1], stack[n-2] = stack[n-2], stack[n-1]
			} else {
				return out, stack, errUnsupported{in, "swap on short stack"}
			}
		case "Dup":
			if len(stack) == 0 {
				return out, stack, errUnsupported{in, "dup on empty stack"}
			}
			stack[len(stack)-1].gotten = false // evaluated once for a destructuring set
			stack = append(stack, stack[len(stack)-1])
		case "Return":
			if in.off+in.size == len(bh.h.code) {
				// The implicit return that ends every handler. A pending
				// value (a tail call such as `continue beep`) is a statement.
				for _, v := range stack {
					if !v.undefined && !v.assigned && !v.it && !v.next {
						emit(b.stmtValue(v))
					}
				}
				stack = nil
				break
			}
			if len(stack) > 0 {
				v, _ := pop(in)
				emit(b.node('L', v.id))
			} else {
				emit(b.node('L'))
			}
		case "PositionalMessageSend":
			call, err := bh.positionalCall(in, &stack)
			if err != nil {
				return out, stack, err
			}
			push(call)
		case "MessageSend":
			call, err := bh.messageSend(in, &stack)
			if err != nil {
				return out, stack, err
			}
			push(call)
		case "Error":
			v, err := bh.errorCommand(in, &stack)
			if err != nil {
				return out, stack, err
			}
			emit(v)
		case "Exit":
			emit(b.node('S'))
		case "TestIf":
			stmt, resume, err := bh.ifStmt(pc, &stack)
			if err != nil {
				return out, stack, err
			}
			emit(stmt)
			next = resume
		case "Tell":
			stmt, resume, isExpr, err := bh.tellStmt(pc, &stack)
			if err != nil {
				return out, stack, err
			}
			if isExpr {
				push(stmt) // (tell x to expr) used as a value
			} else {
				emit(stmt)
			}
			next = resume
		case "LinkRepeat":
			stmt, resume, err := bh.repeatStmt(pc)
			if err != nil {
				return out, stack, err
			}
			emit(stmt)
			next = resume
		case "ErrorHandler":
			stmt, resume, err := bh.tryStmt(pc)
			if err != nil {
				return out, stack, err
			}
			emit(stmt)
			next = resume
		case "Jump":
			// Structural jumps are consumed by the constructs above.
			return out, stack, errUnsupported{in, "stray jump"}
		default:
			return out, stack, errUnsupported{in, "not implemented"}
		}
		pc = next
	}
	return out, stack, nil
}

// expr decompiles [lo, hi) as a single expression.
func (bh *bcHandler) expr(lo, hi int) (int16, error) {
	stmts, err := bh.stmts(lo, hi)
	if err != nil {
		return 0, err
	}
	if len(stmts) != 1 {
		return 0, fmt.Errorf("expected one expression in %05x-%05x, got %d", lo, hi, len(stmts))
	}
	return child0(bh.b.out.nodes[stmts[0]]), nil
}

// stripBoolCoerce removes the `as boolean` the compiler appends to the right
// operand of and/or.
func stripBoolCoerce(b *bcBuilder, id int16) int16 {
	n := b.out.nodes[id]
	if n.typ == 'c' && len(n.children) == 2 {
		if cls := b.out.nodes[n.children[1]]; b.out.osCode[child0(cls)] == "bool" {
			return n.children[0]
		}
	}
	return id
}

// keyOf turns a pushed literal node into a name reference (record keys,
// parameter labels).
func (bh *bcHandler) keyOf(id int16) int16 {
	n := bh.b.out.nodes[id]
	if (n.typ == '1' || n.typ == 'm' || n.typ == 'o') && len(n.children) == 1 && n.children[0] < 0 {
		return n.children[0]
	}
	return bh.b.nameRef("<key>")
}

var binaryNodeType = map[string]byte{
	"Equal": '=', "NotEqual": '>', "GreaterThan": '?', "GreaterThanOrEqual": '@', "LessThan": 'A',
	"LessThanOrEqual": 'B', "StartsWith": 'C', "EndsWith": 'D', "Contains": 'E', "Add": '[',
	"Subtract": '\\', "Multiply": ']', "Divide": '^', "Quotient": '_', "Remainder": '`', "Power": 'a',
	"Concatenate": 'b', "Coerce": 'c',
}

// isReference reports whether n is an object reference (rather than a call
// or literal), which only an explicit get evaluates as a statement.
func isReference(n nodeRec) bool {
	switch n.typ {
	case 'n', '1', '2', '3', '4', '5', '6', '7':
		return true
	}
	return false
}

// stmtValue returns the expression of an expression statement, looking
// through an explicit get.
func (bh *bcHandler) stmtValue(stmt int16) int16 {
	v := child0(bh.b.out.nodes[stmt])
	if n := bh.b.out.nodes[v]; n.typ == 'e' {
		return child0(n)
	}
	return v
}

// consumesValue reports whether the instruction following a construct means
// the construct's value is used as an operand, i.e. it is not a statement or
// block boundary.
func consumesValue(name string) bool {
	switch name {
	case "StoreResult", "Pop", "Return", "GetResult", "Jump", "Exit", "LinkRepeat",
		"EndTell", "EndErrorHandler", "EndConsider", "EndTimeout", "EndTransaction",
		"ErrorHandler", "Tell", "Consider", "BeginTimeout", "BeginTransaction",
		"EndDefineActor", "DefineProcedure", "Dup", "HandleError":
		return false
	}
	return true
}
