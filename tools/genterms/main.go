//go:build darwin

// genterms extracts scripting terminology from the sdef dictionaries of
// Standard Additions and common macOS applications into
// internal/scpt/terms.tsv, so decompilation can name app-specific classes,
// properties, constants and commands on any platform.
//
// Run from the repo root:
//
//	go run ./tools/genterms
//
// Output format, one term per line:
//
//	scope <TAB> kind <TAB> code <TAB> name [<TAB> plural]
//
// scope is "" for Standard Additions (always loaded unless a script's use
// statements exclude it), "AppleScript" for AppleScript's built-in terms,
// "CocoaStandard", or an application name. kind is one of
// c (class), p (property), e (enumerator), E (enumerator qualified by its
// enumeration: code is enumeration code + "/" + enumerator code), v
// (command/event), a (command parameter; code is event code + "/" +
// parameter code), b (bundle identifier of the scope's application), n
// (name of an application that ships with macOS), sc/sp/sv (legacy synonym
// code for a class/property/command, used only when nothing else matches).
package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// sources lists dictionaries in priority order: earlier entries win when two
// dictionaries in the same scope define the same code.
var sources = []struct{ scope, path string }{
	{"", "/System/Library/ScriptingAdditions/StandardAdditions.osax"},
	// Cocoa Standard terms reach scripts only through applications that
	// include them; they are not part of the global terminology.
	{"CocoaStandard", "/System/Library/ScriptingDefinitions/CocoaStandard.sdef"},
	{"Finder", "/System/Library/CoreServices/Finder.app"},
	{"System Events", "/System/Library/CoreServices/System Events.app"},
	{"Image Events", "/System/Library/CoreServices/Image Events.app"},
	{"Database Events", "/System/Library/CoreServices/Database Events.app"},
	{"Folder Actions Setup", "/System/Library/CoreServices/Applications/Folder Actions Setup.app"},
	{"Shortcuts Events", "/System/Library/CoreServices/Shortcuts Events.app"},
	{"VoiceOver", "/System/Library/CoreServices/VoiceOver.app"},
	{"Script Editor", "/System/Applications/Utilities/Script Editor.app"},
	{"Terminal", "/System/Applications/Utilities/Terminal.app"},
	{"System Settings", "/System/Applications/System Settings.app"},
	{"Safari", "/Applications/Safari.app"},
	{"Mail", "/System/Applications/Mail.app"},
	{"Music", "/System/Applications/Music.app"},
	{"TV", "/System/Applications/TV.app"},
	{"Podcasts", "/System/Applications/Podcasts.app"},
	{"TextEdit", "/System/Applications/TextEdit.app"},
	{"Preview", "/System/Applications/Preview.app"},
	{"Calendar", "/System/Applications/Calendar.app"},
	{"Notes", "/System/Applications/Notes.app"},
	{"Reminders", "/System/Applications/Reminders.app"},
	{"Messages", "/System/Applications/Messages.app"},
	{"Contacts", "/System/Applications/Contacts.app"},
	{"Photos", "/System/Applications/Photos.app"},
	{"QuickTime Player", "/System/Applications/QuickTime Player.app"},
	{"Automator", "/System/Applications/Automator.app"},
	{"Font Book", "/System/Applications/Font Book.app"},
	{"Image Capture", "/System/Applications/Image Capture.app"},
	{"Console", "/System/Applications/Utilities/Console.app"},
	{"Xcode", "/Applications/Xcode.app"},
}

type node struct {
	XMLName  xml.Name
	Name     string `xml:"name,attr"`
	Code     string `xml:"code,attr"`
	Plural   string `xml:"plural,attr"`
	Hidden   string `xml:"hidden,attr"`
	Children []node `xml:",any"`
}

// webServiceScope holds the terminology of SOAP and XML-RPC servers.
const webServiceScope = "http://"

type term struct {
	scope, kind, code, name, plural string
	hidden                          bool
}

