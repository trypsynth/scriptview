package mactext

import "testing"

func TestShiftJIS(t *testing.T) {
	for in, want := range map[string]string{
		"\x82\xa0\x82\xa2":        "あい",
		"\x83t\x83@\x83C\x83\x8b": "ファイル",
		"\x8a\xbf\x8e\x9a Talk7":  "漢字 Talk7",
		"\xb1\xb2":                "ｱｲ", // half-width katakana
	} {
		got, ok := ShiftJIS([]byte(in))
		if !ok || got != want {
			t.Errorf("ShiftJIS(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := ShiftJIS([]byte("caf\xe9")); ok {
		t.Error("a lone lead byte decoded")
	}
}

func TestMacRoman(t *testing.T) {
	if got := MacRoman([]byte("caf\x8e \xa5 \xc7x\xc8")); got != "café • «x»" {
		t.Errorf("MacRoman = %q", got)
	}
	b, ok := EncodeMacRoman("café • «x»")
	if !ok || string(b) != "caf\x8e \xa5 \xc7x\xc8" {
		t.Errorf("EncodeMacRoman = %q, %v", b, ok)
	}
	if _, ok := EncodeMacRoman("日本"); ok {
		t.Error("EncodeMacRoman accepted text Mac Roman lacks")
	}
}

// TestLooksJapanese tells Japanese text from Western text that happens to
// decode as Shift JIS.
func TestLooksJapanese(t *testing.T) {
	if !LooksJapanese([][]byte{[]byte("\x83t\x83@\x83C\x83\x8b"), []byte("plain")}) {
		t.Error("Japanese text not detected")
	}
	for _, texts := range [][][]byte{
		{[]byte("Fr\x8ed\x8eric Rinaldi")},          // Frédéric: decodes, but as kanji
		{[]byte("caf\xe9"), []byte("r\x8esum\x8e")}, // é
		{[]byte("Fran\x8dais \x88 la carte")},       // ç, à
	} {
		if LooksJapanese(texts) {
			t.Errorf("%q detected as Japanese", texts)
		}
	}
}
