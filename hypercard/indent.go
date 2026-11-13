package hypercard

import "strings"

// Indent indents a HyperTalk script the way HyperCard shows it. Stacks
// store scripts without indentation: HyperCard indents a script each time
// it shows it. Each level is two spaces. Handlers, repeat loops, and if
// blocks open a level, and else lines up with its if. A line continued
// with ¬ stays at the level of the line it continues.
func Indent(script string) string {
	lines := strings.Split(script, "\n")
	var out []string
	var blocks []string // "on", "repeat", or "if" for each open level
	var names []string  // the name of each open handler, in order
	pad := func(n int) string { return strings.Repeat("  ", max(n, 0)) }
	// singleIf is set after if … then x on one line, whose else, on the
	// next line, belongs to it. pendingIf is set after if without then,
	// whose then comes on the next line.
	singleIf, pendingIf, pendingElseIf := false, false, false

	for i := 0; i < len(lines); i++ {
		// A statement is one line plus the lines it continues onto.
		stmt := []string{strings.TrimSpace(lines[i])}
		for strings.HasSuffix(code(stmt[len(stmt)-1]), "¬") && i+1 < len(lines) {
			i++
			stmt = append(stmt, strings.TrimSpace(lines[i]))
		}
		words := strings.Fields(strings.ToLower(code(strings.Join(stmt, " "))))
		if len(words) == 0 {
			// A blank or comment-only line, at the current level.
			for _, l := range stmt {
				if l == "" {
					out = append(out, "")
				} else {
					out = append(out, pad(len(blocks))+l)
				}
			}
			continue
		}
		first, last := words[0], words[len(words)-1]
		at := len(blocks)
		wasSingleIf, wasPendingIf, wasPendingElseIf := singleIf, pendingIf, pendingElseIf
		singleIf, pendingIf, pendingElseIf = false, false, false
		hasThen := false
		for _, w := range words {
			if w == "then" {
				hasThen = true
			}
		}
		switch {
		case first == "on" || first == "function":
			// A handler starts at the left margin, closing anything an
			// earlier handler left open.
			blocks, names, at = blocks[:0], names[:0], 0
			blocks = append(blocks, "on")
			// HyperCard takes the name as written: in on f-- x, it's f--.
			name := ""
			if raw := strings.Fields(strings.ToLower(stmt[0])); len(raw) > 1 {
				name = raw[1]
			}
			names = append(names, name)
		case first == "end":
			// end if and end repeat close one block. end with the open
			// handler's name closes the handler, and any block left open in
			// it. end with another name closes nothing.
			kind := ""
			if len(words) > 1 {
				kind = words[1]
			}
			if kind != "if" && kind != "repeat" && (len(names) == 0 || names[len(names)-1] != kind) {
				break
			}
			for len(blocks) > 0 {
				top := blocks[len(blocks)-1]
				blocks = blocks[:len(blocks)-1]
				if top == "on" {
					names = names[:len(names)-1]
				}
				if kind == "if" || kind == "repeat" || top == "on" {
					break
				}
			}
			at = len(blocks)
		case first == "repeat":
			blocks = append(blocks, "repeat")
		case first == "then" && wasPendingElseIf:
			// else if c, then then on the next line: it lines up with the
			// else, and the if block goes on.
			at--
		case first == "then" && wasPendingIf:
			// if c on one line, then on the next: then lines up with the if.
			if last == "then" {
				blocks = append(blocks, "if")
			} else {
				singleIf = true
			}
		case first == "else":
			elseIf := len(words) > 1 && words[1] == "if"
			if !wasSingleIf && len(blocks) > 0 && blocks[len(blocks)-1] == "if" {
				// The else of the enclosing if block.
				at--
				if last != "else" && last != "then" && !elseIf {
					blocks = blocks[:len(blocks)-1] // else with a statement ends it
				}
				if len(words) > 1 && words[1] == "repeat" {
					blocks = append(blocks, "repeat") // else repeat …
				}
			} else if last == "else" || last == "then" {
				// The else of a single-line if, opening a block.
				blocks = append(blocks, "if")
			}
			if elseIf && last != "then" && hasThen {
				singleIf = true
			} else if elseIf && !hasThen {
				if at < len(blocks) {
					pendingElseIf = true // in an if block
				} else {
					pendingIf = true
				}
			}
		case first == "if":
			switch {
			case last == "then":
				blocks = append(blocks, "if")
			case hasThen:
				singleIf = true
			default:
				pendingIf = true
			}
		}
		for _, l := range stmt {
			if l == "" {
				out = append(out, "")
				continue
			}
			out = append(out, pad(at)+l)
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
