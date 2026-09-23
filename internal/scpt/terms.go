package scpt

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// coreDict holds AppleScript's own value classes, which take precedence over
// any application's spelling of the same code.
var coreDict = func() *dict {
	d := newDict()
	for _, code := range []string{"TEXT", "utxt", "list", "reco", "long", "doub", "nmbr", "bool", "ldt ", "alis", "enum", "scpt", "hand"} {
		d.classes[code] = fourCCNames[code]
	}
	return d
}()

// builtinDict holds the curated AppleScript language terminology below.
var builtinDict = &dict{
	classes: fourCCNames, props: fourCCNames, enums: fourCCNames,
	events: eventNames, params: map[string]string{}, plurals: map[string]string{},
	qenums: map[string]string{},
}

// fourCCNames maps four-char type/class/property/constant codes to the
// terminology AppleScript prints for them. Codes missing from this table are
// rendered with raw «class xxxx» syntax, which AppleScript itself accepts.
var fourCCNames = map[string]string{
	// Constants
	"true": "true",
	"fals": "false",
	"msng": "missing value",
	"null": "null",
	"yes ": "yes",
	"no  ": "no",
	"ask ": "ask",
	"pi  ": "pi",
	"tab ": "tab",
	"lnfd": "linefeed",
	"ret ": "return",
	"spac": "space",
	"quot": "quote",
	"me  ": "me",
	"rslt": "result",
	"ascr": "AppleScript",
	"cura": "current application",
	"firs": "first",
	"last": "last",
	"midd": "middle",
	"any ": "some",

	// Value classes
	"cobj": "item",
	"citm": "text item",
	"cha ": "character",
	"cwor": "word",
	"cpar": "paragraph",
	"ctxt": "text",
	"TEXT": "string",
	"utxt": "Unicode text",
	"list": "list",
	"reco": "record",
	"long": "integer",
	"doub": "real",
	"nmbr": "number",
	"bool": "boolean",
	"ldt ": "date",
	"alis": "alias",
	"file": "file",
	"enum": "constant",
	"scpt": "script",
	"hand": "handler",

	// Properties
	"pnam": "name",
	"pcls": "class",
	"ID  ": "id",
	"pidx": "index",
	"pcnt": "contents",
	"pALL": "properties",
	"prun": "running",
	"pare": "parent",
	"kMsg": "key",
	"shdt": "short date string",
	"scnd": "seconds",
	"from": "from",
	"osax": "scripting addition",
	"frmk": "framework",
	"rmte": "application responses",
	"obj ": "reference",
	"prop": "property",
	"cRGB": "RGB color",
	"jan ": "January",
	"feb ": "February",
	"mar ": "March",
	"apr ": "April",
	"may ": "May",
	"jun ": "June",
	"jul ": "July",
	"aug ": "August",
	"sep ": "September",
	"oct ": "October",
	"nov ": "November",
	"dec ": "December",
	"mon ": "Monday",
	"tue ": "Tuesday",
	"wed ": "Wednesday",
	"thu ": "Thursday",
	"fri ": "Friday",
	"sat ": "Saturday",
	"sun ": "Sunday",
	"vers": "version",
	"leng": "length",
	"rest": "rest",
	"rvse": "reverse",
	"txdl": "text item delimiters",
	"strq": "quoted form",
	"psxp": "POSIX path",
	"year": "year",
	"mnth": "month",
	"day ": "day",
	"wkdy": "weekday",
	"hour": "hours",
	"min ": "minutes",
	"days": "days",
	"week": "weeks",
	"time": "time",
	"tstr": "time string",
	"dstr": "date string",

	// Considering/ignoring attributes
	"case": "case",
	"whit": "white space",
	"hyph": "hyphens",
	"punc": "punctuation",
	"diac": "diacriticals",
	"expa": "expansion",
	"nume": "numeric strings",

	// Standard Additions results and folders
	"bhit": "button returned",
	"ttxt": "text returned",
	"gavu": "gave up",
	"desk": "desktop",
	"docs": "documents folder",
	"cusr": "home folder",
	"temp": "temporary items",
	"dlib": "library folder",
	"asup": "application support",
	"apps": "applications folder",
	"down": "downloads folder",
}

