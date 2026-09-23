package scpt

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// HandlerSig is a detected handler definition with its resolved parameter list.
type HandlerSig struct {
	Name   string
	Params []string
}

// File holds the parsed contents of a .scpt file.
type File struct {
	// Strings contains all text literals found in the bytecode, in order.
	Strings []string
	// Idents contains all user-defined identifier names, preserving original case.
	// Each entry is deduplicated; the first occurrence wins.
	Idents []string
	// Ints contains all non-negative integer literals found in the bytecode.
	Ints []int
	// Handlers contains handler definitions in source order, with parameter names where resolvable.
	Handlers []HandlerSig
	// dc carries the full parse tree for body reconstruction.
	dc *decompState
	// source is set for files that carry their source directly: compiled
	// JavaScript for Automation and plain-text files named .scpt.
	source string
}

// nodeRec is an internal representation of a 0x0d node record.
type nodeRec struct {
	typ byte
	// flags is the first word of the fixed header. Bit 0 marks an alternate
	// surface syntax: one-line `tell … to`, or `is in` for a contains node.
	flags uint16
	// start and end are the node's source text range (character offsets).
	start, end uint16
	children   []int16
}

// Parse reads and validates a .scpt file, extracting literals, identifiers,
// and handler definitions.
func Parse(data []byte) (*File, error) {
	if bytes.HasPrefix(data, JXAMagic) {
		src, err := parseJXA(data)
		if err != nil {
			return nil, err
		}
		return &File{source: src}, nil
	}
	if len(data) < len(Magic) || !bytes.HasPrefix(data, Magic) {
		if isPlainText(data) {
			return &File{source: string(data)}, nil // uncompiled source saved as .scpt
		}
		return nil, ErrNotSCPT
	}
	if len(data) < len(Magic)+len(Trailer) {
		return nil, ErrTruncated
	}
	body := data[len(Magic):]
	if n := trailerLen(data); n > 0 && n <= len(body) {
		body = body[:len(body)-n]
	}

	b := newBuilder()
	if err := parseRecords(body, b); err != nil {
		// Unknown record layout: fall back to the tolerant byte-wise scanner.
		b = newBuilder()
		scanRecords(data[len(Magic):], b)
	}
	return b.finish(), nil
}

// trailerLen returns the length of the "ascr … 0xFADEDEAD" trailer, which
// records its own size just before the 0xFADEDEAD marker.
func trailerLen(data []byte) int {
	if len(data) < 6 || !bytes.HasSuffix(data, []byte{0xfa, 0xde, 0xde, 0xad}) {
		return 0
	}
	return int(binary.BigEndian.Uint16(data[len(data)-6:]))
}

// builder accumulates decoded records into a File.
type builder struct {
	f         *File
	seenIdent map[string]int
	seenStr   map[string]bool
	seenInt   map[int]bool
	dc        *decompState
}

func newBuilder() *builder {
	return &builder{
		f:         &File{},
		seenIdent: make(map[string]int),
		seenStr:   make(map[string]bool),
		seenInt:   make(map[int]bool),
		dc: &decompState{
			nodes:     make(map[int16]nodeRec),
			refName:   make(map[int16]string),
			intByRef:  make(map[int16]int),
			textByID:  make(map[int16]string),
			paramMap:  make(map[int16][]int16),
			wraps:     make(map[int16][]int16),
			appByID:   make(map[int16]string),
			evCode:    make(map[int16]string),
			realByID:  make(map[int16]float64),
			descByID:  make(map[int16]descRec),
			comments:  make(map[int16]string),
			osCode:    make(map[int16]string),
			osEnum:    make(map[int16]string),
			piped:     make(map[int16]bool),
			wrapType:  make(map[int16]byte),
			canonical: make(map[string]string),
			objects:   make(map[int16]*object),
		},
	}
}

func (b *builder) finish() *File {
	dc := b.dc
	b.f.Handlers = extractHandlerSigs(dc.nodeOrder, dc.nodes, dc.refName, dc.paramMap)
	b.f.dc = dc
	return b.f
}

// canonicalFlags, set by SCRIPTVIEW_CANONICAL=1, discards the syntax tree's
// surface-syntax flags so its output uses default spellings, as bytecode
// decompilation must. It exists for comparing the two decompilers.
var canonicalFlags = os.Getenv("SCRIPTVIEW_CANONICAL") == "1"

