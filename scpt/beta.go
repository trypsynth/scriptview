package scpt

import (
	"encoding/binary"
	"strings"
)

// The AppleScript 1.0b1 beta (FASD 0.95) stores the same syntax tree as
// later versions with a different node alphabet: later versions inserted new
// node types, which shifted the letters after them. The beta also lays out a
// few nodes differently, stores no parentheses, and uses other kind bytes for
// terms and identifiers. translateBeta rewrites a beta object graph into the
// AppleScript 1.0 form, so the rest of the decompiler reads it unchanged.

// betaNodeKind maps a beta node letter to the AppleScript 1.0 letter.
func betaNodeKind(k byte) byte {
	switch {
	case k >= '4' && k <= 'E': // references, insertion points, comparisons
		return k - 3
	case k >= 'H' && k <= 'N': // starts with … not, commands
		return k - 5
	case k >= 'P' && k <= 'i': // lists … control statements, arithmetic, as
		return k - 6
	case k == 'j': // unary minus
		return 'd'
	case k == 'k': // get
		return 'e'
	case k >= 'm' && k <= 'z': // me … copy
		return k - 7
	}
	return k
}

// Beta term and identifier kinds.
const (
	betaKindEvent = 0x31 // suite(4) + event(4) + parameter information
	betaKindCode  = 0x32 // a property code
	betaKindIdent = 0x33
)

// betaFlags maps the flags of a beta node, already relettered, to the 1.0 bits.
func betaFlags(kind byte, f uint16, isStmt bool) uint16 {
	switch kind {
	case 'l':
		if isStmt {
			switch f {
			case 0, 3: // a -- comment after a statement, or on its own line
				return commentLine
			case 4:
				return commentBlock
			case 5:
				return commentNone // a blank line
			}
		}
		switch f {
		case 6:
			return flagContinuation // ¬ before the expression
		case 7:
			return flagContinuation | flagPossessive // ¬ after it
		case 8:
			return flagThe
		}
		return 0
	case '4': // index: first and last keep 0x1; 1st, 2nd use 0x2 later
		if f == 4 {
			return flagNumericOrdinal
		}
		return f
	case 'n': // of: in, my, its
		switch f {
		case 1:
			return flagThe
		case 3:
			return flagAltSyntax
		case 4:
			return flagPossessive
		}
		return 0
	case '=', '>': // is, is equal to; is not, isn't equal to
		switch {
		case f == 1 && kind == '=':
			return 3
		case f == 1:
			return flagThe
		case f == 3:
			return flagAltSyntax
		}
		return 0
	case 'A': // comes before
		if f == 3 {
			return 0x101
		}
		return 0
	case '?': // comes after
		if f == 3 {
			return 0x101
		}
		return 0
	case 'C': // begins with
		if f == 1 {
			return flagThe
		}
		return 0
	case 'E': // is contained by
		if f == 1 {
			return flagAltSyntax
		}
		return 0
	case 'I': // a command written after its direct parameter: x exists
		if f == 1 || f == 5 {
			return 3
		}
		return 0
	case 'i': // `to` instead of `on`
		if f == 1 {
			return flagThe
		}
		return 0
	case '8', '9', ':': // in front of, in back of, behind; front
		if f != 0 {
			return flagThe
		}
		return 0
	case 'Z', '6': // one-line if; where instead of whose
		return f
	}
	return 0
}

// emptyBodies lists, for each node kind, the children that hold a body.
var emptyBodies = map[byte][]int{
	'O': {0}, 'Q': {0, 3}, 'T': {0}, 'U': {0}, 'V': {0}, 'W': {0}, 'X': {0}, 'Y': {0},
	'Z': {1, 3}, 'h': {1}, 'i': {1},
}

