package scpt

import "strings"

// contMarkByte stands for a `¬` line continuation inside a rendered
// expression, followed by one byte giving the continuation line's indent
// level; expandContinuations turns it into " ¬", a newline and the indent.
const contMarkByte = "\uFFFF" // a Unicode noncharacter: never valid in source text

// contMark returns a continuation marker for the current nesting depth.
func (dc *decompState) contMark() string {
	return contMarkByte + string(rune('0'+dc.contDepth+1))
}

// withBreak appends the ¬ continuation character, separated by a space
// except after a colon or opening bracket (`key:¬`, `{¬`) or on an empty line.
func withBreak(s string) string {
	s = strings.TrimRight(s, " ")
	if t := strings.TrimLeft(s, "\t"); t == "" || strings.ContainsAny(t[len(t)-1:], ":{[(") {
		return s + "¬"
	}
	return s + " ¬"
}

func expandContinuations(lines []string) []string {
	var out []string
	for _, line := range lines {
		if !strings.Contains(line, contMarkByte) {
			out = append(out, line)
			continue
		}
		base := line[:len(line)-len(strings.TrimLeft(line, "\t"))]
		parts := strings.Split(line, contMarkByte)
		out = append(out, withBreak(parts[0]))
		for i, p := range parts[1:] {
			level := 1
			if p != "" && p[0] >= '0' && p[0] <= '9'+40 {
				level = int(p[0] - '0')
				p = p[1:]
			}
			p = strings.TrimLeft(p, " ")
			if i < len(parts)-2 {
				p = withBreak(p)
			}
			out = append(out, base+strings.Repeat("\t", level)+p)
		}
	}
	return out
}
