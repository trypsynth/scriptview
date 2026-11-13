package scpt

import (
	"fmt"
	"strings"
)

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
