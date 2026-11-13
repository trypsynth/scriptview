package hypercard

import "strings"

// Indent indents a HyperTalk script the way HyperCard's script editor
// shows it. Stacks store scripts without indentation: the editor indents
// them each time it opens them. Each level is two spaces. Handlers,
// repeat loops, and if blocks open a level, and else lines up with its if.
// A line continued with ¬ is indented one level deeper than the line it
// continues.
func Indent(script string) string {
	lines := strings.Split(script, "\n")
	var out []string
	var blocks []string // "on", "repeat", or "if" for each open level
	level := func() int { return len(blocks) }
	pad := func(n int) string { return strings.Repeat("  ", max(n, 0)) }

	for i := 0; i < len(lines); i++ {
		// A statement is one line plus the lines it continues onto.
		stmt := []string{strings.TrimSpace(lines[i])}
		for strings.HasSuffix(code(stmt[len(stmt)-1]), "¬") && i+1 < len(lines) {
			i++
			stmt = append(stmt, strings.TrimSpace(lines[i]))
		}
		words := strings.Fields(strings.ToLower(code(strings.Join(stmt, " "))))
		first, last := "", ""
		if len(words) > 0 {
			first, last = words[0], words[len(words)-1]
		}
		at := level()
		switch {
		case first == "on" || first == "function":
			blocks = append(blocks, "on")
		case first == "end":
			// end if and end repeat close one block. end with a handler's
			// name closes the handler, and any block left open in it.
			kind := ""
			if len(words) > 1 {
				kind = words[1]
			}
			for len(blocks) > 0 {
				top := blocks[len(blocks)-1]
				blocks = blocks[:len(blocks)-1]
				if kind == "if" || kind == "repeat" || top == "on" {
					break
				}
			}
			at = level()
		case first == "repeat":
			blocks = append(blocks, "repeat")
		case first == "else":
			inBlock := len(blocks) > 0 && blocks[len(blocks)-1] == "if"
			if inBlock {
				at--
			}
			switch {
			case last == "else" || last == "then":
				// else, or else if … then: the block goes on, or a
				// single-line if gets a block for its else part.
				if !inBlock {
					blocks = append(blocks, "if")
				}
			case inBlock && !(len(words) > 1 && words[1] == "if"):
				// else with a statement ends the if.
				blocks = blocks[:len(blocks)-1]
			}
		case first == "if" && last == "then":
			blocks = append(blocks, "if")
		}
		for j, l := range stmt {
			if l == "" {
				out = append(out, "")
				continue
			}
			n := at
			if j > 0 {
				n++
			}
			out = append(out, pad(n)+l)
		}
	}
	return strings.Join(out, "\n")
}

// code returns a line without its comment and trailing space. HyperTalk
// strings have no escapes, so quotes simply toggle.
func code(line string) string {
	inString := false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '"':
			inString = !inString
		case !inString && strings.HasPrefix(line[i:], "--"):
			return strings.TrimSpace(line[:i])
		}
	}
	return strings.TrimSpace(line)
}
