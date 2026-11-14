package mactext

import (
	"embed"
	"encoding/binary"
	"strings"
	"sync"
)

//go:embed tables
var tables embed.FS

// An Encoding is one of classic Mac OS's text encodings. Its table, made by
// tools/genmactext from Apple's mapping files, loads when it's first used.
//
// Bytes 0x00 to 0x7F decode as ASCII in every encoding, although Mac
// Japanese fonts draw 0x5C as ¥: scripts use it as a backslash.
type Encoding struct {
	Name   string
	Script int // the Script Manager's script code
	file   string
	once   sync.Once
	single [128]string // bytes 0x80 to 0xFF
	pairs  map[uint16]string
	lead   [256]bool
}

// The encodings. Mac Roman is built in, and the Roman variants, such as
// Turkish, share its script code.
var (
	Roman           = &Encoding{Name: "Mac Roman"}
	Japanese        = &Encoding{Name: "Mac Japanese", Script: 1, file: "japanese"}
	TradChinese     = &Encoding{Name: "Mac Chinese Traditional", Script: 2, file: "chintrad"}
	Korean          = &Encoding{Name: "Mac Korean", Script: 3, file: "korean"}
	Arabic          = &Encoding{Name: "Mac Arabic", Script: 4, file: "arabic"}
	Hebrew          = &Encoding{Name: "Mac Hebrew", Script: 5, file: "hebrew"}
	Cyrillic        = &Encoding{Name: "Mac Cyrillic", Script: 7, file: "cyrillic"}
	Thai            = &Encoding{Name: "Mac Thai", Script: 21, file: "thai"}
	SimpChinese     = &Encoding{Name: "Mac Chinese Simplified", Script: 25, file: "chinsimp"}
	CentralEuropean = &Encoding{Name: "Mac Central European", Script: 29, file: "centeuro"}
	Greek           = &Encoding{Name: "Mac Greek", file: "greek"}
	Turkish         = &Encoding{Name: "Mac Turkish", file: "turkish"}
	Icelandic       = &Encoding{Name: "Mac Icelandic", file: "icelandic"}
	Croatian        = &Encoding{Name: "Mac Croatian", file: "croatian"}
	Romanian        = &Encoding{Name: "Mac Romanian", file: "romanian"}
	Farsi           = &Encoding{Name: "Mac Farsi", Script: 4, file: "farsi"}
)

var byScript = []*Encoding{Japanese, TradChinese, Korean, Arabic, Hebrew, Cyrillic, Thai, SimpChinese, CentralEuropean}

// ForScript returns the main encoding of a script code, or nil if Apple
// published none. Script 0 is Mac Roman.
func ForScript(code int) *Encoding {
	if code == 0 {
		return Roman
	}
	for _, e := range byScript {
		if e.Script == code {
			return e
		}
	}
	return nil
}

// Multibyte reports whether the encoding has double-byte characters.
func (e *Encoding) Multibyte() bool {
	e.load()
	return len(e.pairs) > 0
}

func (e *Encoding) load() {
	e.once.Do(func() {
		if e.file == "" {
			for i, r := range macRomanHigh {
				e.single[i] = string(r)
			}
			return
		}
		b, err := tables.ReadFile("tables/" + e.file + ".bin")
		if err != nil {
			panic(err)
		}
		for len(b) >= 3 {
			code, n := binary.BigEndian.Uint16(b), int(b[2])
			text := string(b[3 : 3+n])
			b = b[3+n:]
			if code < 0x100 {
				e.single[code-0x80] = text
				continue
			}
			if e.pairs == nil {
				e.pairs = map[uint16]string{}
			}
			e.pairs[code] = text
			e.lead[code>>8] = true
		}
	})
}

// Decode decodes b, and reports false if a byte or byte pair has no
// character in the encoding.
func (e *Encoding) Decode(b []byte) (string, bool) {
	return e.decode(b, true)
}

// decode decodes b. With roman set, it decodes a half-width katakana letter
// alone beside ASCII as Mac Roman.
func (e *Encoding) decode(b []byte, roman bool) (string, bool) {
	if e == Roman {
		return MacRoman(b), true
	}
	e.load()
	var sb strings.Builder
	prevASCII, prevKana := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c < 0x80 {
			sb.WriteByte(c)
			prevASCII, prevKana = isAlnum(rune(c)), false
			continue
		}
		wasASCII, wasKana := prevASCII, prevKana
		prevASCII, prevKana = false, false
		if e.lead[c] && i+1 < len(b) {
			if s, ok := e.pairs[uint16(c)<<8|uint16(b[i+1])]; ok {
				sb.WriteString(s)
				i++
				continue
			}
		}
		s := e.single[c-0x80]
		if s == "" {
			return "", false
		}
		if roman && e == Japanese && isKanaByte(c) {
			next := byte(0)
			if i+1 < len(b) {
				next = b[i+1]
			}
			if !wasKana && !isHalfwidth(next) && (wasASCII || isAlnum(rune(next))) {
				// A half-width katakana letter alone beside ASCII is
				// Mac Roman, as in "©1993" or "haven’t" in a Roman
				// font among Japanese text.
				sb.WriteRune(macRomanHigh[c-0x80])
				continue
			}
		}
		prevKana = isHalfwidth(c)
		sb.WriteString(s)
	}
	return sb.String(), true
}

// isKanaByte reports whether c is a half-width katakana letter in Shift
// JIS, from ｦ to ﾝ.
func isKanaByte(c byte) bool { return c >= 0xA6 && c <= 0xDD }

// isHalfwidth reports whether c is any half-width katakana character in
// Shift JIS, the letters with their punctuation and sound marks, as in ﾃﾞ.
func isHalfwidth(c byte) bool { return c >= 0xA1 && c <= 0xDF }

// DecodeOrRoman decodes b, or decodes it as Mac Roman if it doesn't decode.
func (e *Encoding) DecodeOrRoman(b []byte) string {
	if s, ok := e.Decode(b); ok {
		return s
	}
	return MacRoman(b)
}
