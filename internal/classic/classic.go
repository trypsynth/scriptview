// Package classic reads the files of classic Mac OS.
//
// A classic Mac file has two forks: the data fork, and the resource fork,
// which holds typed, numbered resources. Its Finder information adds a
// four-character file type and creator. Other file systems have no resource
// fork, so it survives only inside a wrapper: an AppleSingle or AppleDouble
// file, MacBinary, or BinHex. This package unwraps these and reads resource
// forks.
package classic

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"

	"github.com/trypsynth/scriptview/internal/mactext"
)

// File is a classic Mac file.
type File struct {
	Name          string // empty when the wrapper doesn't record it
	Type, Creator string // four-character codes, empty when unknown
	Data, Rsrc    []byte // the data fork and the resource fork
}

// Unwrap reads an AppleSingle, AppleDouble, MacBinary, or BinHex file. It
// reports false if data is none of these.
func Unwrap(data []byte) (File, bool) {
	if f, ok := appleSingle(data); ok {
		return f, true
	}
	if f, ok := binHex(data); ok {
		return f, true
	}
	return macBinary(data)
}

// ReadFile reads the file at path with both forks. The resource fork comes
// from the file itself on a Mac, or else from the AppleDouble file that
// carries it on other file systems: ._name next to the file, or in the
// __MACOSX folder that a zip file made on a Mac unpacks to.
func ReadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	f := File{Name: filepath.Base(path), Data: data}
	if rsrc, err := os.ReadFile(path + "/..namedfork/rsrc"); err == nil && len(rsrc) > 0 {
		f.Rsrc = rsrc
		return f, nil
	}
	for _, p := range appleDoublePaths(path) {
		if d, err := os.ReadFile(p); err == nil {
			if double, ok := appleSingle(d); ok {
				f.Rsrc, f.Type, f.Creator = double.Rsrc, double.Type, double.Creator
				return f, nil
			}
		}
	}
	return f, nil
}

func appleDoublePaths(path string) []string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	name := "._" + filepath.Base(abs)
	dir := filepath.Dir(abs)
	out := []string{filepath.Join(dir, name)}
	for rel, root := "", dir; ; {
		out = append(out, filepath.Join(root, "__MACOSX", rel, name))
		parent := filepath.Dir(root)
		if parent == root {
			return out
		}
		rel = filepath.Join(filepath.Base(root), rel)
		root = parent
	}
}

// Resource is one resource of a resource fork.
type Resource struct {
	Type string // four characters, such as "STR#"
	ID   int16
	Name string // empty when the resource has none
	Data []byte
}

// ResourceList returns every resource in a resource fork, in the order of
// the fork's map. It returns what it can read of a damaged fork.
func ResourceList(fork []byte) []Resource {
	be := binary.BigEndian
	if len(fork) < 16 {
		return nil
	}
	dataOff, mapOff := int(be.Uint32(fork)), int(be.Uint32(fork[4:]))
	if mapOff < 0 || mapOff+28 > len(fork) || dataOff < 0 || dataOff > len(fork) {
		return nil
	}
	m := fork[mapOff:]
	typeListOff, nameListOff := int(be.Uint16(m[24:])), int(be.Uint16(m[26:]))
	if typeListOff+2 > len(m) {
		return nil
	}
	typeList := m[typeListOff:]
	nTypes := int(int16(be.Uint16(typeList))) + 1
	var out []Resource
	for i := 0; i < nTypes && 2+i*8+8 <= len(typeList); i++ {
		e := typeList[2+i*8:]
		typ := string(e[:4])
		count := int(int16(be.Uint16(e[4:]))) + 1
		refsOff := int(be.Uint16(e[6:]))
		for j := range count {
			r := refsOff + j*12
			if r+12 > len(typeList) {
				break
			}
			ref := typeList[r:]
			off := dataOff + int(be.Uint32(ref[4:])&0xffffff)
			if off+4 > len(fork) {
				continue
			}
			n := int(be.Uint32(fork[off:]))
			if n < 0 || off+4+n > len(fork) {
				continue
			}
			res := Resource{Type: typ, ID: int16(be.Uint16(ref)), Data: fork[off+4 : off+4+n]}
			// The name is a Pascal string in the name list, or -1 for none.
			if no := int(int16(be.Uint16(ref[2:]))); no >= 0 {
				if p := nameListOff + no; p < len(m) && p+1+int(m[p]) <= len(m) {
					res.Name = mactext.MacRoman(m[p+1 : p+1+int(m[p])])
				}
			}
			out = append(out, res)
		}
	}
	return out
}

// Resources returns the resources of one type in a resource fork, by ID.
func Resources(fork []byte, typ string) map[int16][]byte {
	out := map[int16][]byte{}
	for _, r := range ResourceList(fork) {
		if r.Type == typ {
			out[r.ID] = r.Data
		}
	}
	return out
}

