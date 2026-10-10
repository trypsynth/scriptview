// Command gensjis writes internal/scpt/sjis.bin, the Shift JIS double-byte
// table that scriptview uses for old Japanese scripts. It needs macOS's
// iconv, which knows CP932 (Shift JIS with the usual extensions).
//
// The table has one 16-bit big-endian UTF-16 code unit for each lead byte
// (0x81-0x9F, 0xE0-0xFC) and trail byte (0x40-0xFC except 0x7F), in that
// order. 0 means the pair doesn't decode.
package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"os/exec"
	"unicode/utf16"
	"unicode/utf8"
)

func main() {
	var out bytes.Buffer
	for _, lead := range leads() {
		// One pair per line; iconv -c drops pairs it can't decode, which
		// leaves their lines empty.
		var in bytes.Buffer
		for _, trail := range trails() {
			in.Write([]byte{lead, trail, '\n'})
		}
		cmd := exec.Command("iconv", "-c", "-f", "CP932", "-t", "UTF-8")
		cmd.Stdin = &in
		res, err := cmd.Output()
		if err != nil && len(res) == 0 {
			log.Fatalf("iconv: %v", err)
		}
		lines := bytes.Split(bytes.TrimSuffix(res, []byte("\n")), []byte("\n"))
		if len(lines) != len(trails()) {
			log.Fatalf("lead %#x: %d lines, want %d", lead, len(lines), len(trails()))
		}
		for _, line := range lines {
			var unit uint16
			if r, n := utf8.DecodeRune(line); n == len(line) && n > 0 && r != utf8.RuneError {
				if r1, r2 := utf16.EncodeRune(r); r1 == utf8.RuneError {
					unit = uint16(r)
				} else {
					unit = uint16(r1)
					_ = r2 // CP932 has no characters outside the BMP
				}
			}
			binary.Write(&out, binary.BigEndian, unit)
		}
	}
	if err := os.WriteFile("internal/scpt/sjis.bin", out.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
}

func leads() []byte {
	var b []byte
	for c := 0x81; c <= 0xFC; c++ {
		if c <= 0x9F || c >= 0xE0 {
			b = append(b, byte(c))
		}
	}
	return b
}

func trails() []byte {
	var b []byte
	for c := 0x40; c <= 0xFC; c++ {
		if c != 0x7F {
			b = append(b, byte(c))
		}
	}
	return b
}