func (b *builder) node(id int16, rec nodeRec) {
	if canonicalFlags && (rec.typ != 'l' || len(rec.children) < 2 || rec.children[1] <= 0) {
		rec.flags = 0 // keep only statement lines' comment styles
	}
	if _, exists := b.dc.nodes[id]; !exists {
		b.dc.nodeOrder = append(b.dc.nodeOrder, id)
	}
	b.dc.nodes[id] = rec
}

func (b *builder) text(id int16, s string) {
	b.dc.textByID[id] = s
	if isDisplayable(s) && !b.seenStr[s] {
		b.seenStr[s] = true
		b.f.Strings = append(b.f.Strings, s)
	}
}

func (b *builder) ident(ref int16, names []string) {
	// Script Editor shows every occurrence of an identifier in the spelling
	// of its first appearance; remember that spelling per lowercase name.
	if best := bestName(names); best != "" {
		if _, seen := b.dc.canonical[strings.ToLower(best)]; !seen {
			b.dc.canonical[strings.ToLower(best)] = best
		}
	}
	// A |piped| identifier is stored as its exact spelling plus an empty
	// name; ordinary ones as lowercase plus original spelling (empty when that
	// is already lowercase), so only mixed-case pipes are detectable.
	if ref < 0 && len(names) == 2 && names[1] == "" && names[0] != strings.ToLower(names[0]) {
		b.dc.piped[ref] = true
	}
	if ref < 0 {
		if best := bestName(names); best != "" {
			if _, exists := b.dc.refName[ref]; !exists {
				b.dc.refName[ref] = best
			}
		}
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if idx, exists := b.seenIdent[key]; exists {
			if b.f.Idents[idx] == key && name != key {
				b.f.Idents[idx] = name
			}
		} else {
			b.seenIdent[key] = len(b.f.Idents)
			b.f.Idents = append(b.f.Idents, name)
		}
	}
}

func (b *builder) integer(ref int16, v int) {
	b.dc.intByRef[ref] = v
	if v >= 0 && !b.seenInt[v] {
		b.seenInt[v] = true
		b.f.Ints = append(b.f.Ints, v)
	}
}

// osType records a 0x0a term reference: kind byte plus payload.
func (b *builder) osType(ref int16, kind byte, data []byte) {
	var m map[int16]string
	var code string
	switch {
	case kind == osKindCode && len(data) == 4:
		m, code = b.dc.osCode, fourCC(data)
	case kind == osKindConstant && len(data) == 8:
		m, code = b.dc.osCode, fourCC(data[4:])
		if _, seen := b.dc.osEnum[ref]; !seen {
			b.dc.osEnum[ref] = fourCC(data[:4])
		}
	case kind == osKindEvent && len(data) >= 8:
		m, code = b.dc.evCode, macRoman(data[:8])
	default:
		return
	}
	if _, seen := m[ref]; !seen {
		m[ref] = code
	}
}

// blob records a 0x0f/0x12 data blob: an alias (application specifier) or an
// AEDesc literal.
func (b *builder) blob(id int16, kind byte, data []byte) {
	// A plain descriptor (kind 0x0d): an alias literal, a date, «data …».
	if kind == 0x0d && len(data) >= 4 {
		b.dc.descByID[id] = descRec{typ: string(data[:4]), data: data[4:]}
		return
	}
	// An application specifier: by alias, or by URL ('aprl', e.g. a SOAP
	// endpoint or a file:// URL).
	if i := bytes.Index(data, []byte("aprl")); i >= 0 && i+4 < len(data) {
		if url := printableRun(data[i+4:]); url != "" {
			name := url
			if rest, ok := strings.CutPrefix(url, "file://localhost"); ok {
				name = strings.TrimSuffix(rest[strings.LastIndex(rest, "/")+1:], ".app")
			}
			b.dc.appByID[id] = name
			return
		}
	}
	if name, ok := aliasAppName(data); ok {
		b.dc.appByID[id] = name
	}
}

// printableRun returns the leading run of printable ASCII in b, skipping
// any length or padding bytes before it.
func printableRun(b []byte) string {
	start := 0
	for start < len(b) && (b[start] < 0x20 || b[start] > 0x7e) {
		start++
	}
	end := start
	for end < len(b) && b[end] >= 0x20 && b[end] <= 0x7e {
		end++
	}
	return string(b[start:end])
}

