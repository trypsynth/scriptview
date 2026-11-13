package hypercard

import (
	"fmt"
	"strings"

	"github.com/trypsynth/scriptview/internal/scpt"
)

// Scripts lists every script in the stack, indented, each under a comment
// that names its object the way HyperTalk refers to it, such as
// -- card button "OK" of card "Home". A script in another language says so
// in its comment, and compiled AppleScript is decompiled. Objects without a script are left
// out. When the stack has externals, two comments list them first.
func (s *Stack) Scripts() string {
	var sb strings.Builder
	add := func(label, script, language string, compiled []byte) {
		switch {
		case len(compiled) > 0 && language == "AppleScript":
			script = decompile(compiled)
		case len(compiled) > 0:
			script = fmt.Sprintf("-- (%d bytes of compiled %s)", len(compiled), language)
		case language == "":
			script = Indent(script)
		}
		if language != "" {
			label += " (" + language + ")"
		}
		if strings.TrimSpace(script) == "" {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("-- " + label + "\n")
		sb.WriteString(strings.TrimRight(script, "\n") + "\n")
	}
	var cmds, fns []string
	for _, e := range s.Externals {
		if e.Kind == "XFCN" {
			fns = append(fns, e.Name)
		} else {
			cmds = append(cmds, e.Name)
		}
	}
	if len(cmds) > 0 {
		sb.WriteString("-- external commands: " + strings.Join(cmds, ", ") + "\n")
	}
	if len(fns) > 0 {
		sb.WriteString("-- external functions: " + strings.Join(fns, ", ") + "\n")
	}
	add("stack", s.Script, s.Language, s.Compiled)
	for _, b := range s.Backgrounds {
		add(b.ref(), b.Script, b.Language, b.Compiled)
		for _, p := range b.Parts {
			add("background "+p.ref()+" of "+b.ref(), p.Script, p.Language, p.Compiled)
		}
	}
	for _, c := range s.Cards {
		add(c.ref(), c.Script, c.Language, c.Compiled)
		for _, p := range c.Parts {
			add("card "+p.ref()+" of "+c.ref(), p.Script, p.Language, p.Compiled)
		}
	}
	return sb.String()
}

// Text lists the text of every field, card by card, each under a comment
// that names the field, such as -- background field "Name" of card "Bob".
// Background fields that share their text across cards come first.
func (s *Stack) Text() string {
	var sb strings.Builder
	add := func(label, text string) {
		if text == "" {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("-- " + label + "\n")
		sb.WriteString(strings.TrimRight(strings.ReplaceAll(text, "\r", "\n"), "\n") + "\n")
	}
	for _, b := range s.Backgrounds {
		for _, t := range b.Text {
			add("background "+fieldRef(t.Part)+" of "+b.ref(), t.Text)
		}
	}
	for _, c := range s.Cards {
		for _, t := range c.Text {
			layer := "card "
			if t.Part != nil && c.Background != nil && findPart(c.Background.Parts, t.Part.ID) == t.Part {
				layer = "background "
			}
			add(layer+fieldRef(t.Part)+" of "+c.ref(), t.Text)
		}
	}
	return sb.String()
}

func (b *Background) ref() string { return ref("background", b.Name, int(b.ID)) }
func (c *Card) ref() string       { return ref("card", c.Name, int(c.ID)) }
func (p *Part) ref() string       { return ref(p.Kind.String(), p.Name, p.ID) }

func fieldRef(p *Part) string {
	if p == nil {
		return "field"
	}
	return p.ref()
}

// ref names an object by name, or by ID when it has none.
func ref(kind, name string, id int) string {
	if name == "" {
		return fmt.Sprintf("%s id %d", kind, id)
	}
	return kind + ` "` + name + `"`
}

// decompile prints compiled AppleScript data, or a comment when it doesn't
// read. The script ran inside HyperCard, so HyperCard's terms need no tell
// block.
func decompile(data []byte) string {
	f, err := scpt.ParseWith(data, scpt.Options{Classic: true, Host: "HyperCard"})
	if err != nil {
		return "-- (compiled script that doesn't read: " + err.Error() + ")"
	}
	return scpt.Decompile(f)
}