func main() {
	// Within one dictionary a later definition of a code replaces an earlier
	// one (as AppleScript does); across dictionaries in one scope, the first
	// dictionary wins.
	type slot struct{ index, source int }
	seen := make(map[string]slot)
	var terms []term
	source := 0
	add := func(t term) {
		key := t.scope + "\x00" + t.kind + "\x00" + t.code
		if t.code == "" || t.name == "" {
			return
		}
		if prev, ok := seen[key]; ok {
			// Later definitions win, except that a hidden (legacy) term
			// never replaces a visible one.
			// Command parameters keep their first spelling (using delimiter,
			// not the later using delimiters), as do AppleScript's own
			// properties (id, not a later ID).
			firstWins := t.kind == "a" || (t.kind == "p" && t.scope == "AppleScript")
			if prev.source == source && !(t.hidden && !terms[prev.index].hidden) && !firstWins {
				if t.plural == "" {
					t.plural = terms[prev.index].plural
				}
				terms[prev.index] = t
			} else if prev.source != source && terms[prev.index].hidden && !t.hidden {
				terms[prev.index] = t
			}
			return
		}
		seen[key] = slot{len(terms), source}
		terms = append(terms, t)
	}

	for i, src := range withSystemApps(sources) {
		source = i
		if _, err := os.Stat(src.path); err != nil {
			log.Printf("skip %s: %v", src.path, err)
			continue
		}
		data, err := readSdef(src.path)
		if err != nil {
			log.Printf("skip %s: %v", src.path, err)
			continue
		}
		var root node
		if err := xml.Unmarshal(data, &root); err != nil {
			log.Printf("skip %s: %v", src.path, err)
			continue
		}
		if src.scope != "" {
			// Applications can be targeted by bundle identifier or by
			// creator code: application id "com.apple.finder" / "MACS".
			for _, key := range []string{"CFBundleIdentifier", "CFBundleSignature"} {
				out, err := exec.Command("defaults", "read", src.path+"/Contents/Info", key).Output()
				if id := strings.TrimSpace(string(out)); err == nil && id != "" && id != "????" {
					add(term{scope: src.scope, kind: "b", code: id, name: src.scope})
				}
			}
		}
		var srcTerms []term
		walk(root, "", func(kind, event string, n node) {
			code := n.Code
			if kind == "a" || kind == "E" {
				code = event + "/" + n.Code
			}
			srcTerms = append(srcTerms, term{src.scope, kind, code, n.Name, n.Plural, n.Hidden == "yes"})
		})
		// Within one dictionary a code has a single name: its last visible
		// definition, whatever its kind (Mail's dact is "smtp server" even
		// where a property is meant).
		lastName := map[string]string{}
		for _, t := range srcTerms {
			if (t.kind == "c" || t.kind == "p") && !t.hidden {
				lastName[t.code] = t.name
			}
		}
		for _, t := range srcTerms {
			name, visible := lastName[t.code]
			if t.kind == "c" || t.kind == "p" {
				if t.hidden && visible {
					continue // a deprecated alias of a visible term
				}
				if visible && src.scope != "" {
					t.name = name
				}
			}
			add(t)
		}
	}

	// AppleScript's own built-in terminology, from its aeut resource.
	source = len(sources)
	if data, err := readResource(appleScriptRsrc, "aeut", 0); err != nil {
		log.Printf("skip AppleScript aeut: %v", err)
	} else if aeut, err := parseAete(data, "AppleScript"); err != nil {
		log.Printf("skip AppleScript aeut: %v", err)
	} else {
		for _, t := range aeut {
			add(t)
		}
	}

	// Web services (`application "http://…"`): AppleScript builds their
	// terminology in, so it has no file to read.
	source++
	for _, t := range []term{
		{kind: "v", code: "rpc SOAP", name: "call soap"},
		{kind: "v", code: "rpc RPC2", name: "call xmlrpc"},
		{kind: "p", code: "meth", name: "method name"},
		{kind: "p", code: "mspu", name: "method namespace uri"},
		{kind: "p", code: "parm", name: "parameters"},
		{kind: "p", code: "sact", name: "SOAPAction"},
	} {
		t.scope = webServiceScope
		add(t)
	}

	// Applications that ship with macOS. Script Editor shows an application
	// specifier by its name when the application is installed, and by the
	// alias file name ("Foo.app") when it is not.
	for _, dir := range []string{"/System/Applications", "/System/Applications/Utilities",
		"/System/Library/CoreServices", "/System/Library/CoreServices/Applications", "/Applications"} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".app")
			if !ok {
				continue
			}
			if dir == "/Applications" && name != "Safari" {
				continue // user-installed; only Safari there is part of macOS
			}
			add(term{scope: "", kind: "n", code: name, name: name})
		}
	}

	var buf bytes.Buffer
	buf.WriteString("# Generated by tools/genterms from macOS sdef dictionaries. DO NOT EDIT.\n")
	for _, t := range terms {
		fmt.Fprintf(&buf, "%s\t%s\t%s\t%s", t.scope, t.kind, t.code, t.name)
		if t.plural != "" {
			fmt.Fprintf(&buf, "\t%s", t.plural)
		}
		buf.WriteByte('\n')
	}
	if err := os.WriteFile("internal/scpt/terms.tsv", buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	scopes := map[string]int{}
	for _, t := range terms {
		scopes[t.scope]++
	}
	var names []string
	for s := range scopes {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, s := range names {
		fmt.Printf("%-22q %d terms\n", s, scopes[s])
	}
}

// withSystemApps appends every other scriptable application that ships with
// macOS to the hand-ordered source list.
func withSystemApps(srcs []struct{ scope, path string }) []struct{ scope, path string } {
	have := map[string]bool{}
	for _, s := range srcs {
		have[s.scope] = true
	}
	candidates := []string{"/System/Library/PrivateFrameworks/SpeechObjects.framework/Versions/A/SpeechRecognitionServer.app"}
	for _, dir := range []string{"/System/Applications", "/System/Applications/Utilities",
		"/System/Library/CoreServices", "/System/Library/CoreServices/Applications"} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".app") {
				candidates = append(candidates, dir+"/"+e.Name())
			}
		}
	}
	sort.Strings(candidates[1:])
	for _, path := range candidates {
		name := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".app")
		if have[name] {
			continue
		}
		if out, err := exec.Command("sdef", path).Output(); err != nil || len(out) == 0 {
			continue
		}
		have[name] = true
		srcs = append(srcs, struct{ scope, path string }{name, path})
	}
	return srcs
}