// parseRecords decodes the FASD body as a strict sequence of objects (see
// objects.go) and feeds each to the builder.
func parseRecords(body []byte, b *builder) error {
	objs, err := readObjects(body)
	if err != nil {
		return err
	}
	for _, o := range objs {
		b.dc.objects[o.ref] = o
		switch o.typ {
		case objList, objBinding, objPointerBlock:
			if len(o.refs) > 0 {
				b.dc.paramMap[o.ref] = o.refs
			}
		case objFixnum:
			b.integer(o.ref, int(int16(o.size)))
		case objLong:
			if len(o.data) == 4 {
				b.integer(o.ref, int(int32(binary.BigEndian.Uint32(o.data))))
			}
		case objFloat:
			if len(o.data) == 8 {
				b.dc.realByID[o.ref] = math.Float64frombits(binary.BigEndian.Uint64(o.data))
			}
		case objString:
			if len(o.data) >= 2 {
				if tl := int(binary.BigEndian.Uint16(o.data)); 2+tl <= len(o.data) {
					b.dc.comments[o.ref] = macRoman(o.data[2 : 2+tl])
				}
			}
		case objDataBlockRaw:
			b.text(o.ref, decodeUTF16BE(o.data))
		case objCodeID:
			b.osType(o.ref, o.kind, o.data)
		case objUserID:
			if o.kind == identTypeMarker {
				b.ident(o.ref, identNames(o.data))
			}
		case objDataBlock, objLongDataBlock:
			b.blob(o.ref, o.kind, o.data)
		case objCmdBlock:
			b.node(o.ref, nodeRec{typ: o.kind, flags: o.flags, start: o.start, end: o.end, children: o.refs})
		case objValueBlock, objValueBlock2:
			b.dc.wraps[o.ref] = o.refs
			b.dc.wrapType[o.ref] = o.kind
		}
	}
	return nil
}

// identNames decodes a 0x0b identifier payload: (len(2), name)… — typically a
// lowercase form followed by the original spelling.
func identNames(data []byte) []string {
	var names []string
	for i := 0; i+2 <= len(data); {
		n := int(binary.BigEndian.Uint16(data[i:]))
		i += 2
		if i+n > len(data) {
			break
		}
		if raw := data[i : i+n]; utf8.Valid(raw) {
			names = append(names, string(raw))
		} else {
			names = append(names, macRoman(raw))
		}
		i += n
	}
	return names
}

// scanRecords is the legacy tolerant scanner: it walks the body byte by byte,
// decoding anything that looks like a known record.
func scanRecords(body []byte, b *builder) {
	r := newReader(body)
	for !r.eof() {
		op, err := r.readByte()
		if err != nil {
			break
		}
		switch op {
		case OpcodeParam, OpcodeRecordCell:
			ref, children := collectParamRecord(r)
			if children != nil {
				b.dc.paramMap[ref] = children
			}
		case OpcodeNode:
			if id, rec := collectNodeRecord(r); rec != nil {
				b.node(id, *rec)
			}
		case OpcodeBlob:
			if id, name, ok := peekAppBlob(r); ok {
				b.dc.appByID[id] = name
			} else if id, d, ok := readDescBlob(r); ok {
				b.dc.descByID[id] = d
			}
		case OpcodeComment:
			if id, text, ok := readComment(r); ok {
				b.dc.comments[id] = text
			}
		case OpcodeLong:
			if ref, v, ok := readSizedRecord(r, 4); ok && ref < 0 {
				b.integer(ref, int(int32(binary.BigEndian.Uint32(v))))
			}
		case OpcodeReal:
			if id, v, ok := readSizedRecord(r, 8); ok && id > 0 {
				b.dc.realByID[id] = math.Float64frombits(binary.BigEndian.Uint64(v))
			}
		case OpcodeText:
			if nodeID, s, _, err := readTextLiteral(r); err == nil {
				b.text(nodeID, s)
			}
		case OpcodeIdent:
			if ref, names, ok, err := readIdentRecord(r); err == nil && ok {
				b.ident(ref, names)
			}
		case OpcodeInt:
			if ref, v, ok, err := readIntLiteral(r); err == nil && ok {
				b.integer(ref, v)
			}
		}
	}
	b.dc.osCode, b.dc.evCode = scanOSTypeNames(body)
}

