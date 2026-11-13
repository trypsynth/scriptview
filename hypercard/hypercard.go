// Package hypercard reads HyperCard stacks: their scripts, buttons, fields,
// and the text in the fields.
//
// A stack file is a chain of blocks. Each block starts with its size, a
// four-character type, and an ID. The STAK block holds the stack's own
// settings and script, BKGD and CARD blocks hold the backgrounds and cards
// with their parts, and LIST and PAGE blocks give the order of the cards.
// Other blocks hold pictures, fonts, styles, and free space, which this
// package skips.
package hypercard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/trypsynth/scriptview/classic"
	"github.com/trypsynth/scriptview/mactext"
)

// ErrNotStack is returned for data that isn't a HyperCard stack.
var ErrNotStack = errors.New("not a HyperCard stack")

// IsStack reports whether data starts like a HyperCard stack.
func IsStack(data []byte) bool {
	return len(data) >= 16 && string(data[4:8]) == "STAK"
}

// Stack is a HyperCard stack.
type Stack struct {
	// Version is the stack format: 8 and 9 for HyperCard 1.x, 10 for 2.x.
	Version     int
	Script      string
	Backgrounds []*Background // in the order of the stack's background chain
	Cards       []*Card       // in stack order
	// Externals lists the stack's external commands (XCMD) and functions
	// (XFCN), compiled code in its resource fork that scripts call by name.
	// AddResources fills it in.
	Externals []External
}

// External is an external command or function.
type External struct {
	Kind string // "XCMD" for a command, "XFCN" for a function
	Name string
}

// AddResources reads what the stack keeps in its resource fork: the names
// of its external commands and functions.
func (s *Stack) AddResources(rsrc []byte) {
	for _, r := range classic.ResourceList(rsrc) {
		if (r.Type == "XCMD" || r.Type == "XFCN") && r.Name != "" {
			s.Externals = append(s.Externals, External{Kind: r.Type, Name: r.Name})
		}
	}
}

// Background is a background, which cards share.
type Background struct {
	ID     int32
	Name   string
	Script string
	Parts  []*Part
	// Text holds the text of background fields that share their text
	// across cards.
	Text []Text
}

// Card is a card.
type Card struct {
	ID         int32
	Name       string
	Script     string
	Background *Background
	Parts      []*Part
	// Text holds the text of the card's own fields, and of its background's
	// fields on this card.
	Text []Text
	// Highlighted holds the background buttons that are highlighted on this
	// card, for buttons whose highlight isn't shared across cards.
	Highlighted []*Part
}

// PartKind says whether a part is a button or a field.
type PartKind int

// The part kinds.
const (
	Button PartKind = 1
	Field  PartKind = 2
)

func (k PartKind) String() string {
	switch k {
	case Button:
		return "button"
	case Field:
		return "field"
	}
	return fmt.Sprintf("part kind %d", int(k))
}

// Part is a button or a field.
type Part struct {
	ID     int
	Kind   PartKind
	Name   string
	Script string
}

// Text is the text of one field.
type Text struct {
	Part *Part // the field, or nil if no part has the ID
	Text string
}

type block struct {
	typ  string
	id   int32
	data []byte // the whole block, header included
}

var be = binary.BigEndian

