package scpt

import (
	"bytes"
	"encoding/binary"
)

// Before Mac OS X, a compiled script kept its data in the file's resource
// fork, as a 'scpt' resource, and left the data fork empty. Outside a Mac
// file system the resource fork only survives inside a wrapper: an
// AppleDouble or AppleSingle file, MacBinary, or BinHex. unwrapScript finds
// the compiled script in any of these, or in a bare resource fork.
func unwrapScript(data []byte) ([]byte, bool) {
	if s, ok := appleSingleScript(data); ok {
		return s, true
	}
	if s, ok := binHexScript(data); ok {
		return s, true
	}
	if s, ok := macBinaryScript(data); ok {
		return s, true
	}
	return resourceScript(data)
}

// forkScript returns the compiled script in a file's data fork or, failing
// that, its resource fork.
func forkScript(dataFork, rsrcFork []byte) ([]byte, bool) {
	if bytes.HasPrefix(dataFork, Magic) {
		return dataFork, true
	}
	return resourceScript(rsrcFork)
}

// resourceScript returns the 'scpt' resource of a resource fork, preferring
// ID 128, which Script Editor used.
func resourceScript(fork []byte) ([]byte, bool) {
	res := resources(fork, "scpt")
	if s, ok := res[128]; ok && bytes.HasPrefix(s, Magic) {
		return s, true
	}
	for _, s := range res {
		if bytes.HasPrefix(s, Magic) {
			return s, true
		}
	}
	return nil, false
}

// resources returns the resources of one type in a resource fork, by ID.
func resources(fork []byte, typ string) map[int16][]byte {
	be := binary.BigEndian
	if len(fork) < 16 {
		return nil
	}
	dataOff, mapOff := int(be.Uint32(fork)), int(be.Uint32(fork[4:]))
	if mapOff < 0 || mapOff+28 > len(fork) || dataOff < 0 || dataOff > len(fork) {
		return nil
	}
	m := fork[mapOff:]
	typeListOff := int(be.Uint16(m[24:]))
	if typeListOff+2 > len(m) {
		return nil
	}
	typeList := m[typeListOff:]
	nTypes := int(int16(be.Uint16(typeList))) + 1
	out := map[int16][]byte{}
	for i := 0; i < nTypes && 2+i*8+8 <= len(typeList); i++ {
		e := typeList[2+i*8:]
		if string(e[:4]) != typ {
			continue
		}
		count := int(int16(be.Uint16(e[4:]))) + 1
		refsOff := int(be.Uint16(e[6:]))
		for j := 0; j < count; j++ {
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
			out[int16(be.Uint16(ref))] = fork[off+4 : off+4+n]
		}
	}
	return out
}

// appleSingleScript reads an AppleSingle file, or the AppleDouble header
// file (._name) that carries a file's resource fork on other file systems.
func appleSingleScript(data []byte) ([]byte, bool) {
	be := binary.BigEndian
	if len(data) < 26 {
		return nil, false
	}
	if magic := be.Uint32(data); magic != 0x00051600 && magic != 0x00051607 {
		return nil, false
	}
	var dataFork, rsrcFork []byte
	n := int(be.Uint16(data[24:]))
	for i := 0; i < n && 26+i*12+12 <= len(data); i++ {
		e := data[26+i*12:]
		id, off, length := be.Uint32(e), int(be.Uint32(e[4:])), int(be.Uint32(e[8:]))
		if off < 0 || length < 0 || off+length > len(data) {
			continue
		}
		switch id {
		case 1:
			dataFork = data[off : off+length]
		case 2:
			rsrcFork = data[off : off+length]
		}
	}
	return forkScript(dataFork, rsrcFork)
}

// macBinaryScript reads a MacBinary file: a 128-byte header, then the data
// fork and the resource fork, each padded to a multiple of 128 bytes.
func macBinaryScript(data []byte) ([]byte, bool) {
	be := binary.BigEndian
	if len(data) < 128 || data[0] != 0 || data[74] != 0 || data[82] != 0 || data[1] == 0 || data[1] > 63 {
		return nil, false
	}
	dataLen, rsrcLen := int(be.Uint32(data[83:])), int(be.Uint32(data[87:]))
	rsrcOff := 128 + (dataLen+127)/128*128
	if dataLen < 0 || rsrcLen < 0 || 128+dataLen > len(data) || rsrcOff+rsrcLen > len(data) {
		return nil, false
	}
	return forkScript(data[128:128+dataLen], data[rsrcOff:rsrcOff+rsrcLen])
}

// binHexScript reads a BinHex 4.0 file: text that encodes a header, the data
// fork and the resource fork, six bits per character with run-length
// compression.
func binHexScript(data []byte) ([]byte, bool) {
	start := bytes.Index(data, []byte("(This file must be converted with BinHex"))
	if start < 0 {
		return nil, false
	}
	colon := bytes.IndexByte(data[start:], ':')
	if colon < 0 {
		return nil, false
	}
	raw, ok := decodeBinHex(data[start+colon+1:])
	if !ok || len(raw) < 1 {
		return nil, false
	}
	be := binary.BigEndian
	p := 1 + int(raw[0]) + 1 + 4 + 4 + 2 // name, version, type, creator, flags
	if p+10 > len(raw) {
		return nil, false
	}
	dataLen, rsrcLen := int(be.Uint32(raw[p:])), int(be.Uint32(raw[p+4:]))
	p += 8 + 2 // lengths, header CRC
	if dataLen < 0 || rsrcLen < 0 || p+dataLen+2+rsrcLen > len(raw) {
		return nil, false
	}
	dataFork := raw[p : p+dataLen]
	p += dataLen + 2
	return forkScript(dataFork, raw[p:p+rsrcLen])
}

const binHexAlphabet = "!\"#$%&'()*+,-012345689@ABCDEFGHIJKLMNPQRSTUVXYZ[`abcdefhijklmpqr"

// decodeBinHex decodes BinHex text up to its closing colon.
func decodeBinHex(text []byte) ([]byte, bool) {
	var value [256]int8
	for i := range value {
		value[i] = -1
	}
	for i := 0; i < len(binHexAlphabet); i++ {
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
