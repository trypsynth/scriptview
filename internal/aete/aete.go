// Package aete reads classic Mac OS scripting dictionaries: aete and aeut
// resources, as AppleScript's runtime and old applications store them.
package aete

import (
	"encoding/binary"
	"errors"
	"maps"
	"slices"
	"strings"
)

// Term is one dictionary entry. Kind is v (command), a (command parameter,
// Code "event/keyword"), c (class), p (property), e (enumerator), E
// (enumerator, Code "enum/code"), or x/y for words that need no pipes.
type Term struct {
	Scope, Kind, Code, Name, Plural string
}

// Resource returns resource (typ, id) from resource fork data.
func Resource(d []byte, typ string, id int16) (out []byte, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, errors.New("resource fork: truncated")
		}
	}()
	if len(d) < 16 {
		return nil, errors.New("resource file too short")
	}
	be := binary.BigEndian
	dataOff, mapOff := int(be.Uint32(d)), int(be.Uint32(d[4:]))
	m := d[mapOff:]
	typeList := m[be.Uint16(m[24:]):]
	nTypes := int(int16(be.Uint16(typeList))) + 1
	for i := 0; i < nTypes; i++ {
		e := typeList[2+i*8:]
		if string(e[:4]) != typ {
			continue
		}
		count := int(int16(be.Uint16(e[4:]))) + 1
		refs := typeList[be.Uint16(e[6:]):]
		for j := 0; j < count; j++ {
			r := refs[j*12:]
			if int16(be.Uint16(r)) != id {
				continue
			}
			off := dataOff + int(be.Uint32(r[4:])&0xffffff)
			n := int(be.Uint32(d[off:]))
			return d[off+4 : off+4+n], nil
		}
	}
	return nil, errors.New("resource not found")
}

// Parse decodes an aete/aeut resource into terms for scope.
func Parse(d []byte, scope string) (terms []Term, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("aete: truncated")
		}
	}()
	p := 0
	u16 := func() int { v := int(binary.BigEndian.Uint16(d[p:])); p += 2; return v }
	c4 := func() string { v := MacRoman(d[p : p+4]); p += 4; return v }
	ps := func() string { n := int(d[p]); v := MacRoman(d[p+1 : p+1+n]); p += 1 + n; return v }
	align := func() { p += p & 1 }

	u16() // version
	u16() // language
	u16() // script
	// Script Editor leaves identifiers unpiped when they only spell terms
	// from these legacy suites (e.g. translation, table, display, pixel, null).
	unpipedSuites := map[string]bool{"qdsp": true, "qdrw": true, "tbls": true, "macc": true, "tpnm": true}
	suitesOf := map[string]map[string]bool{}
	note := func(name, suite string) {
		key := strings.ToLower(name)
		if suitesOf[key] == nil {
			suitesOf[key] = map[string]bool{}
		}
		suitesOf[key][suite] = true
	}
	// Classes and properties defined only in the legacy suites print as raw
	// codes («class ccel», not cell): AppleScript switches a suite on only
	// when an application's dictionary names it.
	legacyOnly := map[string]bool{}
	defer func() {
		kept := terms[:0]
		for _, t := range terms {
			if (t.Kind == "c" || t.Kind == "p") && legacyOnly[t.Kind+t.Code] {
				continue
			}
			kept = append(kept, t)
		}
		terms = kept
	}()
	codeSuites := map[string]map[string]bool{}
	noteCode := func(kind, code, suite string) {
		key := kind + code
		if codeSuites[key] == nil {
			codeSuites[key] = map[string]bool{}
		}
		codeSuites[key][suite] = true
		only := true
		for s := range codeSuites[key] {
			only = only && unpipedSuites[s] && s != "tpnm"
		}
		legacyOnly[key] = only
	}
	defer func() {
		for _, word := range slices.Sorted(maps.Keys(suitesOf)) {
			only := true
			for s := range suitesOf[word] {
				only = only && unpipedSuites[s]
			}
			if only {
				// Standard Additions' own aete names the Type Names suite,
				// which switches the whole suite on while it is loaded.
				kind := "x"
				if suitesOf[word]["tpnm"] {
					kind = "y"
				}
				terms = append(terms, Term{Scope: scope, Kind: kind, Code: word, Name: word})
			}
		}
	}()
	for suites := u16(); suites > 0; suites-- {
		ps()
		ps()
		align()
		suite := c4()
		u16()
		u16()
		for events := u16(); events > 0; events-- {
			name := ps()
			ps()
			align()
			code := c4() + c4()
			c4() // reply type
			ps()
			align()
			u16()
			c4() // direct parameter type
			ps()
			align()
			u16()
			terms = append(terms, Term{Scope: scope, Kind: "v", Code: code, Name: name})
			note(name, suite)
			for params := u16(); params > 0; params-- {
				pname := ps()
				align()
				kw := c4()
				c4()
				ps()
				align()
				u16()
				terms = append(terms, Term{Scope: scope, Kind: "a", Code: code + "/" + kw, Name: pname})
			}
		}
		for classes := u16(); classes > 0; classes-- {
			name := ps()
			align()
			code := c4()
			ps()
			align()
			var props []Term
			isPlural := false
			for n := u16(); n > 0; n-- {
				pname := ps()
				align()
				pcode := c4()
				c4()
				ps()
				align()
				u16()
				switch pcode {
				case "c@#!": // marks this entry as the plural form of the class
					isPlural = true
				case "c@#^": // inheritance
				default:
					props = append(props, Term{Scope: scope, Kind: "p", Code: pcode, Name: pname})
				}
			}
			note(name, suite)
			noteCode("c", code, suite)
			for _, p := range props {
				note(p.Name, suite)
				noteCode("p", p.Code, suite)
			}
			if isPlural {
				for i := len(terms) - 1; i >= 0; i-- {
					if terms[i].Kind == "c" && terms[i].Code == code && terms[i].Scope == scope {
						terms[i].Plural = name
						break
					}
				}
			} else {
				terms = append(terms, Term{Scope: scope, Kind: "c", Code: code, Name: name})
			}
			terms = append(terms, props...)
			for elems := u16(); elems > 0; elems-- {
				c4()
				for forms := u16(); forms > 0; forms-- {
					c4()
				}
			}
		}
		for comps := u16(); comps > 0; comps-- {
			ps()
			align()
			c4()
			ps()
			align()
		}
		for enums := u16(); enums > 0; enums-- {
			ecode := c4()
			for n := u16(); n > 0; n-- {
				name := ps()
				align()
				code := c4()
				ps()
				align()
				terms = append(terms, Term{Scope: scope, Kind: "e", Code: code, Name: name})
				note(name, suite)
				terms = append(terms, Term{Scope: scope, Kind: "E", Code: ecode + "/" + code, Name: name})
			}
		}
	}
	return terms, nil
}

var macRomanHigh = []rune("ÄÅÇÉÑÖÜáàâäãåçéèêëíìîïñóòôöõúùûü†°¢£§•¶ß®©™´¨≠ÆØ∞±≤≥¥µ∂∑∏π∫ªºΩæø¿¡¬√ƒ≈∆«»… ÀÃÕŒœ–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ")

// MacRoman decodes Mac OS Roman text.
func MacRoman(b []byte) string {
	out := make([]rune, len(b))
	for i, c := range b {
		if c < 0x80 {
			out[i] = rune(c)
		} else {
			out[i] = macRomanHigh[c-0x80]
		}
	}
	return string(out)
}
