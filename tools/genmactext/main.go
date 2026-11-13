// Command genmactext writes the tables in internal/mactext/tables from
// Apple's mapping files for the classic Mac OS encodings, which unicode.org
// hosts. With -dir, it reads the files from a folder instead of downloading
// them.
//
// Each table is a list of entries, one for each byte or byte pair above
// 0x7F: the code, 2 bytes big-endian, then the length of its UTF-8 text, 1
// byte, then the text. Apple's grouping hints (U+F860 to U+F87F) and
// direction tags and overrides are dropped, so an entry's text is what it shows.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const base = "https://unicode.org/Public/MAPPINGS/VENDORS/APPLE/"

// files maps each table's name to Apple's file.
var files = map[string]string{
	"japanese":  "JAPANESE.TXT",
	"chinsimp":  "CHINSIMP.TXT",
	"chintrad":  "CHINTRAD.TXT",
	"korean":    "KOREAN.TXT",
	"centeuro":  "CENTEURO.TXT",
	"cyrillic":  "CYRILLIC.TXT",
	"greek":     "GREEK.TXT",
	"hebrew":    "HEBREW.TXT",
	"arabic":    "ARABIC.TXT",
	"farsi":     "FARSI.TXT",
	"thai":      "THAI.TXT",
	"turkish":   "TURKISH.TXT",
	"icelandic": "ICELAND.TXT",
	"croatian":  "CROATIAN.TXT",
	"romanian":  "ROMANIAN.TXT",
}

func main() {
	dir := flag.String("dir", "", "read Apple's files from this folder instead of downloading them")
	out := flag.String("out", "internal/mactext/tables", "where to write the tables")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for name, file := range files {
		src, err := read(*dir, file)
		if err != nil {
			log.Fatalf("%s: %v", file, err)
		}
		table, n, err := convert(src)
		if err != nil {
			log.Fatalf("%s: %v", file, err)
		}
		if err := os.WriteFile(filepath.Join(*out, name+".bin"), table, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %d entries\n", name, n)
	}
}

func read(dir, file string) ([]byte, error) {
	if dir != "" {
		return os.ReadFile(filepath.Join(dir, file))
	}
	resp, err := http.Get(base + file)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// convert reads lines such as "0x8140	0x3000	# IDEOGRAPHIC SPACE", whose
// second column can be a sequence such as "0xF860+0x0030+0x002E".
func convert(src []byte) ([]byte, int, error) {
	var out bytes.Buffer
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(src))
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		code, err := strconv.ParseUint(f[0], 0, 16)
		if err != nil {
			return nil, 0, fmt.Errorf("%q: %v", f[0], err)
		}
		if code < 0x80 {
			continue // ASCII, which scriptview keeps as is
		}
		var text strings.Builder
		for u := range strings.SplitSeq(f[1], "+") {
			if strings.HasPrefix(u, "<") {
				continue // a direction tag, such as <RL>
			}
			r, err := strconv.ParseUint(u, 0, 32)
			if err != nil {
				return nil, 0, fmt.Errorf("%q: %v", f[1], err)
			}
			if r >= 0xF860 && r <= 0xF87F || r >= 0x202A && r <= 0x202E {
				continue
			}
			text.WriteRune(rune(r))
		}
		if text.Len() == 0 || text.Len() > 255 {
			continue
		}
		out.Write([]byte{byte(code >> 8), byte(code), byte(text.Len())})
		out.WriteString(text.String())
		n++
	}
	return out.Bytes(), n, sc.Err()
}
