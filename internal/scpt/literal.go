package scpt

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// formatReal prints a real the way AppleScript does: always with a decimal
// point, switching to exponent notation for very large or small magnitudes.
func formatReal(v float64) string {
	if a := math.Abs(v); a != 0 && (a >= 1e4 || a <= 1e-3) {
		m := strings.ToUpper(strconv.FormatFloat(v, 'E', -1, 64))
		mant, exp, _ := strings.Cut(m, "E")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		if exp[0] == '+' {
			exp = "+" + strings.TrimLeft(exp[1:], "0")
		} else {
			exp = "-" + strings.TrimLeft(exp[1:], "0")
		}
		return mant + "E" + exp
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// displayAppName renders an application alias's file name the way Script
// Editor does on a stock Mac: by name for applications that ship with macOS,
// and as the bare file name ("Foo.app") for ones it cannot find.
func displayAppName(file string) string {
	name := strings.TrimSuffix(file, ".app")
	if renamed, ok := scopeAliases[name]; ok {
		return renamed // macOS resolves the old name to the renamed app
	}
	if systemApps[strings.ToLower(name)] || dicts[name] != nil {
		return name
	}
	return file
}

// macEpoch is the zero point of classic Mac OS LongDateTime values.
var macEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

// descLiteral renders an AEDesc literal: dates as `date "…"`, anything else
// as raw «data TYPE0123…».
func descLiteral(d descRec) string {
	if d.typ == "ldt " && len(d.data) == 8 {
		secs := int64(binary.BigEndian.Uint64(d.data))
		t := time.Unix(macEpoch.Unix()+secs, 0).UTC() // no Duration overflow for far dates
		return "date " + quoteAS(t.Format("Monday, January 2, 2006 at 3:04:05\u202fPM"))
	}
	if d.typ == "alis" {
		if path, ok := aliasPath(d.data); ok {
			return "alias " + quoteAS(path)
		}
	}
	return fmt.Sprintf("«data %s%X»", d.typ, d.data)
}

// aliasPath renders an alias record as the HFS path Script Editor shows: its
// volume-relative POSIX path (extended tag 18) on the startup volume, or
// failing that the recorded full HFS path (tag 2). Folders end with ':'.
func aliasPath(rec []byte) (string, bool) {
	// userType(4) size(2) version(2) kind(2) … then tagged extended data at 150.
	if len(rec) < 150 {
		return "", false
	}
	a := rec
	folder := binary.BigEndian.Uint16(a[8:]) == 1
	hfs, _ := aliasTag(a, aliasTagHFSPath)
	posix, _ := aliasTag(a, aliasTagPOSIXPath)
	path := macRoman(hfs)
	if len(posix) > 0 {
		path = "Macintosh HD:" + strings.ReplaceAll(strings.TrimPrefix(string(posix), "/"), "/", ":")
	}
	if path == "" {
		return "", false
	}
	if folder && !strings.HasSuffix(path, ":") {
		path += ":"
	}
	return path, true
}

// Tags of an alias record's extended data.
const (
	aliasTagHFSPath    = 2
	aliasTagPOSIXPath  = 18 // relative to the volume's mount point
	aliasTagMountPoint = 19
)

// aliasTag returns one tagged item of an alias record's extended data, which
// starts at offset 150.
func aliasTag(rec []byte, tag int16) ([]byte, bool) {
	for p := 150; p+4 <= len(rec); {
		t := int16(binary.BigEndian.Uint16(rec[p:]))
		n := int(binary.BigEndian.Uint16(rec[p+2:]))
		if t == -1 || p+4+n > len(rec) {
			break
		}
		if t == tag {
			return rec[p+4 : p+4+n], true
		}
		p += 4 + n + n%2
	}
	return nil, false
}

// quoteAS renders s as an AppleScript string literal. Only backslash and
// double quote are escaped; tabs and newlines stay literal, as Script Editor
// prints them.
func quoteAS(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, contMarkByte, "") // cannot occur in real text
	return `"` + s + `"`
}
