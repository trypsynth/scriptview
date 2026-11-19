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
		refs := objRefs(l)
		for _, o := range l.Objects {
			add(kind+" "+refs[o]+" of "+name, o.Script)
		}
		for _, s := range l.Unmatched {
			add("script of an object of "+name+" (scriptview couldn't tell which)", s)
		}
	}
	add("project", p.Script)
	p.layers(func(w string, win *Window) { add(w, win.Script) }, layer)
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

// Text lists the text of every field in the project, each under a comment
// that names the field, such as -- background field "Name" of card "Home"
// of window "Main". A background field's text is listed for each card, and
// for the background too when it differs from every card's, since it is
// the text from whichever card was showing when the project was saved.
func (p *Project) Text() string {
	var sb strings.Builder
	add := func(label, text string) {
		if text == "" {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("-- " + label + "\n")
		sb.WriteString(strings.TrimRight(text, "\n") + "\n")
	}
	var cards []*Layer
	p.layers(func(_ string, w *Window) { cards = w.Cards }, func(kind, name string, l *Layer) {
		refs := objRefs(l)
		if kind == "card" {
			var bg map[*Object]string
			if l.Background != nil {
				bg = objRefs(l.Background)
			}
			for _, t := range l.Texts {
				add("background "+bg[t.Field]+" of "+name, t.Text)
			}
		}
		for _, o := range l.Objects {
			if kind == "background" && onCard(cards, l, o) {
				continue
			}
			add(kind+" "+refs[o]+" of "+name, o.Text)
		}
	})
	return sb.String()
}

// onCard reports whether a background field's text is a card's text in it.
func onCard(cards []*Layer, bg *Layer, o *Object) bool {
	for _, c := range cards {
		if c.Background != bg {
			continue
		}
		for _, t := range c.Texts {
			if t.Field == o && t.Text == o.Text {
				return true
			}
		}
	}
	return false
}

// layers calls win for each window, and layer for each background and card
// with its kind and its name, such as card "Home" of window "Main", then
// for the cards and backgrounds that no window lists.
func (p *Project) layers(win func(string, *Window), layer func(kind, name string, l *Layer)) {
	for i, w := range p.Windows {
		name := ref("window", w.Name, -1)
		if w.Name == "" {
			name = fmt.Sprintf("window %d", i+1)
		}
		win(name, w)
		for _, l := range w.Backgrounds {
			layer("background", ref("background", l.Name, l.ID)+" of "+name, l)
		}
		for _, l := range w.Cards {
			layer("card", ref("card", l.Name, l.ID)+" of "+name, l)
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
}

// objRefs names a layer's objects, such as button "OK". Objects that share
// a name are told apart by their IDs.
func objRefs(l *Layer) map[*Object]string {
	names := map[string]int{}
	for _, o := range l.Objects {
		names[o.Kind.String()+" "+o.Name]++
	}
	refs := map[*Object]string{}
	for _, o := range l.Objects {
		r := ref(o.Kind.String(), o.Name, o.ID)
		if o.Name != "" && names[o.Kind.String()+" "+o.Name] > 1 {
			r += fmt.Sprintf(" (id %d)", o.ID)
		}
		refs[o] = r
	}
	return refs
}
