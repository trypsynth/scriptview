// Package supercard reads SuperCard projects: the scripts of their windows,
// cards, backgrounds, objects, and menus.
//
// A project keeps everything in its data fork, as a table of blocks: a
// 1024-byte header, a table of block offsets indexed by block ID, and the
// blocks, each with its ID, its length, and a kind. Windows, menus, cards,
// and backgrounds each have a block, and the objects on a card or
// background live in its block. A standalone application built with
// SuperCard has its project in the data fork as well.
package supercard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/trypsynth/scriptview/internal/mactext"
)

// ErrNotProject is returned for data that isn't a SuperCard project.
var ErrNotProject = errors.New("not a SuperCard project")

var be = binary.BigEndian

// Project is a SuperCard project.
type Project struct {
	// Major and Minor give the file format: 1.0 for SuperCard 1 and 2, 1.1
	// for the 2.x trial, 2.1 to 2.7 for SuperCard 3 to 4.5.
	Major, Minor int
	Script       string
	Windows      []*Window // in the order of their blocks
	Menus        []*Menu
	// Unlisted holds the cards and backgrounds that no window lists.
	Unlisted []*Unlisted
}

// Unlisted is a card or a background that no window lists.
type Unlisted struct {
	Kind string // "card" or "background"
	*Layer
}

// Window is a window, with its own cards and backgrounds.
type Window struct {
	Name        string
	Script      string
	Backgrounds []*Layer // in the order the cards first use them
	Cards       []*Layer // in card order
}

// Layer is a card or a background.
type Layer struct {
	ID         int
	Name       string
	Script     string
	Background *Layer // a card's background
	Objects    []*Object
	// Unmatched holds scripts of objects on the layer that scriptview
	// couldn't match to their objects.
	Unmatched []string
}

// Kind says what an object is.
type Kind int

// The object kinds.
const (
	Graphic Kind = 1
	Button  Kind = 2
	Field   Kind = 3
)

func (k Kind) String() string {
	switch k {
	case Graphic:
		return "graphic"
	case Button:
		return "button"
	}
	return "field"
}

// Object is a graphic, a button, or a field.
type Object struct {
	ID     int
	Kind   Kind
	Name   string
	Script string
}

// Menu is a menu of the project's menu bar.
type Menu struct {
	Name   string
	Script string
	Items  []*MenuItem
}

// MenuItem is an item of a menu.
type MenuItem struct {
	Name   string
	Script string
}

// The kinds of block.
const (
	kindProject    = 0
	kindCardList   = 2
	kindWindow     = 4
	kindMenu       = 5
	kindCard       = 7
	kindBackground = 8
)

// layout gives the offsets that differ between file formats: where each
// kind of block keeps its script, and the size of an object's record and
// of a menu item's.
type layout struct {
	project, menu, background, card, window int
	object, item                            int
}

var (
	layout10 = layout{project: 38, menu: 42, background: 48, card: 52, window: 68, object: 82, item: 26}
	layout11 = layout{project: 40, menu: 44, background: 50, card: 54, window: 70, object: 84, item: 28}
	layout2  = layout{project: 54, menu: 58, background: 64, card: 68, window: 84, object: 98, item: 42}
)

// IsProject reports whether data is a SuperCard project, or a standalone
// application's data fork with a project in it.
func IsProject(data []byte) bool {
	_, ok := project(data)
	return ok
}

// project returns a project's bytes. The header gives the format, the
// project's length at 4, and the size of the block table at 16, a
// multiple of 128. A PowerPC standalone's data fork has the application's
// code at 0x400, so its project is the first 0x400 bytes and the last
// length - 0x400.
func project(data []byte) ([]byte, bool) {
	if len(data) < 0x600 {
		return nil, false
	}
	major, minor := be.Uint16(data), be.Uint16(data[2:])
	size, n := int64(be.Uint32(data[4:])), int64(be.Uint32(data[16:]))
	if major < 1 || major > 2 || minor > 15 || n == 0 || n%128 != 0 || 0x400+4*n > size {
		return nil, false
	}
	switch {
	case size == int64(len(data)):
	case size < int64(len(data)) && string(data[0x400:0x408]) == "Joy!peff":
		data = append(data[:0x400:0x400], data[int64(len(data))-(size-0x400):]...)
	default:
		return nil, false
	}
	// Block 0 is the project's own block.
	off := int64(be.Uint32(data[0x400:]))
	if off < 0x400+4*n || off+16 > size || be.Uint32(data[off:]) != 0 || be.Uint16(data[off+12:]) != kindProject {
		return nil, false
	}
	return data, true
}

