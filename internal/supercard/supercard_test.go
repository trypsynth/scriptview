package supercard

import (
	"encoding/binary"
	"slices"
	"strings"
	"testing"

	"github.com/trypsynth/scriptview/internal/mactext"
)

var bo = binary.BigEndian

// builder writes a small SuperCard project, in format 1.0 or 2.1.
type builder struct {
	l      layout
	major  uint16
	minor  uint16
	blocks map[int][]byte // the bytes from 12, the kind first
	handle uint32
}

func newBuilder(major, minor uint16) *builder {
	b := &builder{major: major, minor: minor, blocks: map[int][]byte{}, handle: 0x00120000}
	switch {
	case major == 2:
		b.l = layout2
	case minor == 0:
		b.l = layout10
	default:
		b.l = layout11
	}
	return b
}

// rec encodes a record: its length, then data, padded to an even length.
func rec(data []byte) []byte {
	out := bo.AppendUint32(nil, uint32(len(data)))
	out = append(out, data...)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// script encodes a script record's data: the text, a NUL, and 22 bytes of
// editor state ending in its window's rectangle.
func script(text string) []byte {
	b := append([]byte(strings.ReplaceAll(text, "\n", "\r")), 0)
	b = append(b, 0, 4, 0, 9, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0)
	return append(b, 0, 40, 0, 20, 1, 66, 1, 236)
}

// head returns a block's bytes up to off: its kind, then zeros.
func head(kind, off int) []byte {
	b := make([]byte, off-12)
	bo.PutUint16(b, uint16(kind))
	return b
}

// object is an object for a builder's card or background.
type object struct {
	typ, id      int
	name, script string
	data         []byte
	props        bool // a user properties record, from SuperCard 3 on
}

// objects encodes an object list.
func (b *builder) objects(objs []object) []byte {
	var recs, ents []byte
	for _, o := range objs {
		r := make([]byte, b.l.object)
		bo.PutUint16(r, uint16(o.typ))
		bo.PutUint16(r[8:], uint16(o.id))
		put := func(at int, ok bool) {
			if ok {
				b.handle += 4
				bo.PutUint32(r[at:], b.handle)
			}
		}
		put(62, o.data != nil)
		put(58, o.name != "")
		put(66, o.script != "")
		put(86, o.props)
		if o.data != nil {
			ents = append(ents, rec(o.data)...)
		}
		if o.name != "" {
			ents = append(ents, rec([]byte(o.name))...)
		}
		if o.script != "" {
			ents = append(ents, rec(script(o.script))...)
		}
		if o.props {
			ents = append(ents, rec([]byte("\x00\x01\x00\x00\x00\x08myProp\x00"))...)
		}
		recs = append(recs, r...)
	}
	return bo.AppendUint16(append(recs, ents...), uint16(len(objs)))
}

// layer adds a card or background block.
func (b *builder) layer(id, kind int, name, scr string, objs []object) {
	off := b.l.card
	if kind == kindBackground {
		off = b.l.background
	}
	d := append(head(kind, off), rec(script(scr))...)
	d = append(d, rec(b.objects(objs))...)
	if kind == kindCard {
		d = append(d, rec(nil)...) // the text of the background's fields
	}
	b.blocks[id] = append(d, rec([]byte(name))...)
}

// window adds a window block with one card list, and the list.
func (b *builder) window(id, list int, name, scr string, cards [][4]int) {
	d := head(kindWindow, b.l.window-8)
	d = bo.AppendUint16(d, 1)
	d = bo.AppendUint16(d, uint16(len(cards)))
	d = bo.AppendUint32(d, uint32(list))
	d = append(d, rec(script(scr))...)
	b.blocks[id] = append(d, rec([]byte(name))...)
	l := head(kindCardList, 14)
	for _, c := range cards {
		e := make([]byte, 64)
		for i, v := range c {
			bo.PutUint32(e[4*i:], uint32(v))
		}
		l = append(l, e...)
	}
	b.blocks[list] = l
}

// menu adds a menu block.
func (b *builder) menu(id int, name, scr string, items [][2]string) {
	var recs, ents []byte
	for _, it := range items {
		r := make([]byte, b.l.item)
		b.handle += 4
		bo.PutUint32(r[18:], b.handle)
		name, _ := mactext.EncodeMacRoman(it[0])
		ents = append(ents, rec(name)...)
		if it[1] != "" {
			b.handle += 4
			bo.PutUint32(r[22:], b.handle)
			ents = append(ents, rec(script(it[1]))...)
		}
		recs = append(recs, r...)
	}
	list := bo.AppendUint16(append(recs, ents...), uint16(len(items)))
	d := append(head(kindMenu, b.l.menu), rec(script(scr))...)
	d = append(d, rec(list)...)
	b.blocks[id] = append(d, rec([]byte(name))...)
}

// bytes writes the project: the header, the block table, and the blocks.
func (b *builder) bytes() []byte {
	out := make([]byte, 0x600)
	bo.PutUint16(out, b.major)
	bo.PutUint16(out[2:], b.minor)
	bo.PutUint32(out[16:], 128)
	var ids []int
	for id := range b.blocks {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		d := b.blocks[id]
		bo.PutUint32(out[0x400+4*id:], uint32(len(out)))
		h := bo.AppendUint32(nil, uint32(id))
		h = bo.AppendUint32(h, uint32(len(d)))
		size := (12 + len(d) + 63) / 64 * 64
		h = bo.AppendUint32(h, uint32(size))
		blk := append(h, d...)
		out = append(out, append(blk, make([]byte, size-len(blk))...)...)
	}
	bo.PutUint32(out[4:], uint32(len(out)))
	bo.PutUint32(out[20:], uint32(ids[len(ids)-1]+1))
	return out
}

// sample builds a project with a window, a background, two cards, and a
// menu, and a group on the first card.
func sample(major, minor uint16) *builder {
	b := newBuilder(major, minor)
	b.blocks[0] = append(head(kindProject, b.l.project), rec(script("on openProject\nbeep\nend openProject"))...)
	b.window(5, 6, "Main", "on openWindow\nput 1 into x\nend openWindow", [][4]int{{8, 7, 101, 100}, {9, 7, 102, 100}})
	b.layer(7, kindBackground, "", "", []object{
		{typ: 200, id: 1, name: "Next", script: "on mouseUp\ngo next card\nend mouseUp"},
	})
	group := newBuilder(major, minor)
	group.handle = 0x00130000
	b.layer(8, kindCard, "Home", "on openCard\nif x then\nbeep\nend if\nend openCard", []object{
		{typ: 300, id: 2, data: []byte("\x00\x00Hello\x00"), name: "Notes", props: major == 2},
		{typ: 105, id: 3, data: []byte{1, 2, 3}},
		{typ: 108, id: 4, name: "Group", data: group.objects([]object{
			{typ: 200, id: 5, script: "on mouseUp\nanswer \"in a group\"\nend mouseUp"},
		})},
		{typ: 201, id: 6, script: "on mouseUp\nend mouseUp"},
	})
	b.layer(9, kindCard, "", "", nil)
	b.menu(10, "File", "", [][2]string{{"Open…", "on itemSelect\nopen file\nend itemSelect"}, {"-", ""}})
	return b
}

const sampleScripts = `-- project
on openProject
  beep
end openProject

-- window "Main"
on openWindow
  put 1 into x
end openWindow

-- background button "Next" of background id 100 of window "Main"
on mouseUp
  go next card
end mouseUp

-- card "Home" of window "Main"
on openCard
  if x then
    beep
  end if
end openCard

-- card button id 5 of card "Home" of window "Main"
on mouseUp
  answer "in a group"
end mouseUp

-- card button id 6 of card "Home" of window "Main"
on mouseUp
end mouseUp

-- menuItem "Open…" of menu "File"
on itemSelect
  open file
end itemSelect
`

func TestScripts(t *testing.T) {
	for _, v := range [][2]uint16{{1, 0}, {1, 1}, {2, 1}} {
		data := sample(v[0], v[1]).bytes()
		if !IsProject(data) {
			t.Fatalf("%d.%d: not a project", v[0], v[1])
		}
		p, err := Parse(data)
		if err != nil {
			t.Fatalf("%d.%d: %v", v[0], v[1], err)
		}
		if got := p.Scripts(); got != sampleScripts {
			t.Errorf("%d.%d: got\n%s\nwant\n%s", v[0], v[1], got, sampleScripts)
		}
		if w := p.Windows[0]; len(w.Cards) != 2 || w.Cards[1].ID != 102 || w.Cards[1].Background != w.Backgrounds[0] {
			t.Errorf("%d.%d: cards %+v", v[0], v[1], w.Cards)
		}
		if objs := p.Windows[0].Cards[0].Objects; len(objs) != 5 || objs[0].Name != "Notes" || objs[0].Kind != Field || objs[2].Name != "Group" {
			t.Errorf("%d.%d: objects %+v", v[0], v[1], objs)
		}
	}
}

// TestStandalone reads a PowerPC standalone's data fork, where the
// application's code sits between the project's first 0x400 bytes and the
// rest.
func TestStandalone(t *testing.T) {
	proj := sample(2, 1).bytes()
	code := append([]byte("Joy!peffpwpc"), make([]byte, 500)...)
	data := append(append(append([]byte{}, proj[:0x400]...), code...), proj[0x400:]...)
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Scripts(); got != sampleScripts {
		t.Errorf("got\n%s", got)
	}
}

// TestProtected reads protected scripts, XORed with 0xFD in whole or in
// the text before the last carriage return, and tokenized ones.
func TestProtected(t *testing.T) {
	xor := func(b []byte) []byte {
		out := make([]byte, len(b))
		for i, c := range b {
			out[i] = c ^ 0xFD
		}
		return out
	}
	ps := &parser{enc: nil}
	plain := script("on mouseUp\nbeep\nend mouseUp\n")
	whole := append(append([]byte{}, protected...), xor(plain)...)
	k := strings.IndexByte(string(plain), 0)
	part := append(append(append([]byte{}, protected...), xor(plain[:k-1])...), plain[k-1:]...)
	for _, r := range [][]byte{whole, part} {
		if !isScript(r) {
			t.Errorf("isScript(%q) = false", r)
		}
		if got := ps.scriptRecord(r); got != "on mouseUp\nbeep\nend mouseUp\n" {
			t.Errorf("scriptRecord = %q", got)
		}
	}
	if got := ps.scriptRecord(append(append([]byte{}, tokenized...), 0, 0, 0, 1)); got != Tokenized {
		t.Errorf("tokenized: %q", got)
	}
}

func TestIsProject(t *testing.T) {
	data := sample(1, 0).bytes()
	for _, bad := range [][]byte{
		nil,
		[]byte("hello"),
		data[:len(data)-64],               // the length doesn't match
		append(data, 0),                   // nor here
		append([]byte{0, 3}, data[2:]...), // an unknown format
	} {
		if IsProject(bad) {
			t.Errorf("IsProject(%d bytes) = true", len(bad))
		}
	}
	if _, err := Parse([]byte("hello")); err != ErrNotProject {
		t.Errorf("Parse: %v", err)
	}
}

// TestUnmatched lists the scripts of objects whose records don't fit
// their handles, without naming the objects.
func TestUnmatched(t *testing.T) {
	b := newBuilder(1, 0)
	b.blocks[0] = head(kindProject, b.l.project+4)
	b.window(5, 6, "W", "", [][4]int{{8, 7, 101, 100}})
	b.layer(7, kindBackground, "", "", nil)
	b.layer(8, kindCard, "", "", []object{{typ: 200, id: 1, script: "on mouseUp\nbeep\nend mouseUp"}})
	// Clear the button's script handle, so its script record is left over.
	d := b.blocks[8]
	at := b.l.card - 12
	at = next(at, int(bo.Uint32(d[at:])))
	bo.PutUint32(d[at+4+66:], 0) // after the list's length
	got, _ := Parse(b.bytes())
	want := "-- script of an object of card id 101 of window \"W\" (scriptview couldn't tell which)\non mouseUp\n  beep\nend mouseUp\n"
	if s := got.Scripts(); s != want {
		t.Errorf("got\n%s\nwant\n%s", s, want)
	}
}