// Parse reads a stack.
func Parse(data []byte) (*Stack, error) {
	if !IsStack(data) {
		return nil, ErrNotStack
	}
	blocks := map[string]map[int32]block{}
	var order []block
	for p := 0; p+12 <= len(data); {
		size := int(be.Uint32(data[p:]))
		if size < 12 || p+size > len(data) {
			break
		}
		b := block{typ: string(data[p+4 : p+8]), id: int32(be.Uint32(data[p+8:])), data: data[p : p+size]}
		if blocks[b.typ] == nil {
			blocks[b.typ] = map[int32]block{}
		}
		blocks[b.typ][b.id] = b
		order = append(order, b)
		p += size
		if b.typ == "TAIL" {
			break
		}
	}
	if len(order) == 0 || order[0].typ != "STAK" {
		return nil, ErrNotStack
	}
	stak := order[0].data
	if len(stak) < 56 {
		return nil, fmt.Errorf("hypercard: STAK block too short")
	}
	s := &Stack{Version: int(be.Uint32(stak[16:]))}
	if len(stak) > 0x600 {
		s.Script = cString(stak[0x600:])
	}

	// HyperCard 1.x blocks lack a field before the bitmap ID, so the rest
	// of a background's or card's header comes 4 bytes earlier.
	shift := 0
	if s.Version < 10 {
		shift = -4
	}
	bkgds := map[int32]*Background{}
	for _, b := range backgroundChain(blocks["BKGD"], int32(be.Uint32(stak[40:])), shift) {
		bg := parseBackground(b.data, shift)
		bkgds[bg.ID] = bg
		s.Backgrounds = append(s.Backgrounds, bg)
	}
	for _, id := range cardOrder(blocks, int32(be.Uint32(stak[52:])), shift) {
		b, ok := blocks["CARD"][id]
		if !ok {
			continue
		}
		s.Cards = append(s.Cards, parseCard(b.data, bkgds, shift))
	}
	if len(s.Cards) == 0 {
		// Without a usable card list, take the cards in file order.
		for _, b := range order {
			if b.typ == "CARD" {
				s.Cards = append(s.Cards, parseCard(b.data, bkgds, shift))
			}
		}
	}
	return s, nil
}

// backgroundChain follows the backgrounds' next links from the first one.
func backgroundChain(bkgds map[int32]block, first int32, shift int) []block {
	var out []block
	seen := map[int32]bool{}
	for id := first; !seen[id]; {
		b, ok := bkgds[id]
		if !ok || len(b.data) < 32 {
			break
		}
		seen[id] = true
		out = append(out, b)
		id = int32(be.Uint32(b.data[28+shift:]))
	}
	return out
}

// cardOrder reads the card IDs in stack order from the LIST block and its
// pages.
func cardOrder(blocks map[string]map[int32]block, listID int32, shift int) []int32 {
	list, ok := blocks["LIST"][listID]
	if !ok || len(list.data) < 48 {
		return nil
	}
	l := list.data
	pages := int(be.Uint32(l[16+shift:]))
	entrySize := int(be.Uint16(l[28+shift:]))
	if entrySize < 4 {
		return nil
	}
	var ids []int32
	for i := range pages {
		e := 48 + i*6
		if e+6 > len(l) {
			break
		}
		page, ok := blocks["PAGE"][int32(be.Uint32(l[e:]))]
		if !ok {
			continue
		}
		count := int(be.Uint16(l[e+4:]))
		for j := range count {
			k := 24 + shift + j*entrySize
			if k+4 > len(page.data) {
				break
			}
			ids = append(ids, int32(be.Uint32(page.data[k:])))
		}
	}
	return ids
}

// layer is what backgrounds and cards share: parts, then field text, then
// the name and the script.
type layer struct {
	parts []*Part
	text  []rawText
	name  string
	scr   string
}

type rawText struct {
	partID int
	text   string
}