// Parse reads a project. Its text is Mac Roman, unless it reads better in
// another encoding, such as Mac Japanese: see [mactext.Detect].
func Parse(data []byte) (*Project, error) {
	return ParseWith(data, nil)
}

// ParseWith reads a project, decoding its text in enc, or in the encoding
// it detects if enc is nil.
func ParseWith(data []byte, enc *mactext.Encoding) (*Project, error) {
	data, ok := project(data)
	if !ok {
		return nil, ErrNotProject
	}
	ps := &parser{blocks: blocks(data)}
	ps.major, ps.minor = int(be.Uint16(data)), int(be.Uint16(data[2:]))
	switch {
	case ps.major == 2:
		ps.layout = layout2
	case ps.minor == 0:
		ps.layout = layout10
	default:
		ps.layout = layout11
	}
	// The first pass gathers the text, to pick its encoding; the second
	// decodes it.
	ps.parse()
	ps.enc = enc
	if enc == nil {
		ps.enc = mactext.Detect(ps.raw, nil)
	}
	ps.raw = nil
	return ps.parse(), nil
}

type block struct {
	kind int
	data []byte // the whole block, header included
}

// blocks reads the block table: from 0x400, the offset of each block by
// ID. A block starts with its ID, the length of the rest after the
// 12-byte header, and its allocated size, and has its kind at 12. Slots of
// freed IDs hold other numbers. Past each block's length is old memory,
// which is never read.
func blocks(data []byte) map[int]block {
	n := int(be.Uint32(data[16:]))
	out := map[int]block{}
	for id := range n {
		off := int(be.Uint32(data[0x400+4*id:]))
		if off%0x40 != 0 || off < 0x400+4*n || off+16 > len(data) || int(be.Uint32(data[off:])) != id {
			continue
		}
		end := off + 12 + int(be.Uint32(data[off+4:]))
		if end > len(data) || end < off+16 {
			continue
		}
		out[id] = block{int(be.Uint16(data[off+12:])), data[off:end]}
	}
	return out
}

type parser struct {
	layout
	major, minor int
	blocks       map[int]block
	enc          *mactext.Encoding
	raw          [][]byte     // the text read, when enc is nil
	seen         map[int]bool // the cards and backgrounds read
}

// parse reads the project's blocks.
func (ps *parser) parse() *Project {
	p := &Project{Major: ps.major, Minor: ps.minor}
	ps.seen = map[int]bool{}
	ids := make([]int, 0, len(ps.blocks))
	for id := range ps.blocks {
		ids = append(ids, id)
	}
	sortInts(ids)
	for _, id := range ids {
		b := ps.blocks[id]
		switch b.kind {
		case kindProject:
			if id == 0 {
				p.Script, _ = ps.script(b.data, ps.layout.project)
			}
		case kindWindow:
			p.Windows = append(p.Windows, ps.window(b.data))
		case kindMenu:
			p.Menus = append(p.Menus, ps.menu(b.data))
		}
	}
	for _, id := range ids {
		b := ps.blocks[id]
		switch {
		case ps.seen[id]:
		case b.kind == kindCard:
			p.Unlisted = append(p.Unlisted, &Unlisted{"card", ps.layer(b.data, ps.layout.card)})
		case b.kind == kindBackground:
			p.Unlisted = append(p.Unlisted, &Unlisted{"background", ps.layer(b.data, ps.layout.background)})
		}
	}
	return p
}

