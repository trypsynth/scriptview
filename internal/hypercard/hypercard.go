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
	"strings"

	"github.com/trypsynth/scriptview/internal/classic"
	"github.com/trypsynth/scriptview/internal/mactext"
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
	Version int
	Script  string
	// Language is the script's language when it isn't HyperTalk, such as
	// "AppleScript". Such a script is stored as text in Script, or as the
	// compiled data of its scripting component in Compiled.
	Language    string
	Compiled    []byte
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
	ID       int32
	Name     string
	Script   string
	Language string // see Stack.Language
	Compiled []byte
	Parts    []*Part
	// Text holds the text of background fields that share their text
	// across cards.
	Text []Text
}

// Card is a card.
type Card struct {
	ID         int32
	Name       string
	Script     string
	Language   string // see Stack.Language
	Compiled   []byte
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
	ID       int
	Kind     PartKind
	Name     string
	Script   string
	Language string // see Stack.Language
	Compiled []byte
	font     int // the ID of its text font, or -1 if unknown
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

// Parse reads a stack. Its text is Mac Roman, unless its fonts and text
// show another encoding, such as Mac Japanese: see [mactext.Detect].
func Parse(data []byte) (*Stack, error) {
	ps := &parser{enc: mactext.Roman}
	s, err := ps.parse(data)
	if err != nil {
		return s, err
	}
	// Only the fonts and kana count: guessing from text alone, a stack's
	// symbol fonts and decorations, such as "∞∞∞", read as anything.
	enc := mactext.Detect(ps.raw, append([]*mactext.Encoding{}, ps.fonts...))
	if enc == mactext.Roman {
		return s, nil
	}
	ps = &parser{enc: enc}
	return ps.parse(data)
}

// parser decodes a stack's text with enc, and keeps the raw bytes of every
// string it decodes in raw, and the encodings of the fonts in its font
// table in fonts.
type parser struct {
	enc   *mactext.Encoding
	raw   [][]byte
	fonts []*mactext.Encoding
	// fontEnc maps the IDs of the stack's fonts to their encodings, for
	// the fonts whose encoding is known, and styleFont maps the IDs of its
	// text styles to their fonts, -1 for a field's own font.
	fontEnc   map[int]*mactext.Encoding
	styleFont map[int]int
	layout
}

// layout is where a stack variant keeps its fields.
type layout struct {
	bo binary.ByteOrder
	// shift moves a background's or card's header fields after the bitmap
	// ID, and the LIST block's fields, which HyperCard 1.x and IIGS keep 4
	// bytes earlier than HyperCard 2.x.
	shift       int
	stackScript int  // offset of the stack script in the STAK block
	bkgdParts   int  // offset of a background's part count
	partName    int  // offset of a part's name in its record
	scriptAfter int  // bytes between a name's NUL and pad byte and its script
	pageTable   int  // offset of the page table in the LIST block
	pageEntries int  // offset of the first card entry in a PAGE block
	plainText   bool // field text is the part's ID and a NUL-terminated string
	// palettes marks HyperCard IIGS, where flag 0x0400 of a background or
	// card means it has a 32-byte color table, 16 colors as 0x0RGB, after
	// its field text.
	palettes bool
}

var (
	layout2x   = layout{bo: binary.BigEndian, stackScript: 0x600, bkgdParts: 36, partName: 30, pageTable: 48, pageEntries: 24}
	layout1x   = layout{bo: binary.BigEndian, shift: -4, stackScript: 0x600, bkgdParts: 32, partName: 30, pageTable: 48, pageEntries: 20, plainText: true}
	layoutIIGS = layout{bo: binary.LittleEndian, shift: -4, stackScript: 0x604, bkgdParts: 36, partName: 42, scriptAfter: 4, pageTable: 40, pageEntries: 24, plainText: true, palettes: true}
)

func (ps *parser) text(b []byte) string {
	ps.raw = append(ps.raw, b)
	return fontSymbols.Replace(ps.enc.DecodeOrRoman(b))
}

// scriptText decodes a script, with its carriage returns as line breaks.
// HyperTalk's continuation character is byte 0xC2 at the end of a line in
// every script system, which is ¬ only in Mac Roman.
func (ps *parser) scriptText(b []byte) string {
	if ps.enc == mactext.Roman {
		return strings.ReplaceAll(ps.text(b), "\r", "\n")
	}
	lines := bytes.Split(b, []byte("\r"))
	out := make([]string, len(lines))
	for i, l := range lines {
		if n := len(l); n > 0 && l[n-1] == 0xC2 {
			out[i] = ps.text(l[:n-1]) + "¬"
		} else {
			out[i] = ps.text(l)
		}
	}
	return strings.Join(out, "\n")
}

// fontTable reads an FTBL block: from 24, a 2-byte ID and a NUL-terminated
// name for each font, padded to an even length. It keeps the encodings of
// the non-Roman fonts in ps.fonts, and the encoding of each font whose
// encoding is known in ps.fontEnc. A font that isn't non-Roman is Roman,
// apart from IDs 0 and 1, the system and application fonts, which follow
// the system's script, and IDs from 16384 up, which a utility may have
// given a font of any script.
func (ps *parser) fontTable(b []byte) {
	if len(b) < 24 {
		return
	}
	n := int(ps.bo.Uint32(b[16:]))
	for p := 24; n > 0 && p+3 <= len(b); n-- {
		id := int(int16(ps.bo.Uint16(b[p:])))
		name := rawCString(b[p+2:])
		p += 2 + len(name) + 1
		p += p & 1
		switch e := mactext.ForFont(id, name); {
		case e != nil:
			ps.fonts = append(ps.fonts, e)
			ps.fontEnc[id] = e
		case id >= 2 && id < 16384:
			ps.fontEnc[id] = mactext.Roman
		}
	}
}

// styleTable reads an STBL block: from 24, 24 bytes for each style, with
// its ID, 4 bytes, at 0, and its font's ID, 2 bytes, at 12.
func (ps *parser) styleTable(b []byte) {
	if len(b) < 24 {
		return
	}
	n := int(ps.bo.Uint32(b[16:]))
	for p := 24; n > 0 && p+24 <= len(b); n, p = n-1, p+24 {
		ps.styleFont[int(int32(ps.bo.Uint32(b[p:])))] = int(int16(ps.bo.Uint16(b[p+12:])))
	}
}

// fontSymbols maps control characters that the system font draws as
// symbols, as in "press ⌘-H" in a field.
var fontSymbols = strings.NewReplacer("\x11", "⌘", "\x12", "✓", "\x13", "◆", "\x14", "\uf8ff")

func (ps *parser) parse(data []byte) (*Stack, error) {
	if !IsStack(data) {
		return nil, ErrNotStack
	}
	ps.layout = layout2x
	if len(data) >= 20 && binary.LittleEndian.Uint32(data[16:]) < 0x100 && be.Uint32(data[16:]) >= 0x100 {
		ps.layout = layoutIIGS // a format version that is small only read little-endian
	}
	bo := ps.bo
	blocks := map[string]map[int32]block{}
	var order []block
	for p := 0; p+12 <= len(data); {
		size := int(bo.Uint32(data[p:]))
		if size < 12 || p+size > len(data) {
			break
		}
		b := block{typ: string(data[p+4 : p+8]), id: int32(bo.Uint32(data[p+8:])), data: data[p : p+size]}
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
	ps.fontEnc, ps.styleFont = map[int]*mactext.Encoding{}, map[int]int{}
	for _, b := range order {
		switch b.typ {
		case "FTBL":
			ps.fontTable(b.data)
		case "STBL":
			ps.styleTable(b.data)
		}
	}
	stak := order[0].data
	if len(stak) < 56 {
		return nil, fmt.Errorf("hypercard: STAK block too short")
	}
	s := &Stack{Version: int(bo.Uint32(stak[16:]))}
	if ps.bo == binary.BigEndian && s.Version < 10 {
		// HyperCard 1.x blocks lack a field before the bitmap ID, so the
		// rest of a background's or card's header comes 4 bytes earlier.
		ps.layout = layout1x
	}
	if len(stak) > ps.stackScript {
		s.Script, s.Language, s.Compiled = ps.script(stak[ps.stackScript:])
	}
	chain := ps.backgroundChain(blocks["BKGD"], int32(bo.Uint32(stak[40:])))
	if len(chain) < len(blocks["BKGD"]) {
		// A broken chain, as in a private-access stack, whose header is
		// scrambled: take the backgrounds in file order.
		chain = chain[:0]
		for _, b := range order {
			if b.typ == "BKGD" {
				chain = append(chain, b)
			}
		}
	}
	bkgds := map[int32]*Background{}
	for _, b := range chain {
		bg := ps.parseBackground(b.data)
		bkgds[bg.ID] = bg
		s.Backgrounds = append(s.Backgrounds, bg)
	}
	for _, id := range ps.cardOrder(blocks, int32(bo.Uint32(stak[52:]))) {
		b, ok := blocks["CARD"][id]
		if !ok {
			continue
		}
		s.Cards = append(s.Cards, ps.parseCard(b.data, bkgds))
	}
	if len(s.Cards) == 0 {
		// Without a usable card list, take the cards in file order.
		for _, b := range order {
			if b.typ == "CARD" {
				s.Cards = append(s.Cards, ps.parseCard(b.data, bkgds))
			}
		}
	}
	return s, nil
}

// backgroundChain follows the backgrounds' next links from the first one.
func (ps *parser) backgroundChain(bkgds map[int32]block, first int32) []block {
	var out []block
	seen := map[int32]bool{}
	for id := first; !seen[id]; {
		b, ok := bkgds[id]
		if !ok || len(b.data) < 32 {
			break
		}
		seen[id] = true
		out = append(out, b)
		id = int32(ps.bo.Uint32(b.data[28+ps.shift:]))
	}
	return out
}

// cardOrder reads the card IDs in stack order from the LIST block and its
// pages.
func (ps *parser) cardOrder(blocks map[string]map[int32]block, listID int32) []int32 {
	list, ok := blocks["LIST"][listID]
	if !ok || len(list.data) < 48 {
		return nil
	}
	l := list.data
	bo := ps.bo
	pages := int(bo.Uint32(l[16+ps.shift:]))
	entrySize := int(bo.Uint16(l[28+ps.shift:]))
	if entrySize < 4 {
		return nil
	}
	var ids []int32
	for i := range pages {
		e := ps.pageTable + i*6
		if e+6 > len(l) {
			break
		}
		page, ok := blocks["PAGE"][int32(bo.Uint32(l[e:]))]
		if !ok {
			continue
		}
		count := int(bo.Uint16(l[e+4:]))
		for j := range count {
			k := ps.pageEntries + j*entrySize
			if k+4 > len(page.data) {
				break
			}
			ids = append(ids, int32(bo.Uint32(page.data[k:])))
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
	lang  string
	osa   []byte
}

type rawText struct {
	partID int
	text   string
}

// parseLayer reads the parts and what follows them, starting at off with
// the part count's fields: count(2), next part ID(2), part list size(4),
// text count(2), text list size(4).
// parseLayer reads a background's or card's parts and field text from off.
// bg is a card's background's parts, whose fields' text the card holds.
func (ps *parser) parseLayer(b []byte, off int, bg []*Part) layer {
	palette := ps.palettes && len(b) >= 18 && ps.bo.Uint16(b[16:])&0x0400 != 0
	var l layer
	if off+14 > len(b) {
		return l
	}
	bo := ps.bo
	nParts := int(bo.Uint16(b[off:]))
	listSize := int(bo.Uint32(b[off+4:]))
	nText := int(bo.Uint16(b[off+8:]))
	textSize := int(bo.Uint32(b[off+10:]))
	p := off + 14
	partsEnd := p + listSize
	for range nParts {
		n := ps.partName
		if p+n > len(b) || p >= partsEnd {
			break
		}
		size := int(bo.Uint16(b[p:]))
		if size < n || p+size > len(b) {
			break
		}
		part := b[p : p+size]
		name := ps.cString(part[n:])
		pt := &Part{ID: int(bo.Uint16(part[2:])), Kind: PartKind(part[4]), Name: name, font: -1}
		if !ps.plainText && len(part) >= 26 {
			pt.font = int(int16(bo.Uint16(part[24:])))
		}
		if ps.palettes {
			pt.Kind &= 0x0f // IIGS keeps more in the kind byte's high bits
		}
		// The name, its NUL, and a pad byte.
		if s := n + len(rawCString(part[n:])) + 2 + ps.scriptAfter; s < len(part) {
			pt.Script, pt.Language, pt.Compiled = ps.script(part[s:])
		}
		l.parts = append(l.parts, pt)
		p += size
	}
	p = min(partsEnd, len(b))
	textEnd := min(p+textSize, len(b))
	for range nText {
		if ps.plainText {
			// HyperCard 1.x and IIGS: the part's ID, then the text up to a
			// NUL.
			if p+2 > textEnd {
				break
			}
			raw := rawCString(b[p+2 : textEnd])
			l.text = append(l.text, rawText{int(int16(bo.Uint16(b[p:]))), ps.text(raw)})
			p += 2 + len(raw) + 1
			continue
		}
		if p+4 > textEnd {
			break
		}
		id := int(int16(bo.Uint16(b[p:])))
		n := int(bo.Uint16(b[p+2:]))
		if p+4+n > len(b) {
			break
		}
		font := -1
		switch {
		case id < 0:
			font = partFont(l.parts, -id)
		case bg != nil:
			font = partFont(bg, id)
		default:
			font = partFont(l.parts, id)
		}
		l.text = append(l.text, rawText{id, ps.fieldText(b[p+4:p+4+n], font)})
		p += 4 + n + n&1 // padded to an even length
	}
	p = textEnd
	if palette {
		p += 32
	}
	if p < len(b) {
		name := rawCString(b[p:])
		l.name = ps.text(name)
		if q := p + len(name) + 1 + ps.scriptAfter; q < len(b) {
			l.scr, l.lang, l.osa = ps.script(b[q:])
		}
	}
	return l
}

// fieldText decodes one field's stored text: a 0 byte for plain text, or
// a style table whose 16-bit length has the high bit set, with a 2-byte
// offset in the text and a 2-byte style ID for each run. Each run decodes
// in the encoding of its font, or of the field's font, font.
func (ps *parser) fieldText(b []byte, font int) string {
	if len(b) == 0 {
		return ""
	}
	type run struct{ start, font int }
	runs := []run{{0, font}}
	if b[0]&0x80 != 0 && len(b) >= 2 {
		n := min(int(be.Uint16(b)&0x7fff), len(b))
		runs = runs[:0]
		for p := 2; p+4 <= n; p += 4 {
			f, ok := ps.styleFont[int(be.Uint16(b[p+2:]))]
			if !ok || f == -1 {
				f = font
			}
			start := int(be.Uint16(b[p:]))
			if len(runs) > 0 && start <= runs[len(runs)-1].start {
				continue // a stale run, after an edit
			}
			runs = append(runs, run{start, f})
		}
		if len(runs) == 0 || runs[0].start > 0 {
			runs = append([]run{{0, font}}, runs...)
		}
		b = b[n:]
	} else {
		b = b[1:]
	}
	b = bytes.TrimRight(b, "\x00")
	ps.raw = append(ps.raw, b)
	var sb strings.Builder
	for i, r := range runs {
		end := len(b)
		if i+1 < len(runs) {
			end = min(runs[i+1].start, len(b)) // runs can outlast the text
		}
		if r.start >= end {
			continue
		}
		sb.WriteString(fontSymbols.Replace(ps.decodeRun(b[r.start:end], r.font)))
	}
	return sb.String()
}

// decodeRun decodes a run of field text in a font. A run in a non-Roman
// font decodes in its font's encoding. A run in a Roman font, in a stack
// that isn't Roman, decodes as Mac Roman only when it shows it: it doesn't
// decode in the stack's encoding, or that reading puts wide characters
// inside an ASCII word, as in "Fr仕屍ic" for "Frédéric". Fonts with low IDs
// can hold any script, renumbered by a utility.
func (ps *parser) decodeRun(b []byte, font int) string {
	e, ok := ps.fontEnc[font]
	switch {
	case !ok:
		return ps.enc.DecodeOrRoman(b)
	case e != mactext.Roman || ps.enc == mactext.Roman:
		return e.DecodeOrRoman(b)
	}
	if s, ok := ps.enc.Decode(b); ok && !mactext.WideInWord(s) {
		return s
	}
	return mactext.MacRoman(b)
}

// partFont returns the text font of the part with the ID, or -1.
func partFont(parts []*Part, id int) int {
	if p := findPart(parts, id); p != nil {
		return p.font
	}
	return -1
}

func (ps *parser) parseBackground(b []byte) *Background {
	bg := &Background{ID: int32(ps.bo.Uint32(b[8:]))}
	l := ps.parseLayer(b, ps.bkgdParts, nil)
	bg.Name, bg.Script, bg.Language, bg.Compiled, bg.Parts = l.name, l.scr, l.lang, l.osa, l.parts
	for _, t := range l.text {
		bg.Text = append(bg.Text, Text{Part: findPart(bg.Parts, t.partID), Text: t.text})
	}
	return bg
}

func (ps *parser) parseCard(b []byte, bkgds map[int32]*Background) *Card {
	c := &Card{ID: int32(ps.bo.Uint32(b[8:]))}
	if len(b) >= 40 {
		c.Background = bkgds[int32(ps.bo.Uint32(b[36+ps.shift:]))]
	}
	var bgParts []*Part
	if c.Background != nil {
		bgParts = c.Background.Parts
	}
	l := ps.parseLayer(b, 40+ps.shift, bgParts)
	c.Name, c.Script, c.Language, c.Compiled, c.Parts = l.name, l.scr, l.lang, l.osa, l.parts
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
				// highlight there, not text: "1" for on, or 0x80 in
				// HyperCard IIGS.
				if t.text == "1" || t.text == "\u00c4" {
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

// script reads a script: HyperTalk text, or, after a NUL byte, a script
// in another language. That one has a 32-byte header, which starts with
// WOSA and holds the header's length, the data's length, the scripting
// component (ascr for AppleScript), and the storage type: scpt for compiled
// data, or a text type for source that wasn't compiled. The data follows
// the header.
func (ps *parser) script(b []byte) (text, language string, compiled []byte) {
	if len(b) < 1+20 || b[0] != 0 || string(b[1:5]) != "WOSA" {
		return ps.scriptText(rawCString(b)), "", nil
	}
	start := 1 + int(be.Uint32(b[5:]))
	n := int(be.Uint32(b[9:]))
	if start > len(b) || n < 0 {
		return "", "", nil
	}
	data := b[start:min(start+n, len(b))]
	language = string(b[13:17])
	if language == "ascr" {
		language = "AppleScript"
	}
	if string(b[17:21]) == "scpt" {
		return "", language, data
	}
	return ps.scriptText(data), language, nil
}

// cString decodes a NUL-terminated string, with HyperCard's carriage
// returns as line breaks.
func (ps *parser) cString(b []byte) string {
	return strings.ReplaceAll(ps.text(rawCString(b)), "\r", "\n")
}
