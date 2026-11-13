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