// window reads a window's block. Its cards are listed in card list
// blocks, of up to 100 cards each: 8 bytes before where its script would
// be with one list, the number of lists, then for each list, the number of
// cards in it and the ID of its block. The script comes after the lists,
// and the name last.
func (ps *parser) window(b []byte) *Window {
	w := &Window{}
	at := ps.layout.window - 8
	if at+2 > len(b) {
		return w
	}
	n := int(be.Uint16(b[at:]))
	if at+2+6*n+4 > len(b) {
		return w
	}
	var lists []int
	for i := range n {
		lists = append(lists, int(be.Uint32(b[at+4+6*i:])))
	}
	w.Script, at = ps.script(b, at+2+6*n)
	w.Name = ps.text(lastRecord(b, at))
	// A card list has a 64-byte entry for each card, from 14: the IDs of
	// the card's block and its background's block, then the card's and
	// the background's ID numbers.
	bgs := map[int]*Layer{}
	for _, list := range lists {
		lb, ok := ps.blocks[list]
		if !ok || lb.kind != kindCardList {
			continue
		}
		for e := 14; e+64 <= len(lb.data); e += 64 {
			cb, ok := ps.blocks[int(be.Uint32(lb.data[e:]))]
			if !ok || cb.kind != kindCard {
				continue
			}
			ps.seen[int(be.Uint32(lb.data[e:]))] = true
			card := ps.layer(cb.data, ps.layout.card)
			card.ID = int(be.Uint32(lb.data[e+8:]))
			bgID := int(be.Uint32(lb.data[e+4:]))
			bg := bgs[bgID]
			if bb, ok := ps.blocks[bgID]; bg == nil && ok && bb.kind == kindBackground {
				ps.seen[bgID] = true
				bg = ps.layer(bb.data, ps.layout.background)
				bg.ID = int(be.Uint32(lb.data[e+12:]))
				bgs[bgID] = bg
				w.Backgrounds = append(w.Backgrounds, bg)
			}
			card.Background = bg
			w.Cards = append(w.Cards, card)
		}
	}
	return w
}

// layer reads a card or background block: its script, then the length of
// its object list and the list, then records that end with its name.
func (ps *parser) layer(b []byte, at int) *Layer {
	l := &Layer{}
	l.Script, at = ps.script(b, at)
	if at+4 > len(b) {
		return l
	}
	end := at + 4 + int(be.Uint32(b[at:]))
	if end > len(b) || end < at+6 {
		return l
	}
	ps.objects(l, b[at+4:end], 0)
	l.Name = ps.text(lastRecord(b, end))
	return l
}

// lastRecord returns the last of the records from b[at:], which is a
// name: after a window's script, or a card's or background's objects,
// can come other records, such as user properties, before the name.
func lastRecord(b []byte, at int) []byte {
	var last []byte
	for at+4 <= len(b) {
		r, ok := record(b, at)
		if !ok {
			break
		}
		last = r
		at = next(at, len(r))
	}
	return last
}

type obj struct {
	*Object
	slots
	group bool
}

// objects reads an object list: a fixed record for each object, then the
// records each object's handles point to, then the number of objects, in
// 2 bytes.
//
// An object's record has its type at 0 (graphics from 100, buttons from
// 200, and fields from 300), its ID at 8, and from 58, its name, data,
// and script handles. The data is a field's text, a graphic's drawing, or
// the like. Each handle that isn't 0 has a record after the fixed ones, in
// the order data, name, script. From SuperCard 3 on, an object can have
// more records, such as its user properties, whose handles aren't always
// at the same offsets. A group, type 108, has the list of the objects in
// it as its data.
func (ps *parser) objects(l *Layer, b []byte, depth int) {
	if len(b) < 2 {
		return
	}
	n := int(be.Uint16(b[len(b)-2:]))
	size := ps.layout.object
	if n*size > len(b)-2 {
		return
	}
	var objs []obj
	for i := range n {
		r := b[i*size : (i+1)*size]
		t := int(be.Uint16(r))
		if t < 100 || t >= 400 {
			return
		}
		o := obj{Object: &Object{ID: int(be.Uint16(r[8:])), Kind: Kind(t / 100)}, group: t == 108}
		o.name, o.data, o.script = be.Uint32(r[58:]) != 0, be.Uint32(r[62:]) != 0, be.Uint32(r[66:]) != 0
		if size == 98 {
			o.extra = extraHandles(r)
		}
		objs = append(objs, o)
	}
	var recs [][]byte
	at := n * size
	for at < len(b)-2 {
		r, ok := record(b, at)
		if !ok {
			return
		}
		recs = append(recs, r)
		at = next(at, len(r))
	}
	if at != len(b)-2 {
		return
	}
	// Match the records to the objects. Names and scripts are easy to tell
	// apart from other records, so for each object, try the most extra
	// records first, and fewer until the rest of the records fit.
	extras := matchRecords(objs, recs)
	if extras == nil {
		for _, r := range recs {
			if isScript(r) {
				if s := ps.scriptRecord(r); strings.TrimSpace(s) != "" {
					l.Unmatched = append(l.Unmatched, s)
				}
			}
		}
		return
	}
	j := 0
	for i, o := range objs {
		var data []byte
		if o.data {
			data = recs[j]
			j++
		}
		if o.name {
			o.Name = ps.text(recs[j])
			j++
		}
		if o.script {
			o.Script = ps.scriptRecord(recs[j])
			j++
		}
		j += extras[i]
		l.Objects = append(l.Objects, o.Object)
		if o.group && data != nil && depth < 8 {
			ps.objects(l, data, depth+1)
		}
	}
}

