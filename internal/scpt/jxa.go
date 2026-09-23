package scpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// JXAMagic starts a compiled JavaScript for Automation script. The body is a
// binary property list {"script": source} followed by a "jscr" trailer.
var JXAMagic = []byte("JsOsaDAS1.001.00")

// parseJXA extracts the JavaScript source from a compiled JXA script.
func parseJXA(data []byte) (string, error) {
	body := data[len(JXAMagic):]
	if n := trailerLen(data); n > 0 && n <= len(body) {
		body = body[:len(body)-n]
	}
	obj, err := readBplist(body)
	if err != nil {
		return "", err
	}
	dict, ok := obj.(map[string]any)
	if !ok {
		return "", errors.New("JXA: plist root is not a dictionary")
	}
	src, ok := dict["script"].(string)
	if !ok {
		return "", errors.New("JXA: no script source in plist")
	}
	// Script Editor shows classic Mac line endings as newlines.
	return strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n"), nil
}

// isPlainText reports whether data looks like uncompiled source text (some
// ".scpt" files are really text files).
func isPlainText(data []byte) bool {
	return len(data) > 0 && utf8.Valid(data) && !bytes.ContainsRune(data, 0)
}

// readBplist decodes the subset of binary property lists used by JXA scripts:
// dictionaries, arrays, strings, integers and booleans.
func readBplist(b []byte) (any, error) {
	if len(b) < 40 || !bytes.HasPrefix(b, []byte("bplist00")) {
		return nil, errors.New("bplist: bad header")
	}
	t := b[len(b)-32:]
	offSize, refSize := int(t[6]), int(t[7])
	numObjs := int(binary.BigEndian.Uint64(t[8:]))
	top := int(binary.BigEndian.Uint64(t[16:]))
	tableOff := int(binary.BigEndian.Uint64(t[24:]))
	// Bounds are checked before any arithmetic can overflow.
	if offSize < 1 || offSize > 8 || refSize < 1 || refSize > 8 ||
		numObjs < 0 || numObjs > len(b) || tableOff < 0 || tableOff > len(b) ||
		top < 0 || top >= numObjs || tableOff+numObjs*offSize > len(b) {
		return nil, errors.New("bplist: bad trailer")
	}
	uint := func(p []byte) int {
		v := 0
		for _, c := range p {
			v = v<<8 | int(c)
		}
		if v < 0 || v > len(b) {
			return len(b) // out of range; callers reject it
		}
		return v
	}
	offset := func(i int) int { return uint(b[tableOff+i*offSize : tableOff+(i+1)*offSize]) }

	var decode func(i, depth int) (any, error)
	decode = func(i, depth int) (any, error) {
		if i < 0 || i >= numObjs || depth > 32 {
			return nil, errors.New("bplist: bad object reference")
		}
		p := offset(i)
		if p < 0 || p >= len(b) {
			return nil, errors.New("bplist: bad offset")
		}
		marker := b[p]
		kind, low := marker>>4, int(marker&0xf)
		p++
		// Collections and strings carry a length, extended by an int object when low == 0xf.
		length := func() (int, error) {
			if low != 0xf {
				return low, nil
			}
			if p >= len(b) || b[p]>>4 != 1 {
				return 0, errors.New("bplist: bad length")
			}
			n := 1 << (b[p] & 0xf)
			if p+1+n > len(b) {
				return 0, errors.New("bplist: truncated length")
			}
			v := uint(b[p+1 : p+1+n])
			p += 1 + n
			return v, nil
		}
		switch kind {
		case 0x0:
			switch marker {
			case 0x08:
				return false, nil
			case 0x09:
				return true, nil
			}
			return nil, nil
		case 0x1:
			n := 1 << low
			if p+n > len(b) {
				return nil, errors.New("bplist: truncated int")
			}
			return uint(b[p : p+n]), nil
		case 0x5, 0x6:
			n, err := length()
			if err != nil {
				return nil, err
			}
			if kind == 0x5 {
				if p+n > len(b) {
					return nil, errors.New("bplist: truncated string")
				}
				return string(b[p : p+n]), nil
			}
			if p+2*n > len(b) {
				return nil, errors.New("bplist: truncated string")
			}
			u := make([]uint16, n)
			for k := range u {
				u[k] = binary.BigEndian.Uint16(b[p+2*k:])
			}
			return string(utf16.Decode(u)), nil
		case 0xa, 0xd:
			n, err := length()
			if err != nil {
				return nil, err
			}
			refs := func(k int) int { return uint(b[p+k*refSize : p+(k+1)*refSize]) }
			if kind == 0xa {
				if p+n*refSize > len(b) {
					return nil, errors.New("bplist: truncated array")
				}
				out := make([]any, n)
				for k := range out {
					v, err := decode(refs(k), depth+1)
					if err != nil {
						return nil, err
					}
					out[k] = v
				}
				return out, nil
			}
			if p+2*n*refSize > len(b) {
				return nil, errors.New("bplist: truncated dict")
			}
			out := make(map[string]any, n)
			for k := 0; k < n; k++ {
				key, err := decode(refs(k), depth+1)
				if err != nil {
					return nil, err
				}
				val, err := decode(refs(n+k), depth+1)
				if err != nil {
					return nil, err
				}
				if ks, ok := key.(string); ok {
					out[ks] = val
				}
			}
			return out, nil
		}
		return nil, errors.New("bplist: unsupported object type")
	}
	return decode(top, 0)
}
