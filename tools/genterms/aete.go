//go:build darwin

package main

import (
	"encoding/binary"
	"errors"
	"maps"
	"os"
	"slices"
	"strings"
)

// appleScriptRsrc holds AppleScript's own built-in terminology as an 'aeut'
// resource (classic aete format) in a data-fork resource file.
const appleScriptRsrc = "/System/Library/Components/AppleScript.component/Contents/Resources/AppleScript.rsrc"

// readResource returns resource (typ, id) from a data-fork resource file.
func readResource(path, typ string, id int16) ([]byte, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
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

// parseAete decodes an aete/aeut resource into terms for scope.
func parseAete(d []byte, scope string) (terms []term, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("aete: truncated")
		}
	}()
	p := 0
	u16 := func() int { v := int(binary.BigEndian.Uint16(d[p:])); p += 2; return v }
	c4 := func() string { v := macRoman(d[p : p+4]); p += 4; return v }
	ps := func() string { n := int(d[p]); v := macRoman(d[p+1 : p+1+n]); p += 1 + n; return v }
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
				terms = append(terms, term{scope: scope, kind: kind, code: word, name: word})
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
			terms = append(terms, term{scope: scope, kind: "v", code: code, name: name})
			note(name, suite)
			for params := u16(); params > 0; params-- {
				pname := ps()
				align()
				kw := c4()
				c4()
				ps()
				align()
				u16()
				terms = append(terms, term{scope: scope, kind: "a", code: code + "/" + kw, name: pname})
			}
		}
		for classes := u16(); classes > 0; classes-- {
			name := ps()
			align()
			code := c4()
			ps()
			align()
			var props []term
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
					props = append(props, term{scope: scope, kind: "p", code: pcode, name: pname})
				}
			}
			note(name, suite)
			for _, p := range props {
				note(p.name, suite)
			}
			if isPlural {
				for i := len(terms) - 1; i >= 0; i-- {
					if terms[i].kind == "c" && terms[i].code == code && terms[i].scope == scope {
						terms[i].plural = name
						break
					}
				}
			} else {
				terms = append(terms, term{scope: scope, kind: "c", code: code, name: name})
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
				terms = append(terms, term{scope: scope, kind: "e", code: code, name: name})
				note(name, suite)
				terms = append(terms, term{scope: scope, kind: "E", code: ecode + "/" + code, name: name})
			}
		}
	}
	return terms, nil
}

var macRomanHigh = []rune("ÄÅÇÉÑÖÜáàâäãåçéèêëíìîïñóòôöõúùûü†°¢£§•¶ß®©™´¨≠ÆØ∞±≤≥¥µ∂∑∏π∫ªºΩæø¿¡¬√ƒ≈∆«»… ÀÃÕŒœ–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ")

func macRoman(b []byte) string {
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
