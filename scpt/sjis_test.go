package scpt

import "testing"

func TestDecodeShiftJIS(t *testing.T) {
	for in, want := range map[string]string{
		"\x82\xa0\x82\xa2":        "あい",
		"\x83t\x83@\x83C\x83\x8b": "ファイル",
		"\x8a\xbf\x8e\x9a Talk7":  "漢字 Talk7",
		"\xb1\xb2":                "ｱｲ", // half-width katakana
	} {
		got, ok := decodeShiftJIS([]byte(in))
		if !ok || got != want {
			t.Errorf("decodeShiftJIS(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := decodeShiftJIS([]byte("caf\xe9")); ok {
		t.Error("a lone lead byte decoded")
	}
}

// TestLooksJapanese checks that Mac Roman text from Western scripts is not
// mistaken for Shift JIS.
func TestLooksJapanese(t *testing.T) {
	japanese := [][]byte{[]byte("\x83t\x83@\x83C\x83\x8b"), []byte("plain")}
	if !looksJapanese(japanese) {
		t.Error("Japanese text not detected")
	}
	for _, texts := range [][][]byte{
		{[]byte("\xa9Apple Computer, Inc."), []byte("\xa5\xa5\xa5\xa5 Example")}, // © and •
		{[]byte("caf\xe9"), []byte("r\x8esum\x8e")},                              // é
		{[]byte("Fran\x8dais \x88 la carte")},                                    // ç, à
	} {
		if looksJapanese(texts) {
			t.Errorf("%q detected as Japanese", texts)
		}
	}
}
