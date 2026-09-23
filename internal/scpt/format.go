// Package scpt parses Apple compiled AppleScript (.scpt) files in the
// FASD (FasdUAS) binary format and reconstructs their source text.
package scpt

import "errors"

// Magic bytes at the start of every FASD file written since AppleScript 1.1.
var Magic = []byte("FasdUAS 1.101.10")

// fasdHeaderLen returns the length of a FASD header: "FasdUAS " and a
// four-byte version such as "1.00", then from version 1.10 on a second
// version ("FasdUAS 1.101.10"). It returns 0 if data is not FASD.
func fasdHeaderLen(data []byte) int {
	isVersion := func(v []byte) bool {
		return len(v) == 4 && isDigit(v[0]) && v[1] == '.' && isDigit(v[2]) && isDigit(v[3])
	}
	if len(data) < 12 || string(data[:8]) != "FasdUAS " || !isVersion(data[8:12]) {
		return 0
	}
	if string(data[8:12]) < "1.10" {
		return 12
	}
	if len(data) < 16 || !isVersion(data[12:16]) {
		return 0
	}
	return 16
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// Trailer marks the end of a FASD file: "ascr" + version(2) + size(2) + 0xFADEDEAD.
var Trailer = []byte{0x61, 0x73, 0x63, 0x72, 0x00, 0x01, 0x00, 0x0c, 0xfa, 0xde, 0xde, 0xad}

// OpcodeText is the text-literal opcode: 0x11 + node_id(2) + byte_len(2) + UTF-16BE data.
const OpcodeText = 0x11

// OpcodeIdent is the identifier opcode: 0x0b + ref(2) + data_len(2) + identifier-data.
const OpcodeIdent = 0x0b

// OpcodeInt is the integer-literal opcode: 0x03 + ref(2) + value(2).
const OpcodeInt = 0x03

// OpcodeNode is the node-descriptor opcode: 0x0d + node_id(2) + field(2) + type(1) + data.
const OpcodeNode = 0x0d

// OpcodeParam is the parameter-list opcode: 0x02 + ref(2) + count(2) + count×2 child refs.
// The ref value matches the positive child ref in an 'I' signature node's second slot.
const OpcodeParam = 0x02

// OpcodeRecordCell is a record-literal cell, laid out like OpcodeParam with
// three children: key ref, value node, next cell (or negative terminator).
const OpcodeRecordCell = 0x06

// OpcodeComment is a source comment: 0x0c + id(2) + len(2) + textLen(2) + text + 4-byte trailer.
// A UTF-16 copy of the same text follows as a 0x0e/0x11 pair.
const OpcodeComment = 0x0c

// OpcodeOSType is the four-byte OSType reference opcode:
// 0x0a + ref(2) + len(1) + 4-byte-FourCC + ...
const OpcodeOSType = 0x0a

// OpcodeLong is a 32-bit integer literal: 0x07 + ref(2, negative) + len(2)=4 + int32.
const OpcodeLong = 0x07

// OpcodeReal is a real literal: 0x08 + node_id(2) + len(2)=8 + IEEE-754 double.
const OpcodeReal = 0x08

// OpcodeBlob is an opaque data record: 0x0f + node_id(2) + byte_len(2) + data.
// Application specifiers store a (nested) alias record here.
const OpcodeBlob = 0x0f

// identTypeMarker is the first byte inside identifier data blocks.
const identTypeMarker = 0x30

var (
	ErrNotSCPT   = errors.New("not a compiled AppleScript file: missing FASD magic")
	ErrTruncated = errors.New("file appears truncated")
)
