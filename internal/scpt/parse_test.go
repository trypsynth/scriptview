package scpt

import (
	"os"
	"path/filepath"
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- String literal tests ---

func TestStringLiteralHello(t *testing.T) {
	data := readFixture(t, "hello.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(f.Strings, "hello world") {
		t.Errorf("expected \"hello world\" in strings %v", f.Strings)
	}
}

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
	if !containsString(f.Strings, input) {
		t.Errorf("expected %q in strings %v", input, f.Strings)
	}
}

// --- Identifier tests ---

func TestIdentifierExtraction(t *testing.T) {
	data := readFixture(t, "test.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	// test.scpt: set x to 42 / set y to x + 1 / return y
	for _, want := range []string{"x", "y"} {
		if !containsIdent(f.Idents, want) {
			t.Errorf("identifier %q not found in %v", want, f.Idents)
		}
	}
}

func TestIdentifierExtractionLargeScript(t *testing.T) {
	data := readFixture(t, "uptime.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	// The uptime script should contain an identifier like "uptime"
	if len(f.Idents) == 0 {
		t.Error("expected at least one identifier in uptime.scpt")
	}
	t.Logf("identifiers found: %v", f.Idents)
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

// --- Integer literal tests ---

func TestIntLiteralTest(t *testing.T) {
	data := readFixture(t, "test.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	// test.scpt has set x to 42 and set y to x + 1 → integers 42 and 1
	if !containsInt(f.Ints, 42) {
		t.Errorf("expected 42 in integers %v", f.Ints)
	}
	if !containsInt(f.Ints, 1) {
		t.Errorf("expected 1 in integers %v", f.Ints)
	}
}

// --- Extract end-to-end tests ---

func TestExtractHello(t *testing.T) {
	path := filepath.Join(fixtureDir(t), "scpt", "hello.scpt")
	src, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "hello world") {
		t.Errorf("expected \"hello world\" in extracted source:\n%s", src)
	}
}

func TestExtractTest(t *testing.T) {
	path := filepath.Join(fixtureDir(t), "scpt", "test.scpt")
	src, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"x", "y", "42"} {
		if !strings.Contains(src, want) {
			t.Errorf("expected %q in extracted source:\n%s", want, src)
		}
	}
}

func TestExtractUptime(t *testing.T) {
	path := filepath.Join(fixtureDir(t), "scpt", "uptime.scpt")
	src, err := Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if src == "" {
		t.Error("expected non-empty source for uptime.scpt")
	}
	t.Logf("extracted:\n%s", src)
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

// --- Best-effort extraction tests ---

func TestBestEffortSourceContainsIdents(t *testing.T) {
	data := readFixture(t, "test.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	src := BestEffortSource(f)
	for _, id := range []string{"x", "y"} {
		if !strings.Contains(src, id) {
			t.Errorf("best-effort source missing identifier %q:\n%s", id, src)
		}
	}
}

func TestBestEffortSourceContainsIntegers(t *testing.T) {
	data := readFixture(t, "test.scpt")
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	src := BestEffortSource(f)
	if !strings.Contains(src, "42") {
		t.Errorf("best-effort source missing integer 42:\n%s", src)
	}
}

// --- helpers ---

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func containsIdent(ss []string, want string) bool {
	return containsString(ss, want)
}

func containsInt(ints []int, want int) bool {
	for _, v := range ints {
		if v == want {
			return true
		}
	}
	return false
}
