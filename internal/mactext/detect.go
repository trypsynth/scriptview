package mactext

import (
	"slices"
	"strings"
	"unicode"
)

// fontNames are the ASCII names of fonts for each non-Roman script, for
// fonts whose IDs don't show their script. A name that starts with one of
// these, after any leading "." or "*", matches.
var fontNames = map[*Encoding][]string{
	Japanese:    {"Osaka", "Heisei", "Narita", "Shizuoka", "Fukuroi", "Chiyoda", "Mito", "Narashino", "Yaesu", "Sumpu", "Kyoto"},
	SimpChinese: {"Beijing", "Song", "Hei", "Kai", "Fang Song"},
	TradChinese: {"Taipei", "Apple LiSung", "Apple LiGothic", "LiSung", "LiGothic"},
	Korean:      {"Seoul", "Munjo", "AppleGothic", "AppleMyungjo"},
	Arabic:      {"Baghdad", "Geeza", "Nadeem", "Kufi"},
	Thai:        {"Ayuthaya", "Krungthep", "Thonburi", "Sathu"},
}

// ForFont returns the encoding of the text in a font, given the font's ID
// and its name's bytes, or nil if they don't show it. IDs from 16384 to
// 32767 belong to non-Roman scripts, 512 to a script, but utilities that
// renumbered fonts also put Roman fonts there, so an ID counts only with a
// name that's either known or not ASCII and decodes in the ID's script.
func ForFont(id int, name []byte) *Encoding {
	n := strings.TrimLeft(string(name), ".*")
	for e, prefixes := range fontNames {
		for _, p := range prefixes {
			if strings.HasPrefix(n, p) && (len(n) == len(p) || !unicode.IsLetter(rune(n[len(p)]))) {
				return e
			}
		}
	}
	if strings.HasSuffix(n, " CE") {
		return CentralEuropean
	}
	if strings.HasSuffix(n, " CY") {
		return Cyrillic
	}
	if id < 16384 || id > 32767 {
		return nil
	}
	e := ForScript((id-16384)/512 + 1)
	if e == nil || IsASCII(name) {
		return nil
	}
	if _, ok := e.Decode(name); !ok {
		return nil
	}
	return e
}

// Detect picks the encoding of a file's strings. hints are the encodings
// of the file's fonts, if it records them, as [ForFont] gives them.
//
// Without a hint, only Japanese is detected, from its kana. A hint makes
// its encoding the candidate, which still needs evidence in the text:
// Roman fonts often sit beside a stray non-Roman one. Strings that don't
// decode in the chosen encoding are left to the caller, to read as Mac
// Roman.
func Detect(texts [][]byte, hints ...*Encoding) *Encoding {
	cands := []*Encoding{Japanese}
	for _, h := range hints {
		if h != nil && h != Roman && !slices.Contains(cands, h) {
			cands = append(cands, h)
		}
	}
	best, bestScore := Roman, 0
	for _, e := range cands {
		hinted := slices.Contains(hints, e)
		if s := score(e, texts, hinted); s > bestScore {
			best, bestScore = e, s
		}
	}
	return best
}

// score counts the characters in texts that show they're in e, or returns
// 0 if the texts don't look like e.
func score(e *Encoding, texts [][]byte, hinted bool) int {
	if !e.Multibyte() {
		return scoreSingle(texts, hinted && e != CentralEuropean)
	}
	strong, against, fails, nonASCII := 0, 0, 0, 0
	for _, t := range texts {
		if IsASCII(t) {
			continue
		}
		nonASCII++
		s, ok := e.Decode(t)
		if !ok {
			fails++
			continue
		}
		rs := []rune(s)
		for i, r := range rs {
			if r >= 0x4E00 && r <= 0x9FFF && i > 0 && i+1 < len(rs) && isLetter(rs[i-1]) && isLetter(rs[i+1]) {
				// An ideograph inside an ASCII word is a Mac Roman accented
				// letter paired with the next byte, as in "Ger閣etechnik"
				// for "Gerätetechnik".
				against++
				continue
			}
			switch {
			case isHalfwidthLetter(r):
				// Mac Roman's curly quotes and dashes are half-width
				// katakana in Shift JIS, alone and touching ASCII letters,
				// as in "havenﾕt", while real ones come in words.
				// HyperTalk's ¬ at the end of a line, byte 0xC2, is ﾂ in
				// Shift JIS, and Japanese scripts use it too.
				lone := (i == 0 || !isHalfwidthLetter(rs[i-1])) && (i+1 == len(rs) || !isHalfwidthLetter(rs[i+1]))
				continuation := r == 'ﾂ' && (i+1 == len(rs) || rs[i+1] == '\r')
				if lone && !continuation && (i > 0 && isAlnum(rs[i-1]) || i+1 < len(rs) && isAlnum(rs[i+1])) {
					against++
				}
			case e == Japanese && r >= 0x3041 && r <= 0x30FF:
				strong++
			case e == Korean && r >= 0xAC00 && r <= 0xD7A3:
				strong++
			case e != Japanese && e != Korean && r >= 0x4E00 && r <= 0x9FFF &&
				(i > 0 && rs[i-1] >= 0x3000 || i+1 < len(rs) && rs[i+1] >= 0x3000):
				// Ideographs next to other wide characters: Mac Roman's
				// accented letters make lone ones.
				strong++
			}
		}
	}
	// A font hint lifts the limit on strings that don't decode: they're
	// often in a Roman font beside the others.
	minStrong, maxFail := 4, nonASCII/10
	if hinted {
		minStrong, maxFail = 2, nonASCII
	}
	if strong < minStrong || strong <= 2*against || fails > maxFail {
		return 0
	}
	return strong
}

// isHalfwidthLetter reports whether r is a half-width katakana letter, not
// punctuation such as ｡ and ｢, which Japanese text uses on its own.
func isHalfwidthLetter(r rune) bool { return r >= 0xFF66 && r <= 0xFF9D }

func isLetter(r rune) bool { return r < 0x80 && unicode.IsLetter(r) }

func isAlnum(r rune) bool { return r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r)) }

// scoreSingle scores a single-byte encoding, which every byte decodes in,
// by its runs of three or more letters above 0x7F: Cyrillic, Greek,
// Hebrew, and Arabic words are made of them, while Mac Roman text has its
// accented letters one or two at a time. Only a hint makes the encoding a
// candidate.
func scoreSingle(texts [][]byte, hinted bool) int {
	if !hinted {
		return 0
	}
	runs, lone := 0, 0
	for _, t := range texts {
		n := 0
		for i := 0; i <= len(t); i++ {
			if i < len(t) && t[i] >= 0x80 {
				n++
				continue
			}
			switch {
			case n >= 3:
				runs++
			case n > 0:
				lone++
			}
			n = 0
		}
	}
	if runs < 2 || runs <= lone {
		return 0
	}
	return runs
}