// eventNames maps suite+event codes (8 chars) to command names.
var eventNames = map[string]string{
	// Standard Additions
	"sysodlog": "display dialog",
	"sysodisA": "display alert",
	"sysonotf": "display notification",
	"sysoexec": "do shell script",
	"sysottos": "say",
	"sysobeep": "beep",
	"sysodela": "delay",
	"sysorand": "random number",
	"sysoGMT ": "time to GMT",
	"sysooffs": "offset",
	"sysontoc": "ASCII character",
	"sysocton": "ASCII number",
	"sysostdf": "choose file",
	"sysostfl": "choose folder",
	"sysonfo4": "info for",
	"sysoload": "load script",
	"sysostor": "store script",
	"sysodsct": "run script",
	"gtqpchlt": "choose from list",
	"earsffdr": "path to",
	"JonsgClp": "the clipboard",
	"JonspClp": "set the clipboard to",
	"rdwrread": "read",
	"rdwrwrit": "write",
	"rdwropen": "open for access",
	"rdwrclos": "close access",
	"rdwrgeof": "get eof",
	"GURLGURL": "open location",
	"misccurd": "current date",
	"ascrcmnt": "log",
	"ascrerr ": "error",
	"ascrnoop": "launch",
	"miscidle": "idle",

	// Core / required suites
	"aevtoapp": "run",
	"aevtodoc": "open",
	"aevtpdoc": "print",
	"aevtquit": "quit",
	"aevtrapp": "reopen",
	"miscactv": "activate",
	"miscslct": "select",
	"corecnte": "count",
	"coreclos": "close",
	"coredelo": "delete",
	"coreclon": "duplicate",
	"coredoex": "exists",
	"corecrel": "make",
	"coremove": "move",
	"coresave": "save",
}

// paramNames maps labeled-parameter keywords (as used in command calls) to
// their terminology. These codes overlap with property codes, so they are
// looked up only in parameter position.
var paramNames = map[string]string{
	// display dialog / alert / notification
	"btns": "buttons",
	"dflt": "default button",
	"cbtn": "cancel button",
	"appr": "with title",
	"disp": "with icon",
	"givu": "giving up after",
	"dtxt": "default answer",
	"htxt": "hidden answer",
	"mesS": "message",
	"as A": "as",
	"subt": "subtitle",
	"nsou": "sound name",
	// say
	"wfsp": "waiting until completion",
	"VOIC": "using",
	"stng": "saving to",
	// do shell script
	"badm": "administrator privileges",
	"RApw": "password",
	"alen": "altering line endings",
	// error
	"errn": "number",
	"ptlr": "partial result",
	"erob": "from",
	"errt": "to",
	// path to
	"from": "from",
	"rtyp": "as",
	"fldc": "folder creation",
	// choose from list / file
	"prmp": "with prompt",
	"inSL": "default items",
	"okbt": "OK button name",
	"mlsl": "multiple selections allowed",
	"empL": "empty selection allowed",
	"ofty": "of type",
	// read / write
	"for ": "for",
	"befo": "before",
	"usin": "until",
	"refn": "to",
	"wrat": "starting at",
	"perm": "with write permission",
	// core suite
	"insh": "at",
	"prdt": "with properties",
	"data": "with data",
	"each": "each",
}

// osTypeKind is the kind byte that follows an OpcodeOSType record header.
const (
	osKindCode     = 0x0a // 4-byte class/property/constant code
	osKindConstant = 0x0b // type(4) + value(4), e.g. boov/true
	osKindEvent    = 0x2e // suite(4) + event(4) + result type(4) + flags…
)

// scanOSTypeNames scans raw FASD bytecode for OpcodeOSType records:
//
//	0x0a ref(2, negative) len(2) kind(1) data(len)
//
// It returns the raw four-char code behind each term ref and the 8-char
// suite+event code behind each command ref. Names are resolved at render
// time, when the enclosing tell target (and so the dictionary) is known.
func scanOSTypeNames(body []byte) (osCode map[int16]string, evCode map[int16]string) {
	osCode = make(map[int16]string)
	evCode = make(map[int16]string)

	for i := 0; i+6 <= len(body); i++ {
		if body[i] != OpcodeOSType {
			continue
		}
		ref := int16(uint16(body[i+1])<<8 | uint16(body[i+2]))
		n := int(uint16(body[i+3])<<8 | uint16(body[i+4]))
		kind := body[i+5]
		if ref >= 0 || i+6+n > len(body) {
			continue
		}
		data := body[i+6 : i+6+n]
		var m map[int16]string
		var code string
		switch {
		case kind == osKindCode && n == 4 && isFourCC(data):
			m, code = osCode, string(data)
		case kind == osKindConstant && n == 8 && isFourCC(data[4:]):
			m, code = osCode, string(data[4:])
		case kind == osKindEvent && n >= 8 && isFourCC(data[:8]):
			m, code = evCode, string(data[:8])
		default:
			continue
		}
		if _, seen := m[ref]; !seen {
			m[ref] = code
		}
	}
	return osCode, evCode
}

//go:embed terms.tsv
var termsTSV string

// dict is one scripting dictionary's terminology, keyed by code.
type dict struct {
	classes, props, enums, events, params, plurals map[string]string
	// qenums holds enumerators keyed by "enumeration/enumerator" code.
	qenums map[string]string
	// words caches the set of lowercase term names, for spotting user
	// identifiers that must be written |piped|.
	words map[string]bool
}

