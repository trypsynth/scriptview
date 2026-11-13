package mactext

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
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

// Detect picks the encoding of a file's strings. fonts are the encodings
// of the file's fonts, as [ForFont] gives them, or nil for a file that
// doesn't list its fonts.
//
// Japanese is always a candidate, since its kana show it. A font makes its
// encoding a candidate too, which still needs evidence in the text: Roman
// fonts often sit beside a stray non-Roman one. When a file lists its
// fonts, they're the only other candidates. When it doesn't, the encoding
// whose reading of the text fits clearly better than Mac Roman's wins.
// Strings that don't decode in the chosen encoding are left to the caller,
// to read as Mac Roman.
func Detect(texts [][]byte, fonts []*Encoding) *Encoding {
	hints := fonts
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
	if best == Roman && fonts == nil {
		best = bestUnhinted(texts)
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

// typographic are the characters above 0x7F that sit inside or beside
// words in Western text.
const typographic = "’‘“”–—·…•™®©°«»¬±¥£¢§¶"

// verdicts judges each character above 0x7F in text decoded as e: 1 if
// it fits the encoding, -1 if it doesn't, and 0 if it shows nothing.
// Letters fit in words, and in an encoding with Latin letters, alone or a
// few at a time. Symbols don't fit touching a letter.
func verdicts(e *Encoding, rs []rune) []int8 {
	v := make([]int8, len(rs))
	high := func(i int) bool { return i >= 0 && i < len(rs) && rs[i] >= 0x80 }
	letter := func(i int) bool { return i >= 0 && i < len(rs) && unicode.IsLetter(rs[i]) }
	for i, r := range rs {
		if r < 0x80 {
			continue
		}
		// The run of characters above 0x7F around r, unless it repeats one
		// character, as in "•••", or is punctuation, as in "”…".
		start, end := i, i+1
		for high(start - 1) {
			start--
		}
		for high(end) {
			end++
		}
		odd, same := false, true
		for _, c := range rs[start:end] {
			odd = odd || c != r && !strings.ContainsRune(typographic, c)
			same = same && c == r
		}
		// Mac Roman's ﬁ and ﬂ ligatures are Turkish's ş and Ş, and rare
		// in Western text.
		isLetter := unicode.IsLetter(r) && r != 'ﬁ' && r != 'ﬂ'
		switch {
		case same && end-start >= 2:
			// A repeated character, as in "•••", is decoration.
		case e.latin() && odd && (end-start >= 4 || end-start == 3 && !allLetters(rs[start:end])):
			// Latin words have their accented letters one or two at a
			// time, or three, as in Turkish "küçük".
			v[i] = -1
		case isLetter && (letter(i-1) || letter(i+1)):
			switch {
			case letter(i-1) && mixedScripts(rs[i-1], r), letter(i+1) && mixedScripts(r, rs[i+1]):
				v[i] = -1
			case letter(i-1) && unicode.IsLower(rs[i-1]) && unicode.IsUpper(r):
				v[i] = -1 // a capital inside a word, as in "DeŘiŖtir"
			case unicode.IsLower(r) && i >= 2 && unicode.IsUpper(rs[i-1]) && unicode.IsUpper(rs[i-2]):
				v[i] = -1 // a small letter after capitals, as in "РЁТи"
			case !letter(i-1) && strings.ContainsRune("ъьы", r):
				v[i] = -1 // no Russian word starts with these
			default:
				v[i] = 1
			}
		case !isLetter && (letter(i-1) || letter(i+1)):
			if strings.ContainsRune(typographic, r) {
				v[i] = 1 // as in "haven’t" and "word–word"
			} else {
				v[i] = -1
			}
		}
	}
	return v
}

func allLetters(rs []rune) bool {
	for _, r := range rs {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// scoringRunes decodes t in a single-byte encoding for plausibility, with
// U+FFFC for a byte above 0x7F that decodes as ASCII punctuation: Arabic
// and Hebrew have right-to-left copies of ASCII punctuation, which are
// symbols, and of the space and digits, which aren't.
func (e *Encoding) scoringRunes(t []byte) []rune {
	if e == Roman {
		return []rune(MacRoman(t))
	}
	e.load()
	rs := make([]rune, len(t))
	for i, c := range t {
		switch {
		case c < 0x80:
			rs[i] = rune(c)
		case e.single[c-0x80] == "":
			rs[i] = utf8.RuneError
		default:
			rs[i] = []rune(e.single[c-0x80])[0]
			if rs[i] < 0x80 && rs[i] != ' ' && !unicode.IsDigit(rs[i]) {
				rs[i] = '\uFFFC'
			}
		}
	}
	return rs
}

// mixedScripts reports whether two letters, at least one above 0x7F, are
// from different alphabets, as in a Latin letter inside a Cyrillic word.
func mixedScripts(a, b rune) bool {
	for _, t := range []*unicode.RangeTable{unicode.Latin, unicode.Cyrillic, unicode.Greek, unicode.Hebrew, unicode.Arabic, unicode.Thai} {
		if unicode.Is(t, a) != unicode.Is(t, b) {
			return true
		}
	}
	return false
}

// latin reports whether e is Mac Roman or one of the encodings that, like
// it, write Latin letters above 0x7F.
func (e *Encoding) latin() bool {
	switch e {
	case Roman, CentralEuropean, Turkish, Icelandic, Croatian, Romanian:
		return true
	}
	return false
}

// unhinted are the encodings that bestUnhinted weighs, in order of
// preference when they tie. Farsi is Arabic with Persian digits. Thai,
// whose letters fill most of the high bytes, would win over Korean, and
// Greek over Western text with symbols such as π.
var unhinted = []*Encoding{Japanese, Korean, SimpChinese, TradChinese, Cyrillic, CentralEuropean, Turkish, Hebrew, Arabic, Icelandic, Croatian, Romanian}

// bestUnhinted picks the encoding whose reading of texts fits best. Mac
// Roman wins unless another fits clearly better.
func bestUnhinted(texts [][]byte) *Encoding {
	best, bestNet := Roman, max(evidence(Roman, texts), 0)+romanMargin
	for _, e := range unhinted {
		if !e.Multibyte() && !beatsRoman(e, texts) {
			continue
		}
		if net := evidence(e, texts); net > bestNet {
			best, bestNet = e, net
		}
	}
	return best
}

// beatsRoman reports whether a single-byte encoding fits texts better than
// Mac Roman where their readings differ: by romanMargin, and on most of
// the characters. Western text often has a stray symbol that another
// encoding reads as a letter, but not many.
func beatsRoman(e *Encoding, texts [][]byte) bool {
	adv, n := 0, 0
	for _, t := range texts {
		if IsASCII(t) || isData(t) {
			continue
		}
		er, rr := e.scoringRunes(t), Roman.scoringRunes(t)
		ev, rv := verdicts(e, er), verdicts(Roman, rr)
		for i := range er {
			if er[i] != rr[i] {
				n++
				adv += int(ev[i]) - int(rv[i])
			}
		}
	}
	return adv >= romanMargin && 2*adv >= n
}

// isData reports whether t holds control characters other than tabs and
// line breaks, as binary data kept in a string does.
func isData(t []byte) bool {
	for _, c := range t {
		if c < 0x20 && c != '\t' && c != '\r' && c != '\n' {
			return true
		}
	}
	return false
}

func sum(v []int8) int {
	n := 0
	for _, x := range v {
		n += int(x)
	}
	return n
}

// romanMargin is how much better than Mac Roman another encoding must fit.
const romanMargin = 3

// evidence weighs how well texts fit e: the characters that fit it, less
// those that don't, with a double-byte character counting twice.
func evidence(e *Encoding, texts [][]byte) int {
	net := 0
	for _, t := range texts {
		if IsASCII(t) || isData(t) {
			continue
		}
		if !e.Multibyte() {
			net += sum(verdicts(e, e.scoringRunes(t)))
			continue
		}
		cs := e.decodeCodes(t)
		wide := func(i int) bool { return i >= 0 && i < len(cs) && cs[i].code > 0xFF }
		for i, c := range cs {
			switch {
			case c.r < 0x80:
			case i > 0 && cs[i-1].r == c.r || i+1 < len(cs) && cs[i+1].r == c.r:
				// A repeated character, as in "∞∞∞", is decoration.
			case c.code > 0xFF && c.code&0xFF < 0x80 && i > 0 && isLetter(cs[i-1].r):
				// A pair whose second byte is ASCII, after a letter: a
				// Mac Roman quote and the next letter, as in "file’s".
				net -= 2
			case c.r == utf8.RuneError:
				net-- // a byte that doesn't decode
			case isHalfwidthLetter(c.r) && !wide(i-1) && !wide(i+1):
				net--
			case c.r >= 0x4E00 && c.r <= 0x9FFF && i > 0 && i+1 < len(cs) && isLetter(cs[i-1].r) && isLetter(cs[i+1].r):
				net -= 2
			case (e == SimpChinese || e == TradChinese) && wide(i) && i+2 < len(cs) && cs[i+1].r == ' ' && wide(i+2):
				net -= 2 // Chinese doesn't put spaces between words; Korean does
			case e == Japanese && c.code>>8 >= 0x88 && c.code>>8 <= 0x98 && (wide(i-1) || wide(i+1)):
				net++ // the common kanji, which Mac Roman's accented letters also make
			case e == Japanese && (c.r >= 0x3041 && c.r <= 0x30FF || c.r == 0x3000),
				e == Korean && c.r >= 0xAC00 && c.r <= 0xD7A3,
				e == SimpChinese && c.code>>8 >= 0xB0 && c.code>>8 <= 0xD7 && (wide(i-1) || wide(i+1)),
				e == TradChinese && c.code>>8 >= 0xA4 && c.code>>8 <= 0xC6 && (wide(i-1) || wide(i+1)):
				// Kana and the ideographic space, hangul, and the common
				// hanzi: GB 2312's and Big5's first levels.
				net += 2
			}
		}
	}
	return net
}

// A coded is a decoded character with its code: a byte, or a byte pair
// above 0xFF.
type coded struct {
	r    rune
	code uint16
}

// decodeCodes decodes b like Decode, keeping each character's code, with
// U+FFFD for a byte that doesn't decode. Of an entry that maps to several
// characters, it keeps the first.
func (e *Encoding) decodeCodes(b []byte) []coded {
	e.load()
	var out []coded
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c < 0x80 {
			out = append(out, coded{rune(c), uint16(c)})
			continue
		}
		if e.lead[c] && i+1 < len(b) {
			code := uint16(c)<<8 | uint16(b[i+1])
			if s, ok := e.pairs[code]; ok {
				out = append(out, coded{[]rune(s)[0], code})
				i++
				continue
			}
		}
		r := utf8.RuneError
		if s := e.single[c-0x80]; s != "" {
			r = []rune(s)[0]
		}
		out = append(out, coded{r, uint16(c)})
	}
	return out
}