// slots says which records an object has: its data, name, and script, and
// up to extra more.
type slots struct {
	data, name, script bool
	extra              int
}

// matchRecords matches records to the objects' slots, and returns how many
// extra records each object has, or nil if the records don't fit.
func matchRecords(objs []obj, recs [][]byte) []int {
	failed := map[[2]int]bool{}
	var match func(i, j int) []int
	match = func(i, j int) []int {
		if i == len(objs) {
			if j == len(recs) {
				return []int{}
			}
			return nil
		}
		if failed[[2]int{i, j}] {
			return nil
		}
		failed[[2]int{i, j}] = true
		s, k := objs[i].slots, j
		if s.data {
			k++
		}
		if s.name {
			if k >= len(recs) || !isName(recs[k]) {
				return nil
			}
			k++
		}
		if s.script {
			if k >= len(recs) || !isScript(recs[k]) {
				return nil
			}
			k++
		}
		for m := s.extra; m >= 0; m-- {
			if k+m > len(recs) || anyScript(recs[k:k+m]) {
				continue
			}
			if rest := match(i+1, k+m); rest != nil {
				return append([]int{m}, rest...)
			}
		}
		return nil
	}
	return match(0, 0)
}

// extraHandles counts the handles a SuperCard 3 or later object record
// may have past its first three: at 86, and at 72, 78, and 94 when the
// value there looks like a handle.
func extraHandles(r []byte) int {
	n := 0
	if be.Uint32(r[86:]) != 0 {
		n++
	}
	for _, at := range []int{72, 78, 94} {
		if h := be.Uint32(r[at:]); h != 0 && h&1 == 0 && sameHeap(h, r, 58, 62, 66, 86) {
			n++
		}
	}
	return n
}

// menu reads a menu's block: its script, its items, and its name.
func (ps *parser) menu(b []byte) *Menu {
	m := &Menu{}
	at := 0
	m.Script, at = ps.script(b, ps.layout.menu)
	if at+4 > len(b) {
		return m
	}
	end := at + 4 + int(be.Uint32(b[at:]))
	if end > len(b) || end < at+6 {
		return m
	}
	m.Items = ps.items(b[at+4 : end])
	m.Name = ps.text(lastRecord(b, end))
	return m
}

// items reads a menu's items: a record for each item, with its name and
// script handles at 18 and 22, then the records they point to, then the
// number of items, in 2 bytes. From SuperCard 3 on, an item can have
// another record, such as its user properties.
func (ps *parser) items(b []byte) []*MenuItem {
	if len(b) < 2 {
		return nil
	}
	n, size := int(be.Uint16(b[len(b)-2:])), ps.layout.item
	if n*size > len(b)-2 {
		return nil
	}
	objs := make([]obj, n)
	for i := range objs {
		r := b[i*size : (i+1)*size]
		objs[i] = obj{Object: &Object{}}
		objs[i].name, objs[i].script = be.Uint32(r[18:]) != 0, be.Uint32(r[22:]) != 0
		for at := 26; at+4 <= size; at += 4 {
			if h := be.Uint32(r[at:]); h != 0 && h&1 == 0 && sameHeap(h, r, 18, 22) {
				objs[i].extra++
			}
		}
	}
	var recs [][]byte
	at := n * size
	for at < len(b)-2 {
		r, ok := record(b, at)
		if !ok {
			return nil
		}
		recs = append(recs, r)
		at = next(at, len(r))
	}
	extras := matchRecords(objs, recs)
	if at != len(b)-2 || extras == nil {
		return nil
	}
	var items []*MenuItem
	j := 0
	for i, o := range objs {
		it := &MenuItem{}
		if o.name {
			it.Name = ps.text(recs[j])
			j++
		}
		if o.script {
			it.Script = ps.scriptRecord(recs[j])
			j++
		}
		j += extras[i]
		items = append(items, it)
	}
	return items
}