func readSdef(path string) ([]byte, error) {
	var data []byte
	var err error
	if strings.HasSuffix(path, ".sdef") {
		data, err = os.ReadFile(path)
	} else {
		data, err = exec.Command("sdef", path).Output()
	}
	if err != nil {
		return nil, err
	}
	return expandIncludes(data, 0), nil
}

var includeRE = regexp.MustCompile(`<xi:include\s+href="file://(?:localhost)?([^"]+)"[^>]*/>`)
var dictBodyRE = regexp.MustCompile(`(?s)<dictionary[^>]*>(.*)</dictionary>`)

// expandIncludes inlines <xi:include href="file:///…sdef"/> elements with the
// body of the referenced dictionary.
func expandIncludes(data []byte, depth int) []byte {
	if depth > 4 {
		return data
	}
	return includeRE.ReplaceAllFunc(data, func(m []byte) []byte {
		path := string(includeRE.FindSubmatch(m)[1])
		inc, err := os.ReadFile(path)
		if err != nil {
			log.Printf("include %s: %v", path, err)
			return nil
		}
		body := dictBodyRE.FindSubmatch(expandIncludes(inc, depth+1))
		if body == nil {
			return nil
		}
		return body[1]
	})
}

// walk visits terminology-bearing elements. event is the enclosing command
// code, for parameters.
func walk(n node, event string, visit func(kind, event string, n node)) {
	kind := ""
	switch n.XMLName.Local {
	case "class", "class-extension", "record-type", "value-type":
		kind = "c"
	case "property", "contents":
		kind = "p"
	case "command", "event":
		kind = "v"
	}
	if kind != "" {
		// <synonym code="…"/> gives a legacy code for the same term.
		for _, c := range n.Children {
			if c.XMLName.Local == "synonym" && c.Code != "" {
				syn := n
				syn.Code = c.Code
				syn.Plural = ""
				visit("s"+kind, "", syn) // legacy synonym: a last-resort name
			}
		}
	}
	if n.Hidden == "yes" {
		// Everything inside a hidden class is hidden too.
		for i := range n.Children {
			n.Children[i].Hidden = "yes"
		}
	}
	switch n.XMLName.Local {
	case "class", "class-extension", "record-type", "value-type":
		visit("c", "", n)
	case "property", "contents":
		visit("p", "", n)
	case "enumeration":
		event = n.Code // reused to carry the enumeration code to enumerators
	case "enumerator":
		visit("e", "", n)
		if event != "" {
			visit("E", event, n)
		}
	case "command", "event":
		visit("v", "", n)
		event = n.Code
	case "parameter":
		if event != "" {
			visit("a", event, n)
		}
	}
	for _, c := range n.Children {
		walk(c, event, visit)
	}
}
