package hypercard

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"
)

// stackBuilder writes a small HyperCard 2.x stack.
type stackBuilder struct{ out []byte }

func (sb *stackBuilder) block(typ string, id int32, body []byte) {
	size := 16 + len(body)
	h := binary.BigEndian.AppendUint32(nil, uint32(size))
	h = append(h, typ...)
	h = binary.BigEndian.AppendUint32(h, uint32(id))
	h = append(h, 0, 0, 0, 0)
	sb.out = append(append(sb.out, h...), body...)
}

// part encodes a part record: size, ID, kind, flags, rectangle, style
// fields, then the name, a pad byte, and the script.
func part(id int, kind PartKind, name, script string) []byte {
	b := binary.BigEndian.AppendUint16(nil, 0)
	b = binary.BigEndian.AppendUint16(b, uint16(id))
	b = append(b, byte(kind), 0)
	b = append(b, make([]byte, 30-len(b))...)
	b = append(append(b, name...), 0, 0)
	if !strings.HasPrefix(script, "\x00") {
		script = strings.ReplaceAll(script, "\n", "\r")
	}
	b = append(append(b, script...), 0)
	if len(b)%2 == 1 {
		b = append(b, 0)
	}
	binary.BigEndian.PutUint16(b, uint16(len(b)))
	return b
}

// osaScript encodes a script in another language: a NUL byte, then the
// WOSA header and the data.
func osaScript(storage string, data []byte) string {
	h := []byte("\x00WOSA")
	h = binary.BigEndian.AppendUint32(h, 32)
	h = binary.BigEndian.AppendUint32(h, uint32(len(data)))
	h = append(h, "ascr"+storage...)
	h = append(h, make([]byte, 1+32-len(h))...)
	return string(append(h, data...))
}

// encodeLayer encodes the part and text lists, then the name and the script.
func encodeLayer(parts [][]byte, texts map[int16]string, name, script string) []byte {
	var pl []byte
	for _, p := range parts {
		pl = append(pl, p...)
	}
	var tl []byte
	for id, text := range texts {
		t := append([]byte{0}, text...)
		tl = binary.BigEndian.AppendUint16(tl, uint16(id))
		tl = binary.BigEndian.AppendUint16(tl, uint16(len(t)))
		tl = append(tl, t...)
		if len(t)%2 == 1 {
			tl = append(tl, 0)
		}
	}
	b := binary.BigEndian.AppendUint16(nil, uint16(len(parts)))
	b = binary.BigEndian.AppendUint16(b, 100)
	b = binary.BigEndian.AppendUint32(b, uint32(len(pl)))
	b = binary.BigEndian.AppendUint16(b, uint16(len(texts)))
	b = binary.BigEndian.AppendUint32(b, uint32(len(tl)))
	b = append(append(b, pl...), tl...)
	b = append(append(b, name...), 0)
	return append(append(b, strings.ReplaceAll(script, "\n", "\r")...), 0)
}

func sampleStack() []byte {
	be := binary.BigEndian
	var sb stackBuilder
	stak := make([]byte, 0x600-16)
	be.PutUint32(stak[0:], 10)   // version, at 16
	be.PutUint32(stak[20:], 1)   // backgrounds
	be.PutUint32(stak[24:], 200) // first background
	be.PutUint32(stak[28:], 2)   // cards
	be.PutUint32(stak[32:], 301) // first card
	be.PutUint32(stak[36:], 50)  // LIST
	stak = append(stak, "on openStack\rput 1 into x\rend openStack\x00"...)
	sb.block("STAK", -1, stak)

	list := make([]byte, 32)
	be.PutUint32(list[0:], 1) // pages
	be.PutUint16(list[12:], 8)
	list = be.AppendUint32(list, 60) // page ID
	list = be.AppendUint16(list, 2)  // cards on the page
	sb.block("LIST", 50, list)
	page := make([]byte, 8)
	for _, id := range []int32{302, 301} { // stack order differs from file order
		page = be.AppendUint32(page, uint32(id))
		page = append(page, 0, 0, 0, 0)
	}
	sb.block("PAGE", 60, page)

	bg := make([]byte, 20)
	be.PutUint32(bg[8:], 2)    // cards
	be.PutUint32(bg[12:], 200) // next background: itself
	be.PutUint32(bg[16:], 200)
	bg = append(bg, encodeLayer([][]byte{
		part(1, Field, "Title", ""),
		part(2, Button, "Next", "on mouseUp\ngo next card\nend mouseUp"),
	}, nil, "Main", "")...)
	sb.block("BKGD", 200, bg)

	for _, c := range []struct {
		id     int32
		name   string
		parts  [][]byte
		texts  map[int16]string
		script string
	}{
		{301, "First", nil, map[int16]string{1: "Hello", 2: "1"}, ""},
		{302, "", [][]byte{part(5, Button, "", "on mouseUp\nif x then\nbeep\nend if\nend mouseUp")}, map[int16]string{-5: "x"}, "on openCard\nend openCard"},
	} {
		card := make([]byte, 24)
		be.PutUint32(card[16:], 60)  // page
		be.PutUint32(card[20:], 200) // background
		card = append(card, encodeLayer(c.parts, c.texts, c.name, c.script)...)
		sb.block("CARD", c.id, card)
	}
	sb.block("TAIL", -1, nil)
	return sb.out
}

