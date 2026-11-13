package scpt

import (
	"encoding/binary"
	"strings"
	"unicode/utf8"

	"github.com/trypsynth/scriptview/internal/mactext"
)

// Classic Mac OS kept text in the script system's encoding: Mac Roman for
// Western languages, and others such as Mac Japanese. A compiled script
// doesn't record which one, so mactext.Detect picks it from the text.

// legacyEncoding returns the encoding of a file's old byte strings.
func legacyEncoding(texts [][]byte) *mactext.Encoding {
	for _, t := range texts {
		if mr := mactext.MacRoman(t); strings.Contains(mr, "«class ") || strings.Contains(mr, "«event ") ||
			strings.Contains(mr, "«constant ") || strings.Contains(mr, "«data ") {
			// AppleScript's raw-code chevrons in Mac Roman (-- folder = «class cfol»):
			// read as Shift JIS, «'s byte pairs up into kanji.
			return mactext.Roman
		}
	}
	return mactext.Detect(texts)
}

// legacyText decodes text that classic Mac OS stored in the script system's
// encoding.
func (dc *decompState) legacyText(b []byte) string {
	if dc.enc == nil {
		return mactext.MacRoman(b)
	}
	return dc.enc.DecodeOrRoman(b)
}

// identNames decodes an identifier record's names: UTF-8 in current files,
// the script system's encoding in old ones.
func (dc *decompState) identNames(data []byte) []string {
	var names []string
	for i := 0; i+2 <= len(data); {
		n := int(binary.BigEndian.Uint16(data[i:]))
		i += 2
		if i+n > len(data) {
			break
		}
		if raw := data[i : i+n]; utf8.Valid(raw) {
			names = append(names, string(raw))
		} else {
			names = append(names, dc.legacyText(raw))
		}
		i += n
	}
	return names
}
