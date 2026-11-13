package scpt

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// fixtureDir returns the path to the testdata directory relative to this file.
func fixtureDir(t *testing.T) string {
	t.Helper()
	// internal/scpt/ → ../../testdata
	return filepath.Join("..", "..", "testdata")
}

// readFixture loads a file from testdata/scpt/.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(fixtureDir(t), "scpt", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return data
}

// --- Magic / header tests ---

func TestMagicAccepted(t *testing.T) {
	data := readFixture(t, "hello.scpt")
	_, err := Parse(data)
	if err != nil {
		t.Fatalf("valid .scpt rejected: %v", err)
	}
}

func TestPlainTextPassthrough(t *testing.T) {
	f, err := Parse([]byte("display dialog \"hi\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := Decompile(f); got != "display dialog \"hi\"\n" {
		t.Errorf("got %q", got)
	}
}

func TestMagicRejected(t *testing.T) {
	for _, bad := range [][]byte{
		[]byte("\x00\x01binary junk"),
		append([]byte("FasdUAS 1.101.1 "), make([]byte, 100)...),
		[]byte{},
		make([]byte, 4),
	} {
		_, err := Parse(bad)
		if err != ErrNotSCPT {
			t.Errorf("expected ErrNotSCPT for %q, got %v", bad[:min(len(bad), 20)], err)
		}
	}
}

// --- String literal tests ---

func TestStringLiteralUTF16BE(t *testing.T) {
	// Build a synthetic FASD file containing a known string literal.
	// OpcodeText(0x11) + node_id(2BE) + byte_len(2BE) + UTF-16BE
	input := "café"
	u16 := utf16.Encode([]rune(input))
	raw := make([]byte, len(u16)*2)
	for i, v := range u16 {
		raw[2*i] = byte(v >> 8)
		raw[2*i+1] = byte(v)
	}
	byteLen := len(raw)

	var payload []byte
	payload = append(payload, OpcodeText)
	payload = append(payload, 0x00, 0x01) // node_id = 1
	payload = append(payload, byte(byteLen>>8), byte(byteLen))
	payload = append(payload, raw...)

	data := append(append([]byte(nil), Magic...), payload...)
	data = append(data, Trailer...)

	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(slices.Collect(maps.Values(f.dc.textByID)), input) {
		t.Errorf("expected %q in texts %v", input, f.dc.textByID)
	}
}

// --- Identifier parsing unit tests ---

func TestParseIdentDataSingleChar(t *testing.T) {
	// 0x30 + 2-byte-BE-len(1) + 'x' + 2-byte-BE-len(1) + 'x'  (lowercase == original)
	data := []byte{0x30, 0x00, 0x01, 'x', 0x00, 0x01, 'x'}
	names := parseIdentData(data)
	if len(names) != 2 || names[0] != "x" || names[1] != "x" {
		t.Errorf("got %v", names)
	}
}

func TestParseIdentDataMixedCase(t *testing.T) {
	// 0x30 + 2-byte-BE-len(18) + "isvoiceoverrunning" + 2-byte-BE-len(18) + "isVoiceOverRunning"
	lower := "isvoiceoverrunning"
	orig := "isVoiceOverRunning"
	data := []byte{0x30, 0x00, byte(len(lower))}
	data = append(data, []byte(lower)...)
	data = append(data, 0x00, byte(len(orig)))
	data = append(data, []byte(orig)...)
	names := parseIdentData(data)
	if len(names) != 2 || names[0] != lower || names[1] != orig {
		t.Errorf("got %v, want [%q, %q]", names, lower, orig)
	}
}

func TestParseIdentDataWrongMarker(t *testing.T) {
	data := []byte{0x31, 0x01, 'x'}
	names := parseIdentData(data)
	if len(names) != 0 {
		t.Errorf("expected no names for wrong marker, got %v", names)
	}
}

func TestParseIdentDataEmpty(t *testing.T) {
	if names := parseIdentData(nil); len(names) != 0 {
		t.Errorf("expected empty, got %v", names)
	}
}

// --- UTF-16BE decode unit tests ---

func TestDecodeUTF16BEASCIIRange(t *testing.T) {
	// "hello" in UTF-16BE
	raw := []byte{0x00, 'h', 0x00, 'e', 0x00, 'l', 0x00, 'l', 0x00, 'o'}
	got := decodeUTF16BE(raw)
	if got != "hello" {
		t.Errorf("got %q, want \"hello\"", got)
	}
}

func TestDecodeUTF16BEOddLen(t *testing.T) {
	// Odd-length input: last byte is dropped.
	raw := []byte{0x00, 'h', 0x00, 'e', 0x00}
	got := decodeUTF16BE(raw)
	if got != "he" {
		t.Errorf("got %q, want \"he\"", got)
	}
}

func TestDecodeUTF16BEEmpty(t *testing.T) {
	got := decodeUTF16BE(nil)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestJunkAfterTrailer(t *testing.T) {
	data := append(readFixture(t, "hello.scpt"), []byte("junk\r\n")...)
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := Decompile(f); got != "display dialog \"hello world\"\n" {
		t.Errorf("got %q", got)
	}
}

func TestShebangBeforeMagic(t *testing.T) {
	data := append([]byte("#!/usr/bin/osascript\n"), readFixture(t, "hello.scpt")...)
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := Decompile(f); got != "display dialog \"hello world\"\n" {
		t.Errorf("got %q", got)
	}
}

// TestClassicContainers reads a compiled script from a bare resource fork,
// an AppleDouble file, MacBinary and BinHex, as classic Mac OS kept them.
func TestClassicContainers(t *testing.T) {
	want, err := os.ReadFile(filepath.Join(fixtureDir(t), "expected", "handlers2.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"handlers2.rsrc", "handlers2.appledouble", "handlers2.bin", "handlers2.hqx"} {
		data, err := os.ReadFile(filepath.Join(fixtureDir(t), "classic", name))
		if err != nil {
			t.Fatal(err)
		}
		f, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := Decompile(f); strings.TrimSpace(got) != strings.TrimSpace(string(want)) {
			t.Errorf("%s: got\n%s", name, got)
		}
		if !f.dc.classic {
			t.Errorf("%s: not read as a classic script", name)
		}
	}
	if f, err := Parse(readFixture(t, "hello.scpt")); err != nil || f.dc.classic {
		t.Errorf("hello.scpt read as a classic script (err %v)", err)
	}
}

// TestOldHeader reads a script whose header has the single version field
// that AppleScript 1.0 wrote ("FasdUAS 1.00").
func TestOldHeader(t *testing.T) {
	data := readFixture(t, "hello.scpt")
	old := append([]byte("FasdUAS 1.00"), data[len(Magic):]...)
	f, err := Parse(old)
	if err != nil {
		t.Fatal(err)
	}
	if got := Decompile(f); got != "display dialog \"hello world\"\n" {
		t.Errorf("got %q", got)
	}
	for _, bad := range []string{"FasdUAS 1.10", "FasdUAS x.00", "FasdUAS 1.101.1 "} {
		if fasdHeaderLen([]byte(bad+"    ")) != 0 {
			t.Errorf("%q accepted", bad)
		}
	}
}

// TestLegacySymbols decodes AppleScript 1.0 symbols: table index*8+1.
func TestLegacySymbols(t *testing.T) {
	for idx, want := range map[uint32]string{
		109: "ret ", 121: "true", 122: "fals", 2: "list", 51: "file", 99: "from",
		132: "ctxt", 127: "aevtodoc", 75: "ascr≠   ", 123: "cura",
	} {
		v := idx*8 + 1
		c, ok := legacySymbol([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}, false)
		if !ok || c.code != want {
			t.Errorf("symbol %d: got %v %v, want %s", idx, c, ok, want)
		}
	}
	// The 1.0b1 beta (FASD 0.95) orders the table differently.
	for idx, want := range map[uint32]string{2: "list", 95: "insh", 96: "kocl", 125: "true", 127: "cura"} {
		v := idx*8 + 1
		c, ok := legacySymbol([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}, true)
		if !ok || c.code != want {
			t.Errorf("beta symbol %d: got %v %v, want %s", idx, c, ok, want)
		}
	}
	if _, ok := legacySymbol([]byte{0, 0, 3, 0xd2}, false); ok {
		t.Error("untagged value accepted")
	}
}

// TestLegacyRunOnlyHandler decodes an AppleScript 1.0 run-only script whose
// root object is its run handler, not a script object. 1.0 also builds every
// list literal with MakeList.
func TestLegacyRunOnlyHandler(t *testing.T) {
	var d []byte
	d = append(d, "FasdUAS 1.00"...)
	// The run handler: a kind 16 vector of seven references.
	d = append(d, 0x0e, 0x00, 0x00, 0x00, 0x07, 16,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xfd, 0xff, 0xfc, 0x00, 0x01, 0x00, 0x02, 0xff, 0xfb)
	d = append(d, 0x01, 0xff, 0xff, 0x00, 0x04, 0x00, 0x00, 0x01, 0xd1) // symbol 58: run
	d = append(d, 0x01, 0xff, 0xfe, 0x00, 0x00)                         // no parameters
	d = append(d, 0x01, 0xff, 0xfd, 0x00, 0x00)
	d = append(d, 0x02, 0xff, 0xfc, 0x00, 0x00) // empty list
	d = append(d, 0x10, 0x00, 0x01, 0x00, 0x00) // no variables
	d = append(d, 0x10, 0x00, 0x02, 0x00, 0x00) // no literals
	// Push1 Push1 MakeList Return: {1}
	d = append(d, 0x11, 0xff, 0xfb, 0x00, 0x04, 0x6b, 0x6b, 0x0d, 0x0f)
	d = append(d, "ascr\x00\x01\x00\x0c\xfa\xde\xde\xad"...)
	f, err := Parse(d)
	if err != nil {
		t.Fatal(err)
	}
	got := Decompile(f)
	if !strings.HasSuffix(got, "\n{1}\n") {
		t.Errorf("got %q", got)
	}
}

// TestClassicTerms checks that classic Mac OS terminology (classic.tsv)
// names codes only in scripts from classic Mac OS.
func TestClassicTerms(t *testing.T) {
	data := readFixture(t, "classic_terms.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := Decompile(f); !strings.Contains(got, "«event earsack »") {
		t.Errorf("current script: got %q, want the raw code", got)
	}
	old := append([]byte("FasdUAS 1.00"), data[len(Magic):]...)
	if f, err = Parse(old); err != nil {
		t.Fatal(err)
	}
	if got := Decompile(f); got != "acknowledge saying \"Hello\"\n" {
		t.Errorf("AppleScript 1.0 script: got %q", got)
	}
}

// TestLegacyCopyResult decodes `copy 1 to x` as AppleScript 1.0 compiles it
// when the copy is also the script's result: Clone Dup PopGlobal.
func TestLegacyCopyResult(t *testing.T) {
	var d []byte
	d = append(d, "FasdUAS 1.00"...)
	d = append(d, 0x0e, 0x00, 0x00, 0x00, 0x07, 16,
		0xff, 0xff, 0xff, 0xfe, 0xff, 0xfd, 0xff, 0xfc, 0x00, 0x01, 0x00, 0x02, 0xff, 0xfb)
	d = append(d, 0x01, 0xff, 0xff, 0x00, 0x04, 0x00, 0x00, 0x01, 0xd1) // symbol 58: run
	d = append(d, 0x01, 0xff, 0xfe, 0x00, 0x00)
	d = append(d, 0x01, 0xff, 0xfd, 0x00, 0x00)
	d = append(d, 0x02, 0xff, 0xfc, 0x00, 0x00)
	d = append(d, 0x10, 0x00, 0x01, 0x00, 0x00)             // no variables
	d = append(d, 0x10, 0x00, 0x02, 0x00, 0x01, 0xff, 0xfa) // literals: x
	d = append(d, 0x0b, 0xff, 0xfa, 0x00, 0x05, 0x30, 0x00, 0x01, 'x', 0x00, 0x00)
	// Push1 GetData Clone Dup PopGlobal x StoreResult GetResult Return
	d = append(d, 0x11, 0xff, 0xfb, 0x00, 0x08, 0x6b, 0x45, 0x51, 0x5b, 0xd0, 0x4f, 0x50, 0x0f)
	d = append(d, "ascr\x00\x01\x00\x0c\xfa\xde\xde\xad"...)
	f, err := Parse(d)
	if err != nil {
		t.Fatal(err)
	}
	got := Decompile(f)
	if !strings.Contains(got, "copy 1 to x\n") || strings.Contains(got, "incomplete") {
		t.Errorf("got %q", got)
	}
}