// parseLayer reads the parts and what follows them, starting at off with
// the part count's fields: count(2), next part ID(2), part list size(4),
// text count(2), text list size(4). A nonzero shift marks HyperCard 1.x.
func parseLayer(b []byte, off, shift int) layer {
	var l layer
	if off+14 > len(b) {
		return l
	}
	nParts := int(be.Uint16(b[off:]))
	listSize := int(be.Uint32(b[off+4:]))
	nText := int(be.Uint16(b[off+8:]))
	textSize := int(be.Uint32(b[off+10:]))
	p := off + 14
	partsEnd := p + listSize
	for range nParts {
		if p+30 > len(b) || p >= partsEnd {
			break
		}
		size := int(be.Uint16(b[p:]))
		if size < 30 || p+size > len(b) {
			break
		}
		part := b[p : p+size]
		name := cString(part[30:])
		script := ""
		if s := 30 + len(rawCString(part[30:])) + 2; s < len(part) {
			script = cString(part[s:]) // the name, its NUL, and a pad byte
		}
		l.parts = append(l.parts, &Part{ID: int(be.Uint16(part[2:])), Kind: PartKind(part[4]), Name: name, Script: script})
		p += size
	}
	p = min(partsEnd, len(b))
	textEnd := min(p+textSize, len(b))
	for range nText {
		if shift != 0 {
			// HyperCard 1.x: the part's ID, then the text up to a NUL.
			if p+2 > textEnd {
				break
			}
			raw := rawCString(b[p+2 : textEnd])
			l.text = append(l.text, rawText{int(int16(be.Uint16(b[p:]))), mactext.MacRoman(raw)})
			p += 2 + len(raw) + 1
			continue
		}
		if p+4 > textEnd {
			break
		}
		id := int(int16(be.Uint16(b[p:])))
		n := int(be.Uint16(b[p+2:]))
		if p+4+n > len(b) {
			break
		}
		l.text = append(l.text, rawText{id, fieldText(b[p+4 : p+4+n])})
		p += 4 + n + n&1 // padded to an even length
	}
	p = textEnd
	if p < len(b) {
		name := rawCString(b[p:])
		l.name = mactext.MacRoman(name)
		if q := p + len(name) + 1; q < len(b) {
			l.scr = cString(b[q:])
		}
	}
	return l
}

// fieldText decodes one field's stored text: a 0 byte for plain text, or
// a style table whose 16-bit length has the high bit set.
func fieldText(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if b[0]&0x80 != 0 && len(b) >= 2 {
		n := int(be.Uint16(b) & 0x7fff)
		if n > len(b) {
			n = len(b)
		}
		b = b[n:]
	} else {
		b = b[1:]
	}
	return mactext.MacRoman(bytes.TrimRight(b, "\x00"))
}

func parseBackground(b []byte, shift int) *Background {
	bg := &Background{ID: int32(be.Uint32(b[8:]))}
	l := parseLayer(b, 36+shift, shift)
	bg.Name, bg.Script, bg.Parts = l.name, l.scr, l.parts
	for _, t := range l.text {
		bg.Text = append(bg.Text, Text{Part: findPart(bg.Parts, t.partID), Text: t.text})
	}
	return bg
}

func parseCard(b []byte, bkgds map[int32]*Background, shift int) *Card {
	c := &Card{ID: int32(be.Uint32(b[8:]))}
	if len(b) >= 40 {
		c.Background = bkgds[int32(be.Uint32(b[36+shift:]))]
	}
	l := parseLayer(b, 40+shift, shift)
	c.Name, c.Script, c.Parts = l.name, l.scr, l.parts
	for _, t := range l.text {
		// A negative ID is one of the card's own fields, a positive one a
		// background field's text on this card.
		var part *Part
		if t.partID < 0 {
			part = findPart(c.Parts, -t.partID)
		} else if c.Background != nil {
			part = findPart(c.Background.Parts, t.partID)
			if part != nil && part.Kind == Button {
				// A background button's entry on a card is its
				// highlight there, "1" for on, not text.
				if t.text == "1" {
					c.Highlighted = append(c.Highlighted, part)
				}
				continue
			}
		}
		c.Text = append(c.Text, Text{Part: part, Text: t.text})
	}
	return c
}

func findPart(parts []*Part, id int) *Part {
	for _, p := range parts {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// rawCString returns the bytes of b up to its first NUL.
func rawCString(b []byte) []byte {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return b[:i]
	}
	return b
}

// cString decodes a NUL-terminated Mac Roman string, with HyperCard's
// carriage returns as line breaks.
func cString(b []byte) string {
	return string(bytes.ReplaceAll([]byte(mactext.MacRoman(rawCString(b))), []byte("\r"), []byte("\n")))
}
