package scpt

import (
	"fmt"
	"strings"
)

// name resolves a negative ref to an identifier, command, or term name.
func (dc *decompState) name(ref int16) string {
	return dc.nameAs(ref, kindValue)
}

// className resolves ref in class position (every X, X 1, …).
func (dc *decompState) className(ref int16) string {
	return dc.nameAs(ref, kindClass)
}

func (dc *decompState) nameAs(ref int16, kind termKind) string {
	if dc.wrapType[ref] == wrapAppTerm {
		// A term compiled against a specific application's dictionary.
		if w := dc.wraps[ref]; len(w) == 2 {
			if app := dc.appOf(w[1]); app != "" {
				dc.scopes = append(dc.scopes, app)
				defer func() { dc.scopes = dc.scopes[:len(dc.scopes)-1] }()
			}
			return dc.nameAs(w[0], kind)
		}
	}
	if name := dc.refName[ref]; name != "" {
		if dc.piped[ref] || dc.isTermWord(name) {
			return "|" + name + "|"
		}
		return quoteIdent(name)
	}
	if code, ok := dc.evCode[ref]; ok {
		return dc.eventName(code)
	}
	if code, ok := dc.osCode[ref]; ok {
		if enum, ok := dc.osEnum[ref]; ok {
			for _, d := range dc.inScope() {
				if name, ok := d.qenums[enum+"/"+code]; ok {
					return name
				}
			}
		}
		if enum, ok := dc.osEnum[ref]; ok && enum != "enum" && enum != "boov" && enum != "type" && enum != "****" {
			// A typed constant from an enumeration nobody in scope defines.
			return "«constant " + rawCode(enum) + rawCode(code) + "»"
		}
		return dc.term(code, kind)
	}
	return fmt.Sprintf("<ref%d>", ref)
}

// className4 returns the class name for a code in scope, or "".
func (dc *decompState) className4(code string) string {
	for _, d := range dc.inScope() {
		if d == nil || d == builtinDict {
			continue
		}
		if name, ok := d.classes[code]; ok {
			return name
		}
	}
	return ""
}

// anyTerm4 names a code by a class or else an AppleScript property.
func (dc *decompState) anyTerm4(code string) string {
	if name := dc.className4(code); name != "" {
		return name
	}
	return dicts["AppleScript"].props[code] // time:, but not application properties
}

// kindOrder lists the namespaces searched for a term of the given kind.
func kindOrder(kind termKind) []func(*dict) map[string]string {
	props := func(d *dict) map[string]string { return d.props }
	classes := func(d *dict) map[string]string { return d.classes }
	enums := func(d *dict) map[string]string { return d.enums }
	switch kind {
	case kindClass:
		return []func(*dict) map[string]string{classes, props, enums}
	case kindProperty:
		return []func(*dict) map[string]string{props, classes}
	}
	return []func(*dict) map[string]string{props, classes, enums}
}

// resolvable reports whether ref names anything.
func (dc *decompState) resolvable(ref int16) bool {
	if dc.wrapType[ref] == wrapAppTerm {
		return true
	}
	_, a := dc.refName[ref]
	_, b := dc.evCode[ref]
	_, c := dc.osCode[ref]
	return a || b || c
}

// usesWithoutAdditions reports whether the script has `use` statements but
// none for scripting additions.
func (dc *decompState) usesWithoutAdditions() bool {
	uses, additions := false, false
	for _, n := range dc.nodes {
		if n.typ != 'x' {
			continue
		}
		uses = true
		if t, ok := dc.nodes[child(n, 1)]; ok && t.typ == '2' && dc.osCode[child0(t)] == "osax" {
			additions = true
		}
	}
	return uses && !additions
}

// usedApps returns the applications that `use` statements name.
func (dc *decompState) usedApps() []string {
	var apps []string
	for _, id := range dc.nodeOrder {
		if n := dc.nodes[id]; n.typ == 'x' {
			if app := dc.appOf(child(n, 1)); app != "" {
				apps = append(apps, app)
			}
		}
	}
	return apps
}

// isTermWord reports whether a user identifier spells a term in scope, so
// that it must be written |piped| to stay an identifier.
func (dc *decompState) isTermWord(name string) bool {
	lower := strings.ToLower(name)
	// Only the innermost application's terms clash, even when its
	// dictionary is unknown.
	var app *dict
	if len(dc.scopes) > 0 {
		app = scopeDict(dc.scopes[len(dc.scopes)-1])
	}
	for _, d := range dc.inScope() {
		if d != coreDict && d != builtinDict && d != dicts[""] && d != dicts["AppleScript"] && d != app {
			continue
		}
		if d == dicts["AppleScript"] && typeNameWords[lower] && !dc.noAdditions {
			return true
		}
		if d == builtinDict || d == coreDict || (d == dicts["AppleScript"] && unpipedWords[lower]) {
			continue
		}
		if d == dicts[""] && dc.noAdditions {
			continue // Standard Additions words only clash when they are loaded
		}
		if d.hasWord(lower) {
			return true
		}
	}
	return false
}

// isBuiltin reports whether ref is a command defined by a dictionary rather
// than a user handler.
func (dc *decompState) isBuiltin(ref int16) bool {
	_, user := dc.refName[ref]
	_, ev := dc.evCode[ref]
	return ev && !user
}