func TestParse(t *testing.T) {
	s, err := Parse(sampleStack())
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 10 || len(s.Backgrounds) != 1 || len(s.Cards) != 2 {
		t.Fatalf("got version %d, %d backgrounds, %d cards", s.Version, len(s.Backgrounds), len(s.Cards))
	}
	if s.Cards[0].ID != 302 || s.Cards[1].Name != "First" {
		t.Errorf("cards out of stack order: %d, %q", s.Cards[0].ID, s.Cards[1].Name)
	}
	first := s.Cards[1]
	if len(first.Text) != 1 || first.Text[0].Part == nil || first.Text[0].Part.Name != "Title" || first.Text[0].Text != "Hello" {
		t.Errorf("background field text: %+v", first.Text)
	}
	if len(first.Highlighted) != 1 || first.Highlighted[0].Name != "Next" {
		t.Errorf("background button highlight: %+v", first.Highlighted)
	}
	if _, err := Parse([]byte("FasdUAS 1.101.10")); err != ErrNotStack {
		t.Errorf("not a stack: got %v", err)
	}
}

func TestScripts(t *testing.T) {
	s, err := Parse(sampleStack())
	if err != nil {
		t.Fatal(err)
	}
	want := `-- stack
on openStack
  put 1 into x
end openStack

-- background button "Next" of background "Main"
on mouseUp
  go next card
end mouseUp

-- card id 302
on openCard
end openCard

-- card button id 5 of card id 302
on mouseUp
  if x then
    beep
  end if
end mouseUp
`
	if got := s.Scripts(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	wantText := `-- card button id 5 of card id 302
x

-- background field "Title" of card "First"
Hello
`
	if got := s.Text(); got != wantText {
		t.Errorf("text: got:\n%s\nwant:\n%s", got, wantText)
	}
}

func TestIndent(t *testing.T) {
	for in, want := range map[string]string{
		// A single-line if, then else with a block.
		"on f\nif a then b\nelse\nc\nend if\nend f": "on f\n  if a then b\n  else\n    c\n  end if\nend f",
		// else if chains, and else with a statement ending the block.
		"if a then\nb\nelse if c then\nd\nelse e": "if a then\n  b\nelse if c then\n  d\nelse e",
		// A continued line stays at its statement's level, and a comment
		// that ends in then opens nothing.
		"repeat with i = 1 to 3 -- count then\nput i ¬\ninto x\nend repeat": "repeat with i = 1 to 3 -- count then\n  put i ¬\n  into x\nend repeat",
		// An else cascade: each else belongs to the single-line if above it.
		"if a then x\nelse\nif b then y\nelse z\nend if": "if a then x\nelse\n  if b then y\n  else z\nend if",
		// then on its own line, after if or else if.
		"if a\nthen\nx\nelse if b\nthen\ny\nend if": "if a\nthen\n  x\nelse if b\nthen\n  y\nend if",
		// else repeat ends the if and opens a loop.
		"if a then\nx\nelse repeat 3\ny\nend repeat\nz": "if a then\n  x\nelse repeat 3\n  y\nend repeat\nz",
		// end with another name than the handler's closes nothing.
		"on f-- x\ny\nend f": "on f-- x\n  y\n  end f",
		// end with the handler's name closes a block left open.
		"on f\nif a then\nb\nend f\non g\nend g": "on f\n  if a then\n    b\nend f\non g\nend g",
		// then inside a string doesn't open a block.
		`put "if then" into x`: `put "if then" into x`,
	} {
		if got := Indent(in); got != want {
			t.Errorf("Indent(%q):\ngot:\n%s\nwant:\n%s", in, got, want)
		}
	}
}

// FuzzParse feeds Parse damaged stacks: it may fail, but not panic.
func FuzzParse(f *testing.F) {
	f.Add(sampleStack())
	f.Fuzz(func(t *testing.T, data []byte) {
		if s, err := Parse(data); err == nil {
			_ = s.Scripts()
			_ = s.Text()
		}
	})
}

// TestExternals lists the stack's externals before its scripts.
func TestExternals(t *testing.T) {
	s, err := Parse(sampleStack())
	if err != nil {
		t.Fatal(err)
	}
	s.Externals = []External{{"XCMD", "AESend"}, {"XFCN", "ShowList"}, {"XCMD", "Beep2"}}
	want := "-- external commands: AESend, Beep2\n-- external functions: ShowList\n\n-- stack\n"
	if got := s.Scripts(); !strings.HasPrefix(got, want) {
		t.Errorf("got:\n%s", got[:min(len(got), 120)])
	}
}

// TestAppleScript decompiles a part whose script is compiled AppleScript,
// and prints one whose AppleScript is stored as text.
func TestAppleScript(t *testing.T) {
	compiled, err := os.ReadFile("../../testdata/scpt/hello.scpt")
	if err != nil {
		t.Fatal(err)
	}
	var sb stackBuilder
	be := binary.BigEndian
	stak := make([]byte, 0x600-16)
	be.PutUint32(stak[0:], 10)
	be.PutUint32(stak[24:], 200)
	be.PutUint32(stak[36:], 50)
	sb.block("STAK", -1, append(stak, 0))
	bg := make([]byte, 20)
	be.PutUint32(bg[12:], 200)
	bg = append(bg, encodeLayer([][]byte{
		part(1, Button, "Compiled", osaScript("scpt", compiled)),
		part(2, Button, "Text", osaScript("TEXT", []byte("beep\rbeep"))),
	}, nil, "Main", "")...)
	sb.block("BKGD", 200, bg)
	sb.block("TAIL", -1, nil)

	s, err := Parse(sb.out)
	if err != nil {
		t.Fatal(err)
	}
	if p := s.Backgrounds[0].Parts[0]; p.Language != "AppleScript" || len(p.Compiled) != len(compiled) {
		t.Errorf("compiled part: language %q, %d bytes", p.Language, len(p.Compiled))
	}
	want := `-- background button "Compiled" of background "Main" (AppleScript)
display dialog "hello world"

-- background button "Text" of background "Main" (AppleScript)
beep
beep
`
	if got := s.Scripts(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestIIGS reads a synthetic HyperCard IIGS stack: little-endian, with
// HyperCard 1.x's card header, part names at offset 42, a 4-byte value
// before each script, and a color table on a card whose flag 0x0400 is set.
func TestIIGS(t *testing.T) {
	le := binary.LittleEndian
	var out []byte
	// block writes a block whose body starts at offset 12, after its ID.
	block := func(typ string, id int32, body []byte) {
		h := le.AppendUint32(nil, uint32(12+len(body)))
		h = append(h, typ...)
		h = le.AppendUint32(h, uint32(id))
		out = append(append(out, h...), body...)
	}
	at := func(n int) []byte { return make([]byte, n-12) } // a body up to offset n
	script := func(s string) []byte {
		return append(append([]byte{4, 0, 0, 0}, strings.ReplaceAll(s, "\n", "\r")...), 0)
	}
	stak := at(0x600)               // the 4 before the script sits at 0x600
	le.PutUint32(stak[16-12:], 5)   // version
	le.PutUint32(stak[40-12:], 200) // first background
	le.PutUint32(stak[52-12:], 50)  // LIST
	block("STAK", -1, append(stak, script("on openStack\nbeep\nend openStack")...))
	list := at(46)
	le.PutUint32(list, 1)         // pages, at 12
	le.PutUint16(list[24-12:], 8) // entry size
	le.PutUint32(list[40-12:], 60)
	le.PutUint16(list[44-12:], 1)
	block("LIST", 50, list)
	page := at(32)
	le.PutUint32(page[24-12:], 301)
	block("PAGE", 60, page)
	bg := at(50)
	le.PutUint32(bg[24-12:], 200) // next background: itself
	block("BKGD", 200, append(append(bg, "Main\x00"...), script("")...))

	part := make([]byte, 42)
	le.PutUint16(part[2:], 3)
	part[4] = 0x11 // a button, with IIGS bits above
	part = append(append(part, "Go\x00\x00"...), script("on mouseUp\ngo next\nend mouseUp")...)
	le.PutUint16(part, uint16(len(part)))
	text := append(le.AppendUint16(nil, 0xfffd), "Hi\x00"...) // card part 3
	card := at(36)
	le.PutUint16(card[16-12:], 0x0400) // flags: a color table follows the text
	le.PutUint32(card[32-12:], 200)    // background
	card = le.AppendUint16(card, 1)
	card = le.AppendUint16(card, 4)
	card = le.AppendUint32(card, uint32(len(part)))
	card = le.AppendUint16(card, 1)
	card = le.AppendUint32(card, uint32(len(text)))
	card = append(append(card, part...), text...)
	card = append(card, make([]byte, 32)...) // the color table
	block("CARD", 301, append(append(card, "Home\x00"...), script("")...))
	block("TAIL", -1, nil)

	s, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cards) != 1 || s.Cards[0].Name != "Home" || len(s.Cards[0].Parts) != 1 {
		t.Fatalf("got %d cards: %+v", len(s.Cards), s.Cards)
	}
	if p := s.Cards[0].Parts[0]; p.Kind != Button || p.Name != "Go" {
		t.Errorf("part: %+v", p)
	}
	want := `-- stack
on openStack
  beep
end openStack

-- card button "Go" of card "Home"
on mouseUp
  go next
end mouseUp
`
	if got := s.Scripts(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := s.Text(); got != "-- card button \"Go\" of card \"Home\"\nHi\n" {
		t.Errorf("text: %q", got)
	}
}
