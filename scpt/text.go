package scpt

// isDisplayable returns true if s looks like user-written text: printable,
// under 2000 bytes, and containing no Private Use Area or Specials codepoints
// (which appear as bytecode noise in decompiled .scpt files).
func isDisplayable(s string) bool {
	if len(s) > 2000 {
		return false
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		// C1 controls, Private Use Area, and Specials are bytecode noise.
		if (r >= 0x80 && r <= 0x9F) || (r >= 0xE000 && r <= 0xF8FF) || (r >= 0xFFF0 && r <= 0xFFFF) {
			return false
		}
	}
	return true
}

// macRomanHigh maps bytes 0x80–0xFF of Mac OS Roman to Unicode.
var macRomanHigh = []rune("ÄÅÇÉÑÖÜáàâäãåçéèêëíìîïñóòôöõúùûü†°¢£§•¶ß®©™´¨≠ÆØ∞±≤≥¥µ∂∑∏π∫ªºΩæø¿¡¬√ƒ≈∆«»…\u00a0ÀÃÕŒœ–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔ\uf8ffÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ")

// macRoman decodes Mac OS Roman bytes (used by identifiers and comments).
func macRoman(b []byte) string {
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
