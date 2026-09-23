package scpt

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Every compiled script, run-only or not, carries bytecode for a small
// stack-based virtual machine. The object graph's root (object 0) is a typed
// vector whose last element is the script's handler table:
//
//	[nil, [handler names…], handler, handler, …]
//
// and each handler is a typed vector of kind 16:
//
//	block16{name, _, positional params, labeled params, variable names, literals, code}
//
// Instructions address variables by index into the variable-name list and
// constants (numbers, strings, terms, handler names) by index into the
// literal list.

// fasValue is a decoded object: nil, int, float64, bool, string (identifier
// or text), fasCode, fasBlock, []fasValue (lists and vectors), fasBinding or
// []byte.
type fasValue any

// fasCode is a term: a class, property or constant code, or an event.
type fasCode struct {
	kind byte // osKindCode, osKindConstant or osKindEvent
	code string
	enum string // enumeration type, for typed constants
}

// fasBlock is a typed vector (value block).
type fasBlock struct {
	kind  byte
	items []fasValue
}

// fasBinding is a record or labeled-parameter chain of key/value pairs.
type fasBinding struct {
	keys, values []fasValue
}

// fasText is a string literal.
type fasText string

// fasApp is an application specifier (from an alias), by file name.
type fasApp string

// fasDesc is another descriptor literal (date, «data …»).
type fasDesc descRec

// value decodes the object behind ref.
func (dc *decompState) value(ref int16) fasValue {
	return dc.valueDepth(ref, 0)
}

func (dc *decompState) valueDepth(ref int16, depth int) fasValue {
	// Objects are shared (and may be cyclic), so decode each only once.
	if v, ok := dc.valueCache[ref]; ok {
		return v
	}
	if dc.valueCache == nil {
		dc.valueCache = map[int16]fasValue{}
	}
	dc.valueCache[ref] = nil // cycle guard
	v := dc.decodeValue(ref, depth)
	dc.valueCache[ref] = v
	return v
}

func (dc *decompState) decodeValue(ref int16, depth int) fasValue {
	o, ok := dc.objects[ref]
	if !ok || depth > 256 {
		return nil
	}
	switch o.typ {
	case objFixnum:
		return int(int16(o.size))
	case objBool:
		return o.size != 0
	case objLong:
		if len(o.data) == 4 {
			return int(int32(binary.BigEndian.Uint32(o.data)))
		}
	case objFloat:
		if len(o.data) == 8 {
			return math.Float64frombits(binary.BigEndian.Uint64(o.data))
		}
	case objUserID:
		if names := identNames(o.data); len(names) > 0 {
			return bestName(names)
		}
	case objCodeID:
		switch {
		case o.kind == osKindCode && len(o.data) == 4:
			return fasCode{kind: osKindCode, code: fourCC(o.data)}
		case o.kind == osKindConstant && len(o.data) == 8:
			return fasCode{kind: osKindConstant, enum: fourCC(o.data[:4]), code: fourCC(o.data[4:])}
		case o.kind == osKindEvent && len(o.data) >= 8:
			return fasCode{kind: osKindEvent, code: macRoman(o.data[:8])}
		}
	case objString:
		return fasText(dc.comments[ref])
	case objDataBlockRaw, objLongDataBlockRaw:
		return o.data
	case objDataBlock, objLongDataBlock:
		if name, ok := aliasAppName(o.data); ok {
			return fasApp(name)
		}
		if o.kind == 0x0d && len(o.data) >= 4 {
			return fasDesc{typ: string(o.data[:4]), data: o.data[4:]}
		}
	case objList:
		// A chain of cons cells: [head, tail].
		var out []fasValue
		for cur, steps := o, 0; cur != nil && cur.typ == objList && len(cur.refs) == 2 && steps < 100000; steps++ {
			out = append(out, dc.valueDepth(cur.refs[0], depth+1))
			next, ok := dc.objects[cur.refs[1]]
			if !ok || next.typ != objList {
				break
			}
			cur = next
		}
		return out
	case objBinding:
		var b fasBinding
		for cur, steps := o, 0; cur != nil && cur.typ == objBinding && len(cur.refs) == 3 && steps < 100000; steps++ {
			b.keys = append(b.keys, dc.valueDepth(cur.refs[0], depth+1))
			b.values = append(b.values, dc.valueDepth(cur.refs[1], depth+1))
			next, ok := dc.objects[cur.refs[2]]
			if !ok {
				break
			}
			cur = next
		}
		return b
	case objPointerBlock:
		out := make([]fasValue, len(o.refs))
		for i, r := range o.refs {
			out[i] = dc.valueDepth(r, depth+1)
		}
		return out
	case objValueBlock, objValueBlock2:
		if o.kind == wrapText && len(o.refs) == 1 {
			if t, ok := dc.textByID[o.refs[0]]; ok {
				return fasText(t)
			}
		}
		b := &fasBlock{kind: o.kind, items: make([]fasValue, len(o.refs))}
		for i, r := range o.refs {
			b.items[i] = dc.valueDepth(r, depth+1)
		}
		return b
	}
	return nil
}

