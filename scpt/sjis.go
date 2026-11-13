package scpt

import (
	"encoding/binary"
	"strings"
	"unicode/utf8"

	"github.com/trypsynth/scriptview/mactext"
)

// Classic Mac OS kept text in the script system's encoding: Mac Roman for
// Western languages, and a Shift JIS variant for Japanese. A compiled script
// doesn't record which one, so a file whose old byte strings all read as
// Japanese is treated as Japanese.

// looksJapanese reports whether old byte strings are Shift JIS: every one
// with non-ASCII bytes decodes, and together they hold several kana or
// kanji.
func looksJapanese(texts [][]byte) bool {
	japanese := 0
	for _, t := range texts {
		if mactext.IsASCII(t) {
			continue
		}
		if mr := mactext.MacRoman(t); strings.Contains(mr, "«class ") || strings.Contains(mr, "«event ") ||
			strings.Contains(mr, "«constant ") || strings.Contains(mr, "«data ") {
			// AppleScript's raw-code chevrons in Mac Roman (-- folder = «class cfol»):
			// read as Shift JIS, «'s byte pairs up into kanji.
			return false
		}
		s, ok := mactext.ShiftJIS(t)
		if !ok {
			return false
		}
		for _, r := range s {
			if r >= 0x3040 && r <= 0x30FF || r >= 0x4E00 && r <= 0x9FFF {
				japanese++
			}
		}
	}
	return japanese >= 4
}

// legacyText decodes text that classic Mac OS stored in the script system's
// encoding.
func (dc *decompState) legacyText(b []byte) string {
	if dc.shiftJIS {
		if s, ok := mactext.ShiftJIS(b); ok {
			return s
		}
	}
	return mactext.MacRoman(b)
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