// hasWord reports whether name (lowercase) is a term in d.
func (d *dict) hasWord(name string) bool {
	if d == nil {
		return false
	}
	if d.words == nil {
		d.words = map[string]bool{}
		for _, m := range []map[string]string{d.classes, d.props, d.enums, d.events, d.plurals, d.qenums} {
			for _, v := range m {
				d.words[strings.ToLower(v)] = true
			}
		}
	}
	return d.words[name]
}

func newDict() *dict {
	return &dict{
		classes: map[string]string{}, props: map[string]string{}, enums: map[string]string{},
		events: map[string]string{}, params: map[string]string{}, plurals: map[string]string{},
		qenums: map[string]string{},
	}
}

// dicts maps scope ("" = always in scope, else application name) to its
// terminology, loaded from the generated terms.tsv.
// unpipedWords are AppleScript terms that Script Editor nonetheless leaves
// unpiped as identifiers: those only defined in the aeut's legacy suites.
var unpipedWords = map[string]bool{}

// additionsWords are legacy AppleScript terms that Script Editor leaves
// unpiped (see unpipedWords) except while Standard Additions is loaded.
var additionsWords = map[string]bool{
	"appletalk": true, "fixed": true, "ip": true, "menu": true, "null": true,
	"point": true, "port": true, "rotation": true,
}

// synonymTerms and synonymEvents hold legacy codes that dictionaries list as
// <synonym code="…"/>; they name a code only when nothing else does.
var synonymTerms, synonymEvents = map[string]string{}, map[string]string{}

// systemApps holds (lowercase) names of applications that ship with macOS.
var systemApps = map[string]bool{}

var dicts, bundleScopes = loadDicts(termsTSV)

// webServiceScope holds the terminology of SOAP and XML-RPC servers, which
// applications named by an http:// URL use.
const webServiceScope = "http://"

// scopeAliases maps former application names to the scope holding their
// terminology today.
var scopeAliases = map[string]string{
	"System Preferences": "System Settings",
	"Address Book":       "Contacts",
	"iCal":               "Calendar",
}

func loadDicts(tsv string) (map[string]*dict, map[string]string) {
	out := map[string]*dict{}
	bundles := map[string]string{}
	for _, line := range strings.Split(tsv, "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 4 {
			continue
		}
		d := out[f[0]]
		if d == nil {
			d = newDict()
			out[f[0]] = d
		}
		code, name := f[2], f[3]
		switch f[1] {
		case "c":
			d.classes[code] = name
			if len(f) > 4 {
				d.plurals[code] = f[4]
			}
		case "p":
			d.props[code] = name
		case "e":
			d.enums[code] = name
		case "v":
			d.events[code] = name
		case "a":
			d.params[code] = name
		case "E":
			d.qenums[code] = name
		case "b":
			bundles[strings.ToLower(code)] = f[0]
		case "n":
			systemApps[strings.ToLower(code)] = true
		case "x":
			unpipedWords[code] = true
		case "sc", "sp":
			synonymTerms[code] = name
		case "sv":
			synonymEvents[code] = name
		}
	}
	return out, bundles
}

// termKind selects which namespaces a lookup searches, in order.
type termKind int

const (
	kindValue    termKind = iota // property, then class, then constant
	kindClass                    // class, then property, then constant
	kindProperty                 // property, then class: never a constant
)

// lookup finds code in d, searching namespaces in the order kind implies.
func (d *dict) lookup(code string, kind termKind) string {
	if d == nil {
		return ""
	}
	order := []map[string]string{d.props, d.classes, d.enums}
	switch kind {
	case kindClass:
		order = []map[string]string{d.classes, d.props, d.enums}
	case kindProperty:
		order = order[:2]
	}
	for _, m := range order {
		if name, ok := m[code]; ok {
			return name
		}
	}
	return ""
}

// rawCode spells a code inside «class …» the way Script Editor does: its four
// bytes as Mac Roman, even when they are not printable.
func rawCode(code string) string {
	if hex, ok := strings.CutPrefix(code, "0x"); ok && len(hex) == 8 {
		if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return macRoman(binary.BigEndian.AppendUint32(nil, uint32(v)))
		}
	}
	return code
}

// fourCC renders a four-byte code as text: Mac Roman when printable, else
// the 0xXXXXXXXX form that sdef files use for binary codes.
func fourCC(b []byte) string {
	if isFourCC(b) {
		return macRoman(b)
	}
	return fmt.Sprintf("0x%08x", binary.BigEndian.Uint32(b))
}

// isFourCC reports whether b is made of printable Mac Roman / ASCII bytes.
func isFourCC(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}
