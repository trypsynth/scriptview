// Package mactext decodes the text encodings of classic Mac OS: Mac Roman,
// and Mac Japanese, a variant of Shift JIS.
//
// Classic Mac OS stored text in the encoding of the script system that was
// running, and files don't record which one. Callers decide, for example
// with [ShiftJIS] failing on text that isn't Japanese.
package mactext

import "slices"

// macRomanHigh maps bytes 0x80 to 0xFF of Mac Roman to Unicode.
var macRomanHigh = []rune("ÄÅÇÉÑÖÜáàâäãåçéèêëíìîïñóòôöõúùûü†°¢£§•¶ß®©™´¨≠ÆØ∞±≤≥¥µ∂∑∏π∫ªºΩæø¿¡¬√ƒ≈∆«»… ÀÃÕŒœ–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ")

// MacRoman decodes Mac Roman bytes. Every byte has a character, so it
// can't fail.
func MacRoman(b []byte) string {
	out := make([]rune, len(b))
	for i, c := range b {
		if c < 0x80 {
			out[i] = rune(c)
		} else {
			out[i] = macRomanHigh[c-0x80]
		}
	}
	return string(out)
}

// EncodeMacRoman encodes s as Mac Roman, reporting false if s has a
// character that Mac Roman doesn't.
func EncodeMacRoman(s string) ([]byte, bool) {
	var out []byte
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		i := slices.Index(macRomanHigh, r)
		if i < 0 {
			return nil, false
		}
		out = append(out, byte(0x80+i))
	}
	return out, true
}

// IsASCII reports whether b has only 7-bit bytes, which read the same in
// every Mac encoding.
func IsASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return false
		}
	}
	return true
}