// appleSingle reads an AppleSingle file, or an AppleDouble header file,
// which has the same layout without the data fork.
func appleSingle(data []byte) (File, bool) {
	be := binary.BigEndian
	if len(data) < 26 {
		return File{}, false
	}
	if magic := be.Uint32(data); magic != 0x00051600 && magic != 0x00051607 {
		return File{}, false
	}
	var f File
	n := int(be.Uint16(data[24:]))
	for i := 0; i < n && 26+i*12+12 <= len(data); i++ {
		e := data[26+i*12:]
		id, off, length := be.Uint32(e), int(be.Uint32(e[4:])), int(be.Uint32(e[8:]))
		if off < 0 || length < 0 || off+length > len(data) {
			continue
		}
		entry := data[off : off+length]
		switch id {
		case 1:
			f.Data = entry
		case 2:
			f.Rsrc = entry
		case 3:
			f.Name = string(entry)
		case 9: // Finder information
			if len(entry) >= 8 {
				f.Type, f.Creator = code(entry[:4]), code(entry[4:8])
			}
		}
	}
	return f, true
}

// macBinary reads a MacBinary file: a 128-byte header, then the data fork
// and the resource fork, each padded to a multiple of 128 bytes.
func macBinary(data []byte) (File, bool) {
	be := binary.BigEndian
	if len(data) < 128 || data[0] != 0 || data[74] != 0 || data[82] != 0 || data[1] == 0 || data[1] > 63 {
		return File{}, false
	}
	dataLen, rsrcLen := int(be.Uint32(data[83:])), int(be.Uint32(data[87:]))
	rsrcOff := 128 + (dataLen+127)/128*128
	if dataLen < 0 || rsrcLen < 0 || 128+dataLen > len(data) || rsrcOff+rsrcLen > len(data) {
		return File{}, false
	}
	return File{
		Name: string(data[2 : 2+int(data[1])]),
		Type: code(data[65:69]), Creator: code(data[69:73]),
		Data: data[128 : 128+dataLen], Rsrc: data[rsrcOff : rsrcOff+rsrcLen],
	}, true
}

// binHex reads a BinHex 4.0 file: text that encodes a header, the data fork
// and the resource fork, six bits per character with run-length
// compression.
func binHex(data []byte) (File, bool) {
	start := bytes.Index(data, []byte("(This file must be converted with BinHex"))
	if start < 0 {
		return File{}, false
	}
	colon := bytes.IndexByte(data[start:], ':')
	if colon < 0 {
		return File{}, false
	}
	raw, ok := decodeBinHex(data[start+colon+1:])
	if !ok || len(raw) < 1 {
		return File{}, false
	}
	be := binary.BigEndian
	nameLen := int(raw[0])
	p := 1 + nameLen + 1 // name, version
	if p+4+4+2+10 > len(raw) {
		return File{}, false
	}
	f := File{Name: string(raw[1 : 1+nameLen]), Type: code(raw[p : p+4]), Creator: code(raw[p+4 : p+8])}
	p += 4 + 4 + 2 // type, creator, flags
	dataLen, rsrcLen := int(be.Uint32(raw[p:])), int(be.Uint32(raw[p+4:]))
	p += 8 + 2 // lengths, header CRC
	if dataLen < 0 || rsrcLen < 0 || p+dataLen+2+rsrcLen > len(raw) {
		return File{}, false
	}
	f.Data = raw[p : p+dataLen]
	p += dataLen + 2
	f.Rsrc = raw[p : p+rsrcLen]
	return f, true
}

const binHexAlphabet = "!\"#$%&'()*+,-012345689@ABCDEFGHIJKLMNPQRSTUVXYZ[`abcdefhijklmpqr"

// decodeBinHex decodes BinHex text up to its closing colon.
func decodeBinHex(text []byte) ([]byte, bool) {
	var value [256]int8
	for i := range value {
		value[i] = -1
	}
	for i := range len(binHexAlphabet) {
		value[binHexAlphabet[i]] = int8(i)
	}
	var packed []byte
	var acc, bits uint
	for _, c := range text {
		if c == ':' {
			break
		}
		v := value[c]
		if v < 0 {
			continue // line breaks and other whitespace
		}
		acc = acc<<6 | uint(v)
		bits += 6
		if bits >= 8 {
			bits -= 8
			packed = append(packed, byte(acc>>bits))
			acc &= 1<<bits - 1
		}
	}
	// Run-length decoding: 0x90 n repeats the previous byte n-1 more
	// times; 0x90 0 is a literal 0x90.
	out := make([]byte, 0, len(packed))
	for i := 0; i < len(packed); i++ {
		c := packed[i]
		if c != 0x90 || i+1 >= len(packed) {
			out = append(out, c)
			continue
		}
		i++
		n := int(packed[i])
		if n == 0 {
			out = append(out, 0x90)
			continue
		}
		if len(out) == 0 {
			return nil, false
		}
		prev := out[len(out)-1]
		for k := 1; k < n; k++ {
			out = append(out, prev)
		}
	}
	return out, true
}

// code returns a four-character code as text, or "" when it's unset.
func code(b []byte) string {
	if bytes.Equal(b, []byte{0, 0, 0, 0}) {
		return ""
	}
	return string(b)
}