// extractHandlerSigs finds handler definitions from the node tree and resolves
// their parameter names.
//
// 'I' node layout: children = [name_ref, params_ref, end_ref].
// params_ref is negative (no params) or positive (a 0x02 param-list record).
// Each 0x02 record child that is a positive node ID of type 'o' carries the
// ident ref for one parameter name.
func extractHandlerSigs(order []int16, nodes map[int16]nodeRec, refName map[int16]string, paramMap map[int16][]int16) []HandlerSig {
	var sigs []HandlerSig
	seen := make(map[string]bool)

	for _, id := range order {
		rec := nodes[id]
		if rec.typ != 'i' {
			continue
		}
		for _, childRef := range rec.children {
			if childRef <= 0 {
				continue
			}
			sig, ok := nodes[childRef]
			if !ok || sig.typ != 'I' || len(sig.children) == 0 {
				continue
			}
			nameRef := sig.children[0]
			if nameRef >= 0 {
				continue
			}
			name, ok := refName[nameRef]
			if !ok || !isValidIdent(name) {
				continue
			}
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true

			var params []string
			if len(sig.children) >= 2 {
				paramsRef := sig.children[1]
				if paramsRef > 0 {
					params = resolveParams(paramsRef, nodes, refName, paramMap)
				}
			}

			sigs = append(sigs, HandlerSig{Name: name, Params: params})
		}
	}
	return sigs
}

// resolveParams resolves a handler signature's positional parameter names
// from its cons chain ([head, tail] cells whose heads are 'o' variable nodes).
func resolveParams(paramsRef int16, nodes map[int16]nodeRec, refName map[int16]string, paramMap map[int16][]int16) []string {
	var params []string
	seen := make(map[int16]bool)
	for ref := paramsRef; ref > 0 && !seen[ref]; {
		seen[ref] = true
		cell, ok := paramMap[ref]
		if !ok || len(cell) != 2 {
			break
		}
		if o, ok := nodes[cell[0]]; ok && o.typ == 'o' && len(o.children) == 1 {
			if name, ok := refName[o.children[0]]; ok {
				params = append(params, name)
			}
		}
		ref = cell[1]
	}
	return params
}

// collectParamRecord reads a 0x02 parameter-list record (opcode already consumed).
// Format: ref(2BE) + count(2BE) + count×2 child refs.
func collectParamRecord(r *reader) (int16, []int16) {
	if r.remaining() < 4 {
		return 0, nil
	}
	ref, err := r.readI16BE()
	if err != nil {
		return 0, nil
	}
	count, err := r.readU16BE()
	if err != nil {
		return ref, nil
	}
	if ref <= 0 || count == 0 || count > 20 {
		return ref, nil
	}
	if r.remaining() < int(count)*2 {
		return ref, nil
	}
	children := make([]int16, count)
	for i := range children {
		v, err := r.readI16BE()
		if err != nil {
			return ref, nil
		}
		children[i] = v
	}
	return ref, children
}

// collectNodeRecord reads a 0x0d node record body (opcode already consumed),
// returning its ID and structure. Returns nil rec for spurious 0x0d bytes.
func collectNodeRecord(r *reader) (int16, *nodeRec) {
	const maxNodeSkip = 300

	if r.remaining() < 5 {
		return 0, nil
	}

	peek := r.data[r.pos : r.pos+5]
	id := int16(uint16(peek[0])<<8 | uint16(peek[1]))
	count := int(uint16(peek[2])<<8 | uint16(peek[3]))
	typ := peek[4]

	if id > 500 || id < -500 || count > 100 || typ < 0x20 || typ > 0x7e {
		return 0, nil
	}
	skip := 11 + count*2
	if skip > maxNodeSkip {
		return 0, nil
	}
	if r.remaining() < skip {
		return 0, nil
	}

	raw, err := r.readBytes(skip)
	if err != nil {
		return 0, nil
	}

	// Child refs start at byte 11 (after id:2, count:2, type:1, fixed:6).
	children := make([]int16, count)
	for i := range children {
		children[i] = int16(uint16(raw[11+i*2])<<8 | uint16(raw[12+i*2]))
	}
	flags := uint16(raw[5])<<8 | uint16(raw[6])
	return id, &nodeRec{typ: typ, flags: flags, children: children}
}