// scopeDict returns the terminology of the application named scope.
func scopeDict(scope string) *dict {
	if alias, ok := scopeAliases[scope]; ok {
		scope = alias
	}
	if strings.HasPrefix(scope, "http://") || strings.HasPrefix(scope, "https://") {
		scope = webServiceScope
	}
	return dicts[scope]
}

// inScope returns the dictionaries to search, innermost tell target first,
// then the AppleScript built-ins and the always-loaded dictionaries.
func (dc *decompState) inScope() []*dict {
	// Only the innermost application's terminology is in scope.
	var out []*dict
	if len(dc.scopes) > 0 {
		if d := scopeDict(dc.scopes[len(dc.scopes)-1]); d != nil {
			out = append(out, d)
		}
	}
	return append([]*dict{coreDict}, append(out, dicts[""], dicts["AppleScript"], builtinDict)...)
}

// term names a four-char code, falling back to raw «class xxxx» syntax.
func (dc *decompState) term(code string, kind termKind) string {
	// Tell targets' dictionaries win outright; among the always-loaded ones
	// (Standard Additions, Cocoa, AppleScript) the term kind decides, so a
	// property beats a constant with the same code.
	var global []*dict
	for _, d := range dc.inScope() {
		if d == nil {
			continue
		}
		if d == dicts[""] || d == dicts["AppleScript"] || d == builtinDict {
			global = append(global, d)
			continue
		}
		if name := d.lookup(code, kind); name != "" {
			return name
		}
	}
	for _, m := range kindOrder(kind) {
		for _, d := range global {
			if name, ok := m(d)[code]; ok {
				return name
			}
		}
	}
	if name, ok := synonymTerms[code]; ok {
		return name
	}
	return "«class " + rawCode(code) + "»"
}

// pluralOf returns the plural form of the class behind ref.
func (dc *decompState) pluralOf(ref int16) string {
	if code, ok := dc.osCode[ref]; ok {
		for _, d := range dc.inScope() {
			if d == nil {
				continue
			}
			if p := d.plurals[code]; p != "" {
				return p
			}
			if d.lookup(code, kindClass) != "" {
				break
			}
		}
	}
	if dc.osCode[ref] == "type" {
		return "types" // not "type classes"
	}
	name := dc.className(ref)
	if strings.HasPrefix(name, "«") {
		return "every " + name // raw codes have no plural form
	}
	return plural(name)
}

func (dc *decompState) eventName(code string) string {
	for _, d := range dc.inScope() {
		if d == nil {
			continue
		}
		if name, ok := d.events[code]; ok {
			return name
		}
	}
	if name, ok := synonymEvents[code]; ok {
		return name
	}
	return "«event " + code + "»"
}

// paramLabel names a command's keyword parameter.
func (dc *decompState) paramLabel(event string, ref int16) string {
	code, ok := dc.osCode[ref]
	if !ok {
		return dc.name(ref)
	}
	for _, d := range dc.inScope() {
		if d == nil {
			continue
		}
		if name, ok := d.params[event+"/"+code]; ok {
			return name
		}
	}
	if name, ok := paramNames[code]; ok && !strings.HasPrefix(dc.eventName(event), "«") {
		return name
	}
	return "«class " + rawCode(code) + "»"
}

// Wrapper (0x0e) record types.
const (
	wrapText      = 0xb1 // string literal: [text record]
	wrapSpecifier = 0x15 // object specifier: [container, key]
	wrapAppTerm   = 0x7a // term from an application's dictionary: [term, application]
	wrapOptional  = 0x7c // optional handler parameter: [_, _, parameter, _, default]
)

// wrappedText returns the string behind a literal payload ref: a 0x0e wrapper
// whose child is the 0x11 text record. The legacy scanner does not record
// wrappers, but the text record always directly follows its wrapper.
func (dc *decompState) wrappedText(id int16) (string, bool) {
	if w, ok := dc.wraps[id]; ok && len(w) > 0 {
		t, ok := dc.textByID[w[0]]
		return t, ok
	}
	t, ok := dc.textByID[id+1]
	return t, ok
}

// appOf returns the application name targeted by expression id, if it is an
// application specifier.
func (dc *decompState) appOf(id int16) string {
	if dc.helperDepth > maxNesting {
		return "" // damaged file: cyclic references
	}
	dc.helperDepth++
	defer func() { dc.helperDepth-- }()
	if app, ok := dc.appByID[id]; ok {
		return strings.TrimSuffix(app, ".app")
	}
	n, ok := dc.nodes[id]
	if !ok {
		return ""
	}
	switch n.typ {
	case 'm', 'l':
		return dc.appOf(firstPositive(n))
	case 'n': // … of application "X"
		return dc.appOf(child(n, 1))
	case '5': // application id "com.example.App"
		if len(n.children) >= 2 && dc.osCode[n.children[0]] == "capp" {
			if lit, ok := dc.nodes[dc.unwrap(n.children[1])]; ok && lit.typ == 'm' {
				if t, ok := dc.wrappedText(child(lit, 0)); ok {
					return bundleScopes[strings.ToLower(t)]
				}
			}
		}
	case '4': // application "Name"
		if len(n.children) == 2 && dc.osCode[n.children[0]] == "capp" {
			if lit, ok := dc.nodes[dc.unwrap(n.children[1])]; ok && lit.typ == 'm' {
				if t, ok := dc.wrappedText(child(lit, 0)); ok {
					return strings.TrimSuffix(t, ".app")
				}
			}
		}
	}
	return ""
}
