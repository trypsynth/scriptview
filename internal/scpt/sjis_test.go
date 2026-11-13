package scpt

import (
	"testing"

	"github.com/trypsynth/scriptview/internal/mactext"
)

// TestLegacyEncoding checks that Mac Roman text from Western scripts is not
// mistaken for Shift JIS.
func TestLegacyEncoding(t *testing.T) {
	japanese := [][]byte{[]byte("\x83t\x83@\x83C\x83\x8b"), []byte("plain")}
	if legacyEncoding(japanese) != mactext.Japanese {
		t.Error("Japanese text not detected")
	}
	for _, texts := range [][][]byte{
		{[]byte("\xa9Apple Computer, Inc."), []byte("\xa5\xa5\xa5\xa5 Example")}, // © and •
		{[]byte("caf\xe9"), []byte("r\x8esum\x8e")},                              // é
		{[]byte("Fran\x8dais \x88 la carte")},                                    // ç, à
	} {
		if legacyEncoding(texts) != mactext.Roman {
			t.Errorf("%q detected as Japanese", texts)
		}
	}
}