// sameHeap reports whether h looks like a handle from the same heap as the
// handles at the given offsets of r: whether their high 16 bits match.
func sameHeap(h uint32, r []byte, offsets ...int) bool {
	for _, at := range offsets {
		if o := be.Uint32(r[at:]); o != 0 && o>>16 == h>>16 {
			return true
		}
	}
	return false
}

// record returns the record at b[at:]: a 4-byte length and that many
// bytes.
func record(b []byte, at int) ([]byte, bool) {
	if at < 0 || at+4 > len(b) {
		return nil, false
	}
	n := int(be.Uint32(b[at:]))
	if n > len(b)-at-4 {
		return nil, false
	}
	return b[at+4 : at+4+n], true
}

// next returns the offset after a record of n bytes at at, which is padded
// to an even length.
func next(at, n int) int {
	return at + 4 + n + n&1
}

// script reads the script record at b[at:], and returns the script and the
// offset after it.
func (ps *parser) script(b []byte, at int) (string, int) {
	r, ok := record(b, at)
	if !ok {
		return "", len(b)
	}
	return ps.scriptRecord(r), next(at, len(r))
}

// Markers of protected and tokenized scripts.
var (
	protected = []byte("CE%\x00\x00\x00\x00\x00")
	tokenized = []byte{0x13, 0x45, 0x77, 0xFF}
)

// Tokenized is what scriptview prints for a tokenized script.
const Tokenized = "-- (a tokenized script, which has no source text for scriptview to read)"

// scriptRecord decodes a script record: the text, a NUL, and the script
// editor's state, ending in its window's rectangle. A protected script is
// XORed with 0xFD after an 8-byte marker, either all of it or the text
// before its last carriage return. Standalones from SuperCard 1 can keep
// their scripts tokenized, with no source text.
func (ps *parser) scriptRecord(r []byte) string {
	if bytes.HasPrefix(r, tokenized) {
		return Tokenized
	}
	if bytes.HasPrefix(r, protected) {
		r = unprotect(r[len(protected):])
	}
	if k := bytes.IndexByte(r, 0); k >= 0 {
		r = r[:k]
	}
	return ps.scriptText(r)
}

// unprotect decodes a protected script's record.
func unprotect(r []byte) []byte {
	d := make([]byte, len(r))
	for i, c := range r {
		d[i] = c ^ 0xFD
	}
	if k := bytes.IndexByte(d, 0); k >= 0 && editorState(len(r)-k) {
		return d
	}
	if k := bytes.IndexByte(r, 0); k > 0 {
		return append(d[:k-1], r[k-1:]...)
	}
	return d
}

// editorState reports whether n bytes after a script's text, counting its
// NUL, is the size of the script editor's state.
func editorState(n int) bool {
	return n == 23 || n == 24 || n == 27
}

// isScript reports whether a record is a script.
func isScript(r []byte) bool {
	if bytes.HasPrefix(r, tokenized) || bytes.HasPrefix(r, protected) {
		return true
	}
	k := bytes.IndexByte(r, 0)
	if k < 0 || !editorState(len(r)-k) {
		return false
	}
	rect := r[len(r)-8:]
	return int16(be.Uint16(rect)) < int16(be.Uint16(rect[4:])) && int16(be.Uint16(rect[2:])) < int16(be.Uint16(rect[6:]))
}

func anyScript(rs [][]byte) bool {
	for _, r := range rs {
		if isScript(r) {
			return true
		}
	}
	return false
}

// isName reports whether a record can be a name: one with no NUL, which
// every script record has.
func isName(r []byte) bool {
	return len(r) < 256 && bytes.IndexByte(r, 0) < 0
}

func (ps *parser) text(b []byte) string {
	if ps.enc == nil {
		ps.raw = append(ps.raw, b)
		return mactext.MacRoman(b)
	}
	return ps.enc.DecodeOrRoman(b)
}

// scriptText decodes a script, with its carriage returns as line breaks.
// SuperTalk's continuation character is byte 0xC2 at the end of a line in
// every script system, which is ¬ only in Mac Roman.
func (ps *parser) scriptText(b []byte) string {
	if ps.enc == nil || ps.enc == mactext.Roman {
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

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
