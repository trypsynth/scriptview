package supercard

import (
	"fmt"
	"strings"

	"github.com/trypsynth/scriptview/internal/hypercard"
)

// Scripts lists every script in the project, indented, each under a
// comment that names its object the way SuperTalk refers to it, such as
// -- card button "OK" of card "Home" of window "Main". Objects without a
// script are left out.
func (p *Project) Scripts() string {
	var sb strings.Builder
	add := func(label, script string) {
		if strings.TrimSpace(script) == "" {
			return
		}
		if script != Tokenized {
			script = hypercard.Indent(script)
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("-- " + label + "\n")
		sb.WriteString(strings.TrimRight(script, "\n") + "\n")
	}
	layer := func(kind, name string, l *Layer) {
		add(name, l.Script)
		// Objects that share a name are told apart by their IDs.
		names := map[string]int{}
		for _, o := range l.Objects {
			names[o.Kind.String()+" "+o.Name]++
		}
		for _, o := range l.Objects {
			obj := ref(o.Kind.String(), o.Name, o.ID)
			if o.Name != "" && names[o.Kind.String()+" "+o.Name] > 1 {
				obj += fmt.Sprintf(" (id %d)", o.ID)
			}
			add(kind+" "+obj+" of "+name, o.Script)
		}
		for _, s := range l.Unmatched {
			add("script of an object of "+name+" (scriptview couldn't tell which)", s)
		}
	}
	add("project", p.Script)
	for i, w := range p.Windows {
		win := ref("window", w.Name, -1)
		if w.Name == "" {
			win = fmt.Sprintf("window %d", i+1)
		}
		add(win, w.Script)
		for _, l := range w.Backgrounds {
			layer("background", ref("background", l.Name, l.ID)+" of "+win, l)
		}
		for _, l := range w.Cards {
			layer("card", ref("card", l.Name, l.ID)+" of "+win, l)
		}
	}
	// A card or background that no window lists has no ID number.
	for i, u := range p.Unlisted {
		name := ref(u.Kind, u.Name, -1)
		if u.Name == "" {
			name = fmt.Sprintf("%s %d that no window lists", u.Kind, i+1)
		}
		layer(u.Kind, name, u.Layer)
	}
	for i, m := range p.Menus {
		menu := ref("menu", m.Name, -1)
		if m.Name == "" {
			menu = fmt.Sprintf("menu %d", i+1)
		}
		add(menu, m.Script)
		for j, it := range m.Items {
			item := ref("menuItem", it.Name, -1)
			if it.Name == "" {
				item = fmt.Sprintf("menuItem %d", j+1)
			}
			add(item+" of "+menu, it.Script)
		}
	}
	return sb.String()
}

// ref names an object by name, or by ID when it has none.
func ref(kind, name string, id int) string {
	if name == "" {
		return fmt.Sprintf("%s id %d", kind, id)
	}
	return kind + ` "` + name + `"`
}
