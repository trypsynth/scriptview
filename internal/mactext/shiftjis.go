package mactext

import (
	_ "embed"
	"encoding/binary"
	"strings"
)

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

// ShiftJIS decodes Mac Japanese text, reporting false if b isn't
// valid Shift JIS.
func ShiftJIS(b []byte) (string, bool) {
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

// LooksJapanese reports whether texts are Mac Japanese rather than Mac
// Roman: every one with non-ASCII bytes decodes as Shift JIS, and together
// they hold several kana. An accented Mac Roman letter such as é is a Shift
// JIS lead byte, so Western text sometimes decodes, but then as stray kanji
// with no kana: "Frédéric" reads as "Fr仕屍ic".
func LooksJapanese(texts [][]byte) bool {
	kana := 0
	for _, t := range texts {
		if IsASCII(t) {
			continue
		}
		s, ok := ShiftJIS(t)
		if !ok {
			return false
		}
		for _, r := range s {
			if r >= 0x3040 && r <= 0x30FF || r >= 0xFF66 && r <= 0xFF9D {
				kana++ // hiragana, katakana, half-width katakana
			}
		}
	}
	return kana >= 4
}