// translateBeta rewrites the objects of a beta file in place and returns the
// list with any objects it added.
func translateBeta(objs []*object) []*object {
	// The decompiler follows only shared (non-negative) references to nodes
	// and lists, as later compilers write them, so give every inline one a
	// fresh positive number. Values stay inline.
	// Scripts with a kind 18 root, the 8-item form, come from an earlier
	// build whose symbol table lacks the second index and ID entries: their
	// true and false are entries 123 and 124, where later beta files keep
	// those key forms.
	if len(objs) > 0 && objs[0].ref == 0 && objs[0].typ == objValueBlock2 && objs[0].kind == 18 {
		for _, o := range objs {
			if o.typ == objSymbol && len(o.data) == 4 {
				if v := binary.BigEndian.Uint32(o.data); v&7 == 1 && v>>3 >= 123 {
					o.data = binary.BigEndian.AppendUint32(nil, v+2<<3)
				}
			}
		}
	}

	maxRef := int16(0)
	for _, o := range objs {
		if o.ref > maxRef {
			maxRef = o.ref
		}
	}
	renum := map[int16]int16{}
	next := maxRef
	newRef := func() int16 {
		if next < 0x7ffe {
			next++
		}
		return next
	}
	minRef := int16(0)
	for _, o := range objs {
		if o.ref < minRef {
			minRef = o.ref
		}
	}
	for _, o := range objs {
		if o.ref < 0 && (isContainer(o.typ) || o.typ == objString) { // comments too
			if _, ok := renum[o.ref]; !ok {
				renum[o.ref] = newRef()
			}
		}
	}
	byRef := map[int16]*object{}
	for _, o := range objs {
		if r, ok := renum[o.ref]; ok {
			o.ref = r
		}
		for i, c := range o.refs {
			if r, ok := renum[c]; ok {
				o.refs[i] = r
			}
		}
		if _, ok := byRef[o.ref]; !ok {
			byRef[o.ref] = o
		}
	}

	add := func(o *object) int16 {
		if isContainer(o.typ) {
			o.ref = newRef()
		} else if minRef > -0x7ffe {
			minRef--
			o.ref = minRef // values stay inline, where the decompiler names them
		}
		objs = append(objs, o)
		byRef[o.ref] = o
		return o.ref
	}
	isNil := func(ref int16) bool {
		o := byRef[ref]
		return o == nil || (o.typ == objSymbol && len(o.data) == 0) || (o.typ == objList && len(o.refs) == 0)
	}
	varNode := func(ref int16) int16 {
		if o := byRef[ref]; o != nil && o.typ == objUserID {
			return add(&object{typ: objCmdBlock, kind: 'o', refs: []int16{ref}})
		}
		return ref
	}

	// Statements are the items of blocks, the bodies of compound statements
	// and the comment lines; everything else under an 's' is an expression.
	stmtLine := map[int16]bool{}
	for _, o := range objs {
		if o.typ == objCmdBlock && o.kind == 's' && (o.flags <= 5 || isNil(child0refs(o.refs))) {
			stmtLine[o.ref] = true
		}
	}

	for _, o := range objs {
		switch o.typ {
		case objCodeID:
			switch o.kind {
			case osKindConstant:
				if len(o.data) == 4 { // an enumerator without its type
					o.data = append([]byte("enum"), o.data...)
				}
			case betaKindEvent:
				o.kind = osKindEvent
			case betaKindCode:
				o.kind = osKindCode2
			}
			continue
		case objUserID:
			if o.kind == betaKindIdent {
				o.kind = identTypeMarker
			}
			continue
		case objCmdBlock:
		default:
			continue
		}
		o.kind = betaNodeKind(o.kind)
		o.flags = betaFlags(o.kind, o.flags, stmtLine[o.ref])
		// Empty bodies store a literal nil.
		for _, i := range emptyBodies[o.kind] {
			if i < len(o.refs) {
				if c := byRef[o.refs[i]]; c != nil && c.typ == objCmdBlock && betaNodeKind(c.kind) == 'm' && isNil(child0refs(c.refs)) {
					o.refs[i] = 0
				}
			}
		}
		switch o.kind {
		case 'm': // a precompiled reference: expand it into reference nodes
			if len(o.refs) == 1 {
				if kind, refs, ok := specNode(o.refs[0], byRef, add, 0); ok {
					o.kind, o.refs = kind, refs
				}
			}
		case 'I': // a handler call keeps positional arguments in the labeled slot
			if len(o.refs) == 3 && isNil(o.refs[1]) {
				if c := byRef[o.refs[2]]; c != nil && c.typ == objList && len(c.refs) > 0 {
					o.refs[1], o.refs[2] = o.refs[2], 0
				}
			}
		case 'i': // handler: [name, body, direct parameter, positional parameters]
			if len(o.refs) < 2 {
				continue
			}
			args := int16(0)
			if len(o.refs) > 2 && !isNil(o.refs[2]) {
				args = varNode(o.refs[2])
			} else if len(o.refs) > 3 && !isNil(o.refs[3]) {
				args = o.refs[3]
			}
			sig := add(&object{typ: objCmdBlock, kind: 'I', refs: []int16{o.refs[0], args, 0}})
			o.refs = []int16{sig, o.refs[1]}
		case 'Q': // try: [body, error variable, error parameters, handler]
			if len(o.refs) < 4 {
				continue
			}
			msg, params, handler := o.refs[1], o.refs[2], o.refs[3]
			if isNil(msg) {
				msg = 0
			} else {
				msg = varNode(msg)
			}
			if isNil(params) {
				params = 0
			}
			if h := byRef[handler]; h == nil || h.typ != objCmdBlock || (betaNodeKind(h.kind) == 'm' && isNil(child0refs(h.refs))) {
				handler = 0 // an empty `on error` clause stores a nil literal
			}
			clause := add(&object{typ: objCmdBlock, kind: 'R', refs: []int16{0, msg, params}})
			o.refs = []int16{o.refs[0], clause, handler}
		case 'h': // script object: the name is a string
			if len(o.refs) > 0 {
				if s := byRef[o.refs[0]]; s != nil && s.typ == objString && len(s.data) >= 2 {
					if n := int(binary.BigEndian.Uint16(s.data)); 2+n <= len(s.data) {
						name := s.data[2 : 2+n]
						lower := []byte(strings.ToLower(string(name)))
						data := binary.BigEndian.AppendUint16(nil, uint16(len(lower)))
						data = append(data, lower...)
						if string(lower) == string(name) {
							data = append(data, 0, 0)
						} else {
							data = binary.BigEndian.AppendUint16(data, uint16(len(name)))
							data = append(data, name...)
						}
						o.refs[0] = add(&object{typ: objUserID, kind: identTypeMarker, data: data})
					}
				}
			}
		}
	}
	// A reference as one end of a range needs parentheses, which beta files
	// don't store: text from (end of word 1) to (end of word 2).
	for _, o := range objs {
		if o.typ != objCmdBlock || o.kind != '7' || len(o.refs) != 3 {
			continue
		}
		for i := 1; i < 3; i++ {
			c := byRef[o.refs[i]]
			if c == nil || c.typ != objCmdBlock {
				continue
			}
			if c.kind == 'l' && c.flags&flagParens == 0 {
				if in := byRef[child0refs(c.refs)]; in != nil && in.typ == objCmdBlock && in.kind == 'n' {
					c.flags |= flagParens
				}
			} else if c.kind == 'n' {
				o.refs[i] = add(&object{typ: objCmdBlock, kind: 'l', flags: flagParens, refs: []int16{o.refs[i], 0}})
			}
		}
	}
	return objs
}

