package scpt

import (
	"encoding/binary"
	"fmt"
	"io"
)

// A compiled script body is a serialized object graph. Every record starts
// with a type byte, a reference number and a size field:
//
//	type(1) ref(2) size(2) payload…
//
// References link objects together: a negative reference names an object
// stored inline (it is the next object in the stream when first used) and a
// non-negative one names a shared object that may be referenced repeatedly.
// Objects are decoded depth-first in the order they are first referenced, so
// the stream is simply every object in that order.
//
// Object types (the names follow AppleScript's own loader):
const (
	objSymbol           = 0x01 // symbol: size!=0 → 8-byte value, else nil
	objList             = 0x02 // cons cell: size refs, [head, tail]
	objFixnum           = 0x03 // small integer: the value is the size field
	objValueBlock       = 0x04 // typed vector: kind(1) then size refs
	objBinding          = 0x06 // record/labeled-parameter cell: [key, value, next]
	objLong             = 0x07 // 32-bit integer
	objFloat            = 0x08 // double
	objBool             = 0x09 // boolean: the value is the size field
	objCodeID           = 0x0a // term: kind(1) then a class/constant/event code
	objUserID           = 0x0b // identifier: kind(1)=0x30 then (len, name)×2
	objString           = 0x0c // Mac Roman string pair (comments, old literals)
	objCmdBlock         = 0x0d // syntax tree node: kind(1) flags(2) codeStart(2) codeEnd(2) refs
	objValueBlock2      = 0x0e // typed vector: kind(1) then size refs
	objDataBlock        = 0x0f // descriptor: kind(1) then size bytes
	objPointerBlock     = 0x10 // untyped vector of size refs
	objDataBlockRaw     = 0x11 // untyped bytes (UTF-16 text, bytecode)
	objLongDataBlock    = 0x12 // descriptor with 32-bit length
	objLongDataBlockRaw = 0x13 // untyped bytes with 32-bit length
)

// object is one decoded record.
type object struct {
	typ  byte
	ref  int16
	size uint16 // raw size field (the value itself for fixnums and booleans)
	kind byte   // kind byte, for types that have one
	// For cmdBlock: flags, code start and code end.
	flags, start, end uint16
	refs              []int16 // child references
	data              []byte  // payload bytes
}

// readObjects decodes the body as a sequence of objects, returning them in
// stream order.
func readObjects(body []byte) ([]*object, error) {
	r := newReader(body)
	var out []*object
	for !r.eof() {
		o := &object{}
		var err error
		if o.typ, err = r.readByte(); err != nil {
			return nil, err
		}
		if r.remaining() < 4 {
			return nil, fmt.Errorf("truncated object %#x at %d", o.typ, r.offset())
		}
		o.ref, _ = r.readI16BE()
		o.size, _ = r.readU16BE()
		n := int(o.size)
		refs := func(count int) error {
			if r.remaining() < 2*count {
				return io.ErrUnexpectedEOF
			}
			o.refs = make([]int16, count)
			for i := range o.refs {
				o.refs[i], _ = r.readI16BE()
			}
			return nil
		}
		switch o.typ {
		case objSymbol, objList, objBinding, objPointerBlock:
			if o.typ == objSymbol {
				// A symbol with a non-zero size carries an 8-byte value.
				if n != 0 {
					if o.data, err = r.readBytes(8); err != nil {
						return nil, err
					}
				}
				break
			}
			err = refs(n)
		case objFixnum, objBool:
			// The value is the size field; no payload.
		case objLong, objFloat, objString, objDataBlockRaw:
			o.data, err = r.readBytes(n)
		case objCodeID, objUserID, objDataBlock:
			if o.kind, err = r.readByte(); err == nil {
				o.data, err = r.readBytes(n)
			}
		case objCmdBlock:
			if o.kind, err = r.readByte(); err != nil {
				return nil, err
			}
			hdr, err := r.readBytes(6)
			if err != nil {
				return nil, err
			}
			o.flags = binary.BigEndian.Uint16(hdr)
			o.start = binary.BigEndian.Uint16(hdr[2:])
			o.end = binary.BigEndian.Uint16(hdr[4:])
			err = refs(n)
		case objValueBlock, objValueBlock2:
			if o.kind, err = r.readByte(); err == nil {
				err = refs(n)
			}
		case objLongDataBlock:
			if o.kind, err = r.readByte(); err != nil {
				return nil, err
			}
			fallthrough
		case objLongDataBlockRaw:
			var lenb []byte
			if lenb, err = r.readBytes(4); err == nil {
				o.data, err = r.readBytes(int(binary.BigEndian.Uint32(lenb)))
			}
		default:
			return nil, fmt.Errorf("unknown record %#x at %d", o.typ, r.offset()-5)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
