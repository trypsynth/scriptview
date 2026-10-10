package scpt

import (
	_ "embed"
	"encoding/binary"
	"strings"
	"unicode/utf8"
)

// Classic Mac OS kept text in the script system's encoding: Mac Roman for
// Western languages, and a Shift JIS variant for Japanese. A compiled script
// doesn't record which one, so a file whose old byte strings all read as
// Japanese is treated as Japanese.

// sjisTable maps each Shift JIS lead and trail byte pair to a UTF-16 code
// unit (0 if the pair doesn't decode). tools/gensjis makes it.
//
//go:embed sjis.bin
var sjisTable []byte

const sjisTrails = 0xFC - 0x40 // trail bytes 0x40-0xFC except 0x7F

func sjisLead(c byte) (int, bool) {
	switch {
	case c >= 0x81 && c <= 0x9F:
		return int(c - 0x81), true
	case c >= 0xE0 && c <= 0xFC:
		return int(c-0xE0) + 0x9F - 0x81 + 1, true
	}
	return 0, false
}

// decodeShiftJIS decodes Mac Japanese text, reporting false if b isn't
// valid Shift JIS.
func decodeShiftJIS(b []byte) (string, bool) {
	var sb strings.Builder
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c < 0x80:
			sb.WriteByte(c)
		case c >= 0xA1 && c <= 0xDF: // half-width katakana
			sb.WriteRune(rune(0xFF61 + int(c-0xA1)))
		case c == 0xA0:
			sb.WriteRune(' ')
		case c == 0xFD: // Mac Japanese adds these three
			sb.WriteRune('©')
		case c == 0xFE:
			sb.WriteRune('™')
		case c == 0xFF:
			sb.WriteRune('…')
		default:
			lead, ok := sjisLead(c)
			if !ok || i+1 >= len(b) || b[i+1] < 0x40 || b[i+1] > 0xFC || b[i+1] == 0x7F {
				return "", false
			}
			t := int(b[i+1]) - 0x40
			if b[i+1] > 0x7F {
				t--
			}
			k := 2 * (lead*sjisTrails + t)
			if k+2 > len(sjisTable) {
				return "", false
			}
			r := rune(binary.BigEndian.Uint16(sjisTable[k:]))
			if r == 0 {
				return "", false
			}
			sb.WriteRune(r)
			i++
		}
	}
	return sb.String(), true
}

// looksJapanese reports whether old byte strings are Shift JIS: every one
// with non-ASCII bytes decodes, and together they hold several kana or
// kanji.
func looksJapanese(texts [][]byte) bool {
	japanese := 0
	for _, t := range texts {
		if isASCII(t) {
			continue
		}
		s, ok := decodeShiftJIS(t)
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

func isASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

// legacyText decodes text that classic Mac OS stored in the script system's
// encoding.
func (dc *decompState) legacyText(b []byte) string {
	if dc.shiftJIS {
		if s, ok := decodeShiftJIS(b); ok {
			return s
		}
	}
	return macRoman(b)
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