// readTextLiteral reads the body of a 0x11 text-literal record.
// Format: node_id(2BE signed) + byte_len(2BE) + UTF-16BE string
func readTextLiteral(r *reader) (nodeID int16, s string, ok bool, err error) {
	nodeID, err = r.readI16BE()
	if err != nil {
		return 0, "", false, err
	}
	byteLen, err := r.readU16BE()
	if err != nil {
		return nodeID, "", false, err
	}
	if byteLen > 20000 || byteLen%2 != 0 {
		return nodeID, "", false, nil
	}
	raw, err := r.readBytes(int(byteLen))
	if err != nil {
		return nodeID, "", false, err
	}
	s = decodeUTF16BE(raw)
	if !isDisplayable(s) {
		return nodeID, s, false, nil
	}
	return nodeID, s, true, nil
}

// readIdentRecord reads the body of a 0x0b identifier record.
// Returns the raw ref (for handler detection), both name variants, and ok.
//
// Format: ref(2BE signed) + data_len(2BE) + identifier-data
func readIdentRecord(r *reader) (ref int16, names []string, ok bool, err error) {
	ref, err = r.readI16BE()
	if err != nil {
		return 0, nil, false, err
	}
	dataLen, err := r.readU16BE()
	if err != nil {
		return ref, nil, false, err
	}
	if ref >= 0 || dataLen == 0 || dataLen > 512 {
		return ref, nil, false, nil
	}
	readLen := int(dataLen) + 1
	if readLen > r.remaining() {
		readLen = int(dataLen)
	}
	raw, err := r.readBytes(readLen)
	if err != nil {
		return ref, nil, false, err
	}
	names = parseIdentData(raw)
	return ref, names, true, nil
}

// parseIdentData extracts identifier names from the raw data of a 0x0b record.
// The data starts with a 0x30 type marker followed by entries of (len:2BE, name:bytes).
func parseIdentData(data []byte) []string {
	if len(data) == 0 || data[0] != identTypeMarker {
		return nil
	}
	var names []string
	i := 1
	for i+1 < len(data) {
		nameLen := int(data[i])<<8 | int(data[i+1])
		i += 2
		if nameLen == 0 || i+nameLen > len(data) {
			break
		}
		name := string(data[i : i+nameLen])
		if isValidIdent(name) {
			names = append(names, name)
		}
		i += nameLen
	}
	return names
}

// readIntLiteral reads the body of a 0x03 integer-literal record.
// Format: ref(2BE signed) + value(2BE signed)
func readIntLiteral(r *reader) (ref int16, v int, ok bool, err error) {
	ref, err = r.readI16BE()
	if err != nil {
		return 0, 0, false, err
	}
	if ref >= 0 {
		_, err = r.readI16BE()
		return ref, 0, false, err
	}
	raw, err := r.readI16BE()
	if err != nil {
		return ref, 0, false, err
	}
	return ref, int(raw), true, nil
}

