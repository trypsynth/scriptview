package mactext

import (
	"slices"
	"testing"
)

func TestDecode(t *testing.T) {
	for _, c := range []struct {
		e        *Encoding
		in, want string
	}{
		{Japanese, "\x82\xa0\x82\xa2", "あい"},
		{Japanese, "\x83t\x83@\x83C\x83\x8b", "ファイル"},
		{Japanese, "\x8a\xbf\x8e\x9a Talk7", "漢字 Talk7"},
		{Japanese, "\xb1\xb2", "ｱｲ"},   // half-width katakana
		{Japanese, "\x86\xa1", "♡"},    // an Apple addition
		{Japanese, "\x85\xab", "XIII"}, // a sequence, without its hint
		{Japanese, "\x80\xfd", "\\©"},  // one-byte additions
		{SimpChinese, "\xb5\xc4", "的"},
		{TradChinese, "\xaa\xba", "的"},
		{Korean, "\xc0\xcc", "이"},
		{Cyrillic, "\xe4\xe0", "да"},
		{Hebrew, "\xf9\xec\xe5\xed", "שלום"},
		{CentralEuropean, "\x88", "ą"},
	} {
		if got, ok := c.e.Decode([]byte(c.in)); !ok || got != c.want {
			t.Errorf("%s.Decode(%q) = %q, %v; want %q", c.e.Name, c.in, got, ok, c.want)
		}
	}
	// A lone lead byte, and rows of CP932 that Mac Japanese lacks.
	for _, in := range []string{"caf\xe9", "\xfa\x40", "\xf0\x40"} {
		if s, ok := Japanese.Decode([]byte(in)); ok {
			t.Errorf("Japanese.Decode(%q) = %q, want failure", in, s)
		}
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
func TestDetect(t *testing.T) {
	b := func(s ...string) [][]byte {
		var out [][]byte
		for _, x := range s {
			out = append(out, []byte(x))
		}
		return out
	}
	for _, c := range []struct {
		texts [][]byte
		hints []*Encoding
		want  *Encoding
	}{
		{b("\x83t\x83@\x83C\x83\x8b", "plain"), nil, Japanese},
		// One string that doesn't decode doesn't sink the rest.
		{append(slices.Repeat(b("\x82\xa0\x82\xa2"), 10), []byte("caf\xe9")), nil, Japanese},
		{b("Fr\x8ed\x8eric Rinaldi"), nil, Roman},
		{b("caf\xe9", "r\x8esum\x8e"), nil, Roman},
		{b("Fran\x8dais \x88 la carte"), nil, Roman},
		// É and the next letter make katakana: "Électromécanicien".
		{slices.Repeat(b("\x83lectrom\x8ecanicien", "Pr\x9fungszeugnis Ger\x8atetechnik"), 5), nil, Roman},
		// Curly quotes are half-width katakana in Shift JIS.
		{b("haven\xd5t", "\xd2Build\xd3", "it\xd5s", "\xa9 1994"), nil, Roman},
		{b("\x82\xa0\x82\xa2\xa1", "\xa2\x82\xa4\x82\xa6\xa3"), nil, Japanese},                                   // with ｡, ｢, and ｣
		{b("\x82\xa0\x82\xa2 and\xc2\r\x82\xa4\x82\xa6", "if a then\xc2\rb", "if b then\xc2\rc"), nil, Japanese}, // ¬
		{b("\xb5\xc4\xca\xc7\xd2\xbb"), nil, Roman},
		{b("\xb5\xc4\xca\xc7\xd2\xbb"), []*Encoding{SimpChinese}, SimpChinese},
		{b("\xc0\xcc\xb4\xd9"), []*Encoding{Korean}, Korean},
		{b("\x91\xef\xe0\xf1\xe8\xe1\xee", "\x8f\xf0\xe8\xe2\xe5\xf2"), []*Encoding{Cyrillic}, Cyrillic}, // Спасибо, Привет
		{b("caf\x8e"), []*Encoding{Cyrillic}, Roman},
	} {
		if got := Detect(c.texts, c.hints...); got != c.want {
			t.Errorf("Detect(%q, %v) = %s, want %s", c.texts, c.hints, got.Name, c.want.Name)
		}
	}
}

func TestForFont(t *testing.T) {
	for _, c := range []struct {
		id   int
		name string
		want *Encoding
	}{
		{16384, "Osaka", Japanese},
		{3, "Osaka", Japanese},
		{16385, "\x8d\xd7\x96\xbe\x92\xa9\x91\xcc", Japanese}, // 細明朝体
		{28672, "*Beijing*", SimpChinese},
		{27710, "Times New Roman", nil},
		{30999, "GPalatino", nil},
		{3, "Geneva", nil},
		{2, "Chicago CE", CentralEuropean},
		{4, "Heiseiish", nil},
	} {
		if got := ForFont(c.id, []byte(c.name)); got != c.want {
			t.Errorf("ForFont(%d, %q) = %v, want %v", c.id, c.name, got, c.want)
		}
	}
}
