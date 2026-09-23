package scpt

import (
	"fmt"
	"os"
	"strings"
)

// Decompile reconstructs AppleScript source from the FASD parse tree.
func Decompile(f *File) string {
	if f.dc == nil {
		return f.source
	}
	if f.RunOnly() {
		src, err := DecompileBytecode(f)
		if err != nil {
			return fmt.Sprintf("-- run-only script; bytecode decompilation failed: %v\n", err) + src
		}
		return "-- Decompiled from run-only bytecode: comments and original formatting are not recoverable.\n\n" + src
	}
	return decompileWith(f.dc, f.Handlers)
}

// Extract reads a .scpt file and returns a best-effort reconstruction of its source.
func Extract(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	return ExtractBytes(data)
}

// ExtractBytes works like Extract but operates on already-loaded file contents.
func ExtractBytes(data []byte) (string, error) {
	f, err := Parse(data)
	if err != nil {
		return "", err
	}
	return BestEffortSource(f), nil
}

// BestEffortSource generates a human-readable representation from parsed literals.
// It lists string literals, identifiers, and integers found in the bytecode.
func BestEffortSource(f *File) string {
	var sb strings.Builder
	sb.WriteString("-- Source reconstructed via bytecode extraction (not full decompilation)\n\n")

	if len(f.Handlers) > 0 {
		sb.WriteString("-- Handlers found:\n")
		for _, h := range f.Handlers {
			fmt.Fprintf(&sb, "--   on %s(...)\n", h.Name)
		}
		sb.WriteByte('\n')
	}

	if len(f.Idents) > 0 {
		sb.WriteString("-- Identifiers found:\n")
		for _, id := range f.Idents {
			fmt.Fprintf(&sb, "--   %s\n", id)
		}
		sb.WriteByte('\n')
	}

	if len(f.Strings) > 0 {
		sb.WriteString("-- String literals found:\n")
		for _, s := range f.Strings {
			if isDisplayable(s) {
				fmt.Fprintf(&sb, "--   %q\n", s)
			}
		}
		sb.WriteByte('\n')
	}

	if len(f.Ints) > 0 {
		sb.WriteString("-- Integer literals found:\n")
		for _, v := range f.Ints {
			fmt.Fprintf(&sb, "--   %d\n", v)
		}
	}

	return sb.String()
}

// Tree renders the raw node graph as an indented tree rooted at every node
// that no other node references. Intended for reverse-engineering the format.
func Tree(f *File) string {
	dc := f.dc
	if dc == nil {
		return "(no bytecode: this file holds plain source text)\n"
	}
	referenced := make(map[int16]bool)
	for _, n := range dc.nodes {
		for _, ch := range n.children {
			referenced[ch] = true
		}
	}
	for _, cell := range dc.paramMap {
		for _, ch := range cell {
			referenced[ch] = true
		}
	}
	var sb strings.Builder
	seen := make(map[int16]bool)
	var walk func(id int16, depth int)
	walk = func(id int16, depth int) {
		ind := strings.Repeat("  ", depth)
		if id <= 0 {
			fmt.Fprintf(&sb, "%s%d %s\n", ind, id, dc.describeRef(id))
			return
		}
		if seen[id] {
			fmt.Fprintf(&sb, "%s#%d (seen)\n", ind, id)
			return
		}
		seen[id] = true
		if n, ok := dc.nodes[id]; ok {
			fmt.Fprintf(&sb, "%s#%d '%c' flags=%#x pos=%d-%d %v\n", ind, id, n.typ, n.flags, n.start, n.end, n.children)
			for _, ch := range n.children {
				walk(ch, depth+1)
			}
			return
		}
		if cell, ok := dc.paramMap[id]; ok {
			fmt.Fprintf(&sb, "%s@%d list\n", ind, id)
			for _, ch := range cell {
				walk(ch, depth+1)
			}
			return
		}
		if app, ok := dc.appByID[id]; ok {
			fmt.Fprintf(&sb, "%s#%d app %q\n", ind, id, app)
			return
		}
		if c, ok := dc.comments[id]; ok {
			fmt.Fprintf(&sb, "%s#%d comment/roman %q\n", ind, id, c)
			return
		}
		if t, ok := dc.wrappedText(id); ok {
			fmt.Fprintf(&sb, "%s#%d text %q\n", ind, id, t)
			return
		}
		fmt.Fprintf(&sb, "%s#%d ?\n", ind, id)
	}
	for _, id := range dc.nodeOrder {
		if !referenced[id] {
			walk(id, 0)
		}
	}
	return sb.String()
}

func (dc *decompState) describeRef(id int16) string {
	if name, ok := dc.refName[id]; ok {
		return "name=" + name
	}
	if v, ok := dc.intByRef[id]; ok {
		return fmt.Sprintf("int=%d", v)
	}
	if code, ok := dc.osCode[id]; ok {
		return "ostype=" + dc.name(id) + " '" + code + "'"
	}
	if code, ok := dc.evCode[id]; ok {
		return "event=" + dc.name(id) + " '" + code + "'"
	}
	return ""
}

// RunOnly reports whether the script was saved run-only: it carries bytecode
// but no syntax tree.
func (f *File) RunOnly() bool {
	dc := f.dc
	if dc == nil || dc.objects == nil {
		return false
	}
	// The root's second item is the syntax tree; run-only scripts have none.
	// (Script objects inside closures can still carry fragments of one.)
	root, ok := dc.objects[0]
	if !ok || len(root.refs) < 2 {
		return false
	}
	tree, ok := dc.objects[root.refs[1]]
	return ok && tree.typ == objSymbol
}