// bestName returns the most informative name from a list: prefers mixed-case
// over all-lowercase (which is the compiler-generated duplicate).
func bestName(names []string) string {
	for _, n := range names {
		if n != strings.ToLower(n) {
			return n
		}
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

// isValidIdent returns true if s looks like a plausible AppleScript identifier.
func isValidIdent(s string) bool {
	if len(s) == 0 || len(s) > 200 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// decodeUTF16BE converts a byte slice of UTF-16 big-endian codepoints to a Go string.
func decodeUTF16BE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
	}
	return string(utf16.Decode(u16))
}

// aliasAppName extracts the application name from blob data embedding an
// alias record ("alis"), from the alias's filename field, minus ".app".
func aliasAppName(blob []byte) (string, bool) {
	i := bytes.Index(blob, []byte("alis"))
	if i < 0 {
		return "", false
	}
	// "alis" + length(4), then the alias record: userType(4) size(2) version(2)
	// kind(2) volumeName(str27, 28 bytes) createDate(4) fsType(2) driveType(2)
	// parentDirID(4) fileName(str63) — i.e. the filename Pascal string is at +50.
	fn := i + 4 + 4 + 50 - 4
	if fn >= len(blob) {
		return "", false
	}
	l := int(blob[fn])
	if l == 0 || l > 63 || fn+1+l > len(blob) {
		return "", false
	}
	name := macRoman(blob[fn+1 : fn+1+l])
	// Script Editor names an application it can find by its display name,
	// like the applications that ship with macOS.
	if appInstalled(blob[i+4:], name) {
		name = strings.TrimSuffix(name, ".app")
	}
	return name, true
}

// peekAppBlob inspects a 0x0f blob record (opcode already consumed) without
// advancing the reader. If the blob embeds an alias record ("alis"), it returns
// the application name taken from the alias's filename field, minus ".app".
func peekAppBlob(r *reader) (int16, string, bool) {
	if r.remaining() < 4 {
		return 0, "", false
	}
	hdr := r.data[r.pos : r.pos+4]
	id := int16(uint16(hdr[0])<<8 | uint16(hdr[1]))
	n := int(uint16(hdr[2])<<8 | uint16(hdr[3]))
	if id <= 0 || n < 64 || r.remaining() < 4+n {
		return 0, "", false
	}
	blob := r.data[r.pos+4 : r.pos+4+n]
	i := bytes.Index(blob, []byte("alis"))
	if i < 0 {
		return 0, "", false
	}
	// "alis" + length(4), then the alias record: userType(4) size(2) version(2)
	// kind(2) volumeName(str27, 28 bytes) createDate(4) fsType(2) driveType(2)
	// parentDirID(4) fileName(str63) — i.e. the filename Pascal string is at +50.
	fn := i + 4 + 4 + 50 - 4
	if fn >= len(blob) {
		return 0, "", false
	}
	l := int(blob[fn])
	if l == 0 || l > 63 || fn+1+l > len(blob) {
		return 0, "", false
	}
	name := string(blob[fn+1 : fn+1+l])
	return id, strings.TrimSuffix(name, ".app"), true
}

// readSizedRecord reads ref(2) + len(2) + data when len equals want; otherwise
// it consumes nothing so the scanner can resync byte-wise.
func readSizedRecord(r *reader, want int) (int16, []byte, bool) {
	if r.remaining() < 4+want {
		return 0, nil, false
	}
	hdr := r.data[r.pos : r.pos+4]
	if int(uint16(hdr[2])<<8|uint16(hdr[3])) != want {
		return 0, nil, false
	}
	ref := int16(uint16(hdr[0])<<8 | uint16(hdr[1]))
	r.pos += 4
	data, _ := r.readBytes(want)
	return ref, data, true
}

// descRec is an AEDesc literal (date, «data …») stored in a 0x0f blob.
type descRec struct {
	typ  string
	data []byte
}

// readDescBlob reads a 0x0f blob holding an AEDesc:
// id(2) len(2) kind(1)=0x0d type(4) data(len-4).
func readDescBlob(r *reader) (int16, descRec, bool) {
	if r.remaining() < 9 {
		return 0, descRec{}, false
	}
	b := r.data[r.pos:]
	id := int16(uint16(b[0])<<8 | uint16(b[1]))
	n := int(uint16(b[2])<<8 | uint16(b[3]))
	if id <= 0 || n < 4 || b[4] != 0x0d || r.remaining() < 5+n || !isFourCC(b[5:9]) {
		return 0, descRec{}, false
	}
	d := descRec{typ: string(b[5:9]), data: b[9 : 5+n]}
	r.pos += 5 + n
	return id, d, true
}

// readComment reads a 0x0c source-comment record:
// id(2) len(2) textLen(2) text(textLen, Mac Roman/ASCII) trailer(4).
// It consumes nothing unless the lengths are self-consistent.
func readComment(r *reader) (int16, string, bool) {
	if r.remaining() < 6 {
		return 0, "", false
	}
	b := r.data[r.pos:]
	id := int16(uint16(b[0])<<8 | uint16(b[1]))
	n := int(uint16(b[2])<<8 | uint16(b[3]))
	tl := int(uint16(b[4])<<8 | uint16(b[5]))
	if id <= 0 || n != 2+tl+4 || r.remaining() < 4+n {
		return 0, "", false
	}
	text := string(b[6 : 6+tl])
	r.pos += 4 + n
	return id, text, true
}