// handlerCode is one handler's (or the implicit run handler's) bytecode and
// the tables its instructions index into.
type handlerCode struct {
	name     fasValue // identifier string or fasCode event
	params   []string
	pattern  []string // a list-pattern direct parameter: on run {a, b}
	vars     []string
	literals []fasValue
	code     []byte
}

// handlerTable returns the script's handlers from the root object's table.
func (dc *decompState) handlerTable() []handlerCode {
	root, ok := dc.value(0).(*fasBlock)
	if !ok || len(root.items) == 0 {
		return nil
	}
	table, ok := root.items[len(root.items)-1].([]fasValue)
	if !ok {
		return nil
	}
	var out []handlerCode
	for _, item := range table[min(2, len(table)):] {
		if h, ok := dc.handlerFrom(item); ok {
			out = append(out, h)
		}
	}
	return out
}

// allHandlers returns the handler table plus handlers nested in literal
// tables (script objects built at run time).
func (dc *decompState) allHandlers() []handlerCode {
	queue := dc.handlerTable()
	var out []handlerCode
	for len(queue) > 0 && len(out) < 10000 {
		h := queue[0]
		queue = queue[1:]
		out = append(out, h)
		for _, lit := range h.literals {
			if nested, ok := dc.handlerFrom(lit); ok {
				queue = append(queue, nested)
			}
		}
	}
	return out
}

// handlerFrom decodes a kind-16 handler block:
// [name, _, positional params, labeled params, variable names, literals, code].
func (dc *decompState) handlerFrom(v fasValue) (handlerCode, bool) {
	b, ok := v.(*fasBlock)
	if !ok || (b.kind != 16 && b.kind != 17) || len(b.items) < 7 {
		return handlerCode{}, false
	}
	h := handlerCode{name: b.items[0]}
	h.vars = stringsOf(b.items[4])
	if lits, ok := b.items[5].([]fasValue); ok {
		h.literals = lits
	}
	if code, ok := b.items[6].([]byte); ok {
		h.code = code
	}
	// Positional parameters: block4{count, block0{name…}}; an event
	// handler's direct parameter is a bare name.
	switch p := b.items[2].(type) {
	case *fasBlock:
		if len(p.items) >= 2 {
			if names, ok := p.items[1].(*fasBlock); ok {
				h.params = stringsOf(names.items)
			}
		}
	case string:
		h.params = []string{p}
	case []fasValue:
		// A pattern parameter: on run {input, parameters}.
		h.pattern = stringsOf(p)
	}
	return h, true
}