func child0refs(refs []int16) int16 {
	if len(refs) == 0 {
		return 0
	}
	return refs[0]
}

// isContainer reports whether objects of type typ hold references to others.
func isContainer(typ byte) bool {
	switch typ {
	case objCmdBlock, objList, objBinding, objPointerBlock, objValueBlock, objValueBlock2:
		return true
	}
	return false
}

// specNode expands a precompiled reference, a typed vector of kind 21 to 31
// whose first item is the container, into the node kind and children of an
// equivalent reference. Values inside it get literal nodes.
func specNode(ref int16, byRef map[int16]*object, add func(*object) int16, depth int) (byte, []int16, bool) {
	v := byRef[ref]
	if v == nil || (v.typ != objValueBlock && v.typ != objValueBlock2) || len(v.refs) == 0 || depth > 32 {
		return 0, nil, false
	}
	lit := func(r int16) int16 {
		if w := byRef[r]; w != nil && w.typ == objCmdBlock {
			return r
		}
		if k, refs, ok := specNode(r, byRef, add, depth+1); ok {
			return add(&object{typ: objCmdBlock, kind: k, refs: refs})
		}
		return add(&object{typ: objCmdBlock, kind: 'm', refs: []int16{r}})
	}
	it := v.refs[1:]
	var part int16
	switch {
	case v.kind == 21 && len(it) == 1: // property
		part = add(&object{typ: objCmdBlock, kind: '1', refs: []int16{it[0]}})
	case v.kind == 22 && len(it) == 1, v.kind == 23 && len(it) == 1: // every, some
		part = add(&object{typ: objCmdBlock, kind: '2' + byte(v.kind-22), refs: []int16{it[0]}})
	case v.kind == 24 && len(it) == 2: // index
		part = add(&object{typ: objCmdBlock, kind: '4', refs: []int16{it[0], lit(it[1])}})
	case v.kind == 27 && len(it) == 3: // range
		part = add(&object{typ: objCmdBlock, kind: '7', refs: []int16{it[0], lit(it[1]), lit(it[2])}})
	case v.kind == 28 && len(it) == 1, v.kind == 29 && len(it) == 1: // before, after
		part = add(&object{typ: objCmdBlock, kind: '8' + byte(v.kind-28), refs: []int16{it[0]}})
	case v.kind == 30 && len(it) == 0, v.kind == 31 && len(it) == 0: // beginning, end
		part = add(&object{typ: objCmdBlock, kind: ':' + byte(v.kind-30)})
	default:
		return 0, nil, false
	}
	c := byRef[v.refs[0]]
	if c == nil || (c.typ == objSymbol && len(c.data) == 0) {
		p := byRef[part]
		return p.kind, p.refs, true
	}
	return 'n', []int16{part, lit(v.refs[0])}, true
}
