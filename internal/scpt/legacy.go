package scpt

import "encoding/binary"

// AppleScript 1.0 (FASD versions 0.98 and 1.00) stored many built-in terms
// as symbols: an index into a table inside the AppleScript runtime, tagged as
// index*8+1. Apple's table is gone from current macOS, which refuses these
// files, so these entries were worked out from how old scripts use them.
var legacySymbols = map[uint32]fasCode{
	2:   {kind: osKindCode, code: "list"},
	10:  {kind: osKindCode, code: "pcls"}, // class
	14:  {kind: osKindCode, code: "TEXT"}, // string
	15:  {kind: osKindCode, code: "scpt"}, // script
	20:  {kind: osKindCode, code: "obj "}, // reference
	49:  {kind: osKindCode, code: "cobj"}, // item
	50:  {kind: osKindCode, code: "citm"}, // text item
	51:  {kind: osKindCode, code: "fss "}, // file specification
	52:  {kind: osKindCode, code: "alis"},
	95:  {kind: osKindCode, code: "leng"},
	96:  {kind: osKindCode, code: "pcnt"}, // contents
	98:  {kind: osKindCode, code: "to  "}, // handler label
	99:  {kind: osKindCode, code: "of  "}, // handler label
	103: {kind: osKindCode, code: "errn"}, // error … number
	109: {kind: osKindCode, code: "ret "}, // return
	117: {kind: osKindCode, code: "rest"},
	121: {kind: osKindConstant, enum: "boov", code: "fals"},
	122: {kind: osKindConstant, enum: "boov", code: "true"},
	132: {kind: osKindCode, code: "cha "}, // character
	133: {kind: osKindCode, code: "txdl"}, // text item delimiters
	135: {kind: osKindCode, code: "ascr"}, // AppleScript
}

// legacySymbol decodes an AppleScript 1.0 symbol payload.
func legacySymbol(data []byte) (fasCode, bool) {
	if len(data) != 4 {
		return fasCode{}, false
	}
	v := binary.BigEndian.Uint32(data)
	if v&7 != 1 {
		return fasCode{}, false
	}
	c, ok := legacySymbols[v>>3]
	return c, ok
}