func stringsOf(v fasValue) []string {
	list, _ := v.([]fasValue)
	out := make([]string, 0, len(list))
	for _, x := range list {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// Opcode names, indexed by opcode byte. Bytes 0xa0-0xff pack a small operand
// into the opcode: PushVariable/PopVariable/PushGlobal/PopGlobal take 4 bits,
// PushLiteral 5.
var opNames = func() [256]string {
	var t [256]string
	base := []string{
		"Equal", "NotEqual", "GreaterThan", "GreaterThanOrEqual", "LessThan", "LessThanOrEqual", "StartsWith", "EndsWith",
		"Contains", "And", "Or", "Not", "MessageSend", "MakeList", "MakeRecord", "Return",
		"Continue", "ObjectAliasQuote", "Tell", "Consider", "ErrorHandler", "Error", "Exit", "LinkRepeat",
		"RepeatNTimes", "RepeatWhile", "RepeatUntil", "RepeatInCollection", "RepeatInRange", "TestIf", "Add", "Subtract",
		"Multiply", "Divide", "Quotient", "Remainder", "Power", "Concatenate", "Coerce", "Negate",
		"GetData", "PushMe", "PushIt", "PositionalMessageSend",
	}
	copy(t[:], base)
	for i := 0x2c; i <= 0x37; i++ {
		t[i] = "MakeObjectAlias"
	}
	for i := 0x38; i <= 0x44; i++ {
		t[i] = "MakeComp"
	}
	rest := map[int]string{
		0x45: "GetData", 0x46: "SetData", 0x47: "CopyData",
		0x4a: "PositionalContinue", 0x4b: "DefineActor", 0x4c: "DefineProcedure", 0x4d: "DefineClosure",
		0x4e: "DefineProperty", 0x4f: "StoreResult", 0x50: "GetResult", 0x51: "Clone", 0x52: "Of",
		0x53: "EndDefineActor", 0x54: "EndOf", 0x55: "EndTell", 0x56: "EndConsider", 0x57: "EndErrorHandler",
		0x58: "HandleError", 0x59: "Jump", 0x5a: "Pop", 0x5b: "Dup", 0x5c: "GCSwap",
		0x5d: "PushVariableExtended", 0x5e: "PopVariableExtended", 0x5f: "PushGlobalExtended",
		0x60: "PopGlobalExtended", 0x61: "PushLiteralExtended", 0x62: "PushParentVariable", 0x63: "PopParentVariable",
		0x64: "PushNext", 0x65: "PushTrue", 0x66: "PushFalse", 0x67: "PushEmpty", 0x68: "PushUndefined",
		0x69: "PushMinus1", 0x6a: "Push0", 0x6b: "Push1", 0x6c: "Push2", 0x6d: "Push3",
		0x6e: "BeginTimeout", 0x6f: "EndTimeout", 0x70: "BeginTransaction", 0x71: "EndTransaction",
		0x75: "MatchLiteral", 0x76: "MakeVector",
	}
	for i, n := range rest {
		t[i] = n
	}
	for i := 0xa0; i <= 0xff; i++ {
		t[i] = [...]string{"PushVariable", "PopVariable", "PushGlobal", "PopGlobal", "PushLiteral", "PushLiteral"}[(i-0xa0)/16]
	}
	return t
}()

// opWords gives the number of 16-bit operand words that follow an opcode.
var opWords = map[string]int{
	"Jump": 1, "TestIf": 1, "And": 1, "Or": 1, "LinkRepeat": 1,
	"PushLiteralExtended": 1, "PushGlobalExtended": 1, "PopGlobalExtended": 1,
	"PushVariableExtended": 1, "PopVariableExtended": 1,
	"MessageSend": 1, "PositionalMessageSend": 1, "Continue": 1, "PositionalContinue": 1,
	"Tell": 1, "Consider": 1, "ErrorHandler": 1, "EndErrorHandler": 1, "HandleError": 2,
	"PushParentVariable": 2, "PopParentVariable": 2, "DefineActor": 1, "DefineProcedure": 1, "DefineProperty": 1,
	"RepeatInRange": 1, "RepeatInCollection": 1, "BeginTransaction": 1,
}

// instr is one decoded instruction.
type instr struct {
	off  int    // byte offset
	op   byte   // opcode byte
	name string // mnemonic
	args []int  // operands: the packed operand, then any 16-bit words
	size int    // encoded length
}

// decodeInstr decodes the instruction at off.
func decodeInstr(code []byte, off int) (instr, error) {
	if off >= len(code) {
		return instr{}, fmt.Errorf("offset %d past end", off)
	}
	op := code[off]
	in := instr{off: off, op: op, name: opNames[op], size: 1}
	if in.name == "" {
		return in, fmt.Errorf("unknown opcode %#x at %d", op, off)
	}
	switch {
	case op >= 0xe0:
		in.args = append(in.args, int(op&0x1f))
	case op >= 0xa0:
		in.args = append(in.args, int(op&0x0f))
	case in.name == "MakeObjectAlias":
		in.args = append(in.args, int(op)-0x2c)
	case in.name == "MakeComp":
		in.args = append(in.args, int(op)-0x38)
	}
	for i := 0; i < opWords[in.name]; i++ {
		if off+in.size+2 > len(code) {
			return in, fmt.Errorf("truncated %s at %d", in.name, off)
		}
		in.args = append(in.args, int(int16(binary.BigEndian.Uint16(code[off+in.size:]))))
		in.size += 2
	}
	return in, nil
}

// disassemble decodes a whole code block.
func disassemble(code []byte) ([]instr, error) {
	var out []instr
	for off := 0; off < len(code); {
		in, err := decodeInstr(code, off)
		if err != nil {
			return out, err
		}
		out = append(out, in)
		off += in.size
	}
	return out, nil
}

// jumpTarget returns the absolute target of a relative branch operand, which
// counts from the byte after the opcode.
func (in instr) jumpTarget(word int) int {
	return in.off + 1 + word
}

// isBranch reports whether the instruction's first word is a branch offset.
func (in instr) isBranch() bool {
	switch in.name {
	case "Jump", "TestIf", "And", "Or", "LinkRepeat", "ErrorHandler", "EndErrorHandler":
		return true
	}
	return false
}

// Disassemble renders the script's bytecode as annotated assembly.
func Disassemble(f *File) string {
	dc := f.dc
	if dc == nil || dc.objects == nil {
		return "(no bytecode)\n"
	}
	var sb strings.Builder
	// Handlers can hold nested code (script objects defined at run time) in
	// their literal tables; list those too.
	for _, h := range dc.allHandlers() {
		fmt.Fprintf(&sb, "handler %s", dc.fasString(h.name))
		if len(h.params) > 0 {
			fmt.Fprintf(&sb, "(%s)", strings.Join(h.params, ", "))
		}
		fmt.Fprintf(&sb, "  vars=%v\n", h.vars)
		prog, err := disassemble(h.code)
		for _, in := range prog {
			fmt.Fprintf(&sb, "  %05x  %-22s%s\n", in.off, in.name, dc.operandText(h, in))
		}
		if err != nil {
			fmt.Fprintf(&sb, "  !! %v\n", err)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// operandText annotates an instruction's operands.
func (dc *decompState) operandText(h handlerCode, in instr) string {
	var parts []string
	lit := func(i int) string {
		if i >= 0 && i < len(h.literals) {
			return dc.fasString(h.literals[i])
		}
		return fmt.Sprintf("L%d?", i)
	}
	variable := func(i int) string {
		if i >= 0 && i < len(h.vars) {
			return h.vars[i]
		}
		return fmt.Sprintf("var%d?", i)
	}
	switch in.name {
	case "PushLiteral", "PushLiteralExtended", "PushGlobal", "PushGlobalExtended",
		"PopGlobal", "PopGlobalExtended", "MessageSend", "PositionalMessageSend":
		idx := in.args[len(in.args)-1]
		parts = append(parts, fmt.Sprintf("%d  ; %s", idx, lit(idx)))
	case "PushVariable", "PopVariable", "PushVariableExtended", "PopVariableExtended":
		idx := in.args[len(in.args)-1]
		parts = append(parts, fmt.Sprintf("%d  ; %s", idx, variable(idx)))
	default:
		for i, a := range in.args {
			if i == 0 && in.isBranch() {
				parts = append(parts, fmt.Sprintf("→%05x", in.jumpTarget(a)))
				continue
			}
			parts = append(parts, fmt.Sprint(a))
		}
	}
	return strings.Join(parts, " ")
}

// fasString renders a decoded value compactly for listings.
func (dc *decompState) fasString(v fasValue) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case string:
		return x
	case fasText:
		return quoteAS(string(x))
	case fasCode:
		switch x.kind {
		case osKindEvent:
			return dc.eventName(x.code)
		case osKindConstant:
			return dc.term(x.code, kindValue)
		}
		return dc.term(x.code, kindValue)
	case *fasBlock:
		parts := make([]string, len(x.items))
		for i, it := range x.items {
			parts[i] = dc.fasString(it)
		}
		return fmt.Sprintf("block%d{%s}", x.kind, strings.Join(parts, ", "))
	case []fasValue:
		parts := make([]string, len(x))
		for i, it := range x {
			parts[i] = dc.fasString(it)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []byte:
		return fmt.Sprintf("<%d bytes>", len(x))
	case fasBinding:
		parts := make([]string, len(x.keys))
		for i := range x.keys {
			parts[i] = dc.fasString(x.keys[i]) + ":" + dc.fasString(x.values[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case float64:
		return formatReal(x)
	case fasApp:
		return "application " + quoteAS(displayAppName(string(x)))
	case fasDesc:
		return descLiteral(descRec(x))
	}
	return fmt.Sprint(v)
}
