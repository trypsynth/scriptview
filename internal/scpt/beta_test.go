package scpt

import (
	"encoding/binary"
	"strings"
	"testing"
)

// bnode is one record of a synthetic beta (FASD 0.95) file.
type bnode struct {
	typ, kind byte
	flags     uint16
	data      []byte
	kids      []*bnode
	ref       int16
}

func bNode(kind byte, flags uint16, kids ...*bnode) *bnode {
	return &bnode{typ: objCmdBlock, kind: kind, flags: flags, kids: kids}
}

func bNil() *bnode { return &bnode{typ: objSymbol} }

func bSym(index uint32) *bnode {
	return &bnode{typ: objSymbol, data: binary.BigEndian.AppendUint32(nil, index*8+1)}
}

func bFix(v int16) *bnode { return &bnode{typ: objFixnum, data: []byte{byte(v >> 8), byte(v)}} }

func bStr(s string) *bnode {
	d := binary.BigEndian.AppendUint16(nil, uint16(len(s)))
	return &bnode{typ: objString, data: append(append(d, s...), 0, 2, 0, 0)}
}

func bID(name string) *bnode {
	lower := strings.ToLower(name)
	d := binary.BigEndian.AppendUint16(nil, uint16(len(lower)))
	d = append(d, lower...)
	if lower == name {
		d = append(d, 0, 0)
	} else {
		d = binary.BigEndian.AppendUint16(d, uint16(len(name)))
		d = append(d, name...)
	}
	return &bnode{typ: objUserID, kind: betaKindIdent, data: d}
}

func bVar(name string) *bnode { return bNode('v', 0, bID(name)) }
func bLit(v *bnode) *bnode    { return bNode('t', 0, v) }

// bList chains items into cons cells.
func bList(items ...*bnode) *bnode {
	if len(items) == 0 {
		return &bnode{typ: objList}
	}
	return &bnode{typ: objList, kids: []*bnode{items[0], bList(items[1:]...)}}
}

// encodeBeta writes a FASD 0.95 file whose root, a vector of the given kind,
// holds body.
func encodeBeta(kind byte, body *bnode) []byte {
	out := []byte("FasdUAS 0.95")
	next := int16(-1)
	var emit func(n *bnode)
	emit = func(n *bnode) {
		for _, k := range n.kids {
			k.ref, next = next, next-1
		}
		var refs []byte
		for _, k := range n.kids {
			refs = binary.BigEndian.AppendUint16(refs, uint16(k.ref))
		}
		out = append(out, n.typ)
		out = binary.BigEndian.AppendUint16(out, uint16(n.ref))
		switch n.typ {
		case objFixnum:
			out = append(out, n.data...)
		case objCmdBlock:
			out = binary.BigEndian.AppendUint16(out, uint16(len(n.kids)))
			out = append(out, n.kind)
			out = binary.BigEndian.AppendUint16(out, n.flags)
			out = append(out, 0, 0, 0, 0)
			out = append(out, refs...)
		case objValueBlock2:
			out = binary.BigEndian.AppendUint16(out, uint16(len(n.kids)))
			out = append(out, n.kind)
			out = append(out, refs...)
		case objList:
			out = binary.BigEndian.AppendUint16(out, uint16(len(n.kids)))
			out = append(out, refs...)
		case objUserID, objCodeID:
			out = binary.BigEndian.AppendUint16(out, uint16(len(n.data)))
			out = append(out, n.kind)
			out = append(out, n.data...)
		default:
			out = binary.BigEndian.AppendUint16(out, uint16(len(n.data)))
			out = append(out, n.data...)
		}
		for _, k := range n.kids {
			emit(k)
		}
	}
	emit(&bnode{typ: objValueBlock2, kind: kind, kids: []*bnode{bNil(), body}})
	return append(out, "ascr\x00\x01\x00\x0c\xfa\xde\xde\xad"...)
}

// betaSample is a beta script that uses most of the beta's node layouts.
func betaSample() *bnode {
	const symTrue = 125
	return bNode('r', 0, bList(
		bNode('s', 3, bNil(), bStr(" a comment")),
		bNode('y', 0, bLit(bFix(3)), bVar("x")),
		bNode('`', 0,
			bNode('@', 1, bVar("x"), bLit(bFix(3))), // x is 3
			bNode('y', 0, bNode('i', 0, bNode('c', 0, bVar("x"), bLit(bFix(2))), bLit(bSym(14))), bVar("y")),
			bList(),
			bNode('y', 0, bLit(bSym(symTrue)), bVar("y"))),
		bNode('_', 0, bLit(bNil()), bID("i"), bLit(bFix(1)), bLit(bFix(10)), bNil()),
		bNode('W', 0,
			bNode('y', 0, bNode('N', 0, bID("add"), bNil(), bList(bVar("x"), bLit(bFix(2)))), bVar("z")),
			bID("m"), bList(),
			bNode('y', 0, bVar("m"), bVar("z"))),
		bNode('s', 5, bNil(), bNil()),
		bNode('p', 0, bID("add"),
			bNode('r', 0, bList(bNode('R', 0, bNode('a', 0, bVar("a"), bVar("b"))))),
			bNil(), bList(bID("a"), bID("b"))),
	))
}

// TestBeta decodes a synthetic AppleScript 1.0b1 beta script, whose syntax
// tree uses the beta's node letters and layouts.
func TestBeta(t *testing.T) {
	f, err := Parse(encodeBeta(17, betaSample()))
	if err != nil {
		t.Fatal(err)
	}
	want := `-- a comment
set x to 3
if x is 3 then
	set y to (x * 2) as string
else
	set y to true
end if
repeat with i from 1 to 10
end repeat
try
	set z to add(x, 2)
on error m
	set z to m
end try

on add(a, b)
	return a + b
end add
`
	if got := Decompile(f); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestBetaEarlyBuild checks the symbols of the earlier beta build, whose
// files have a kind 18 root: entries 123 and 124 are true and false.
func TestBetaEarlyBuild(t *testing.T) {
	body := bNode('r', 0, bList(
		bNode('y', 0, bLit(bSym(123)), bVar("a")),
		bNode('y', 0, bLit(bSym(124)), bVar("b")),
	))
	f, err := Parse(encodeBeta(18, body))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Decompile(f), "set a to true\nset b to false\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
