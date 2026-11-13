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

// identTypeMarker is the first byte inside identifier data blocks.
const identTypeMarker = 0x30

var (
	ErrNotSCPT   = errors.New("not a compiled AppleScript file: missing FASD magic")
	ErrTruncated = errors.New("file appears truncated")
)
