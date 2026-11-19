package hfs

import (
	"bytes"
	"compress/bzip2"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"

	"github.com/trypsynth/scriptview/internal/classic"
)

// A chunk is a run of a compressed image's sectors, stored in one piece
// of its data.
type chunk struct {
	sector, count int64 // in 512-byte sectors
	kind          int
	off, n        int64 // where its data is, and how long
}

// The kinds of chunk.
const (
	chunkZero = iota
	chunkRaw
	chunkADC
	chunkZlib
	chunkBzip2
	chunkUnknown
)

// chunkedDisk reads a disk image whose sectors are stored in chunks,
// decompressing one chunk at a time. It isn't safe for concurrent use.
type chunkedDisk struct {
	r      io.ReaderAt
	chunks []chunk // sorted by sector
	size   int64
	last   int // the index of the chunk in buf, or -1
	buf    []byte
}

// maxChunk is the largest chunk to decompress, to stop a damaged image
// from asking for huge buffers.
const maxChunk = 64 << 20

func newChunkedDisk(r io.ReaderAt, chunks []chunk) (*chunkedDisk, error) {
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].sector < chunks[j].sector })
	d := &chunkedDisk{r: r, chunks: chunks, last: -1}
	for _, c := range chunks {
		if c.count < 0 || c.sector < 0 || c.count*512 > maxChunk {
			return nil, fmt.Errorf("hfs: chunk of %d sectors", c.count)
		}
		d.size = max(d.size, (c.sector+c.count)*512)
	}
	return d, nil
}

func (d *chunkedDisk) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= d.size {
			return n, io.EOF
		}
		sector := pos / 512
		i := sort.Search(len(d.chunks), func(i int) bool { return d.chunks[i].sector+d.chunks[i].count > sector })
		if i == len(d.chunks) || d.chunks[i].sector > sector {
			// A gap between chunks reads as zeros, up to the next chunk.
			end := d.size
			if i < len(d.chunks) {
				end = d.chunks[i].sector * 512
			}
			k := min(int64(len(p)-n), end-pos)
			clear(p[n : n+int(k)])
			n += int(k)
			continue
		}
		data, err := d.chunk(i)
		if err != nil {
			return n, err
		}
		c := d.chunks[i]
		n += copy(p[n:], data[pos-c.sector*512:])
	}
	return n, nil
}

// chunk returns chunk i's sectors.
func (d *chunkedDisk) chunk(i int) ([]byte, error) {
	if i == d.last {
		return d.buf, nil
	}
	c := d.chunks[i]
	size := c.count * 512
	out := make([]byte, size)
	if c.kind == chunkZero || c.n == 0 {
		d.last, d.buf = i, out
		return out, nil
	}
	if c.n < 0 || c.n > maxChunk {
		return nil, fmt.Errorf("hfs: chunk of %d bytes", c.n)
	}
	raw := make([]byte, c.n)
	if _, err := d.r.ReadAt(raw, c.off); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	switch c.kind {
	case chunkRaw:
		copy(out, raw)
	case chunkADC:
		copy(out, adc(raw, int(size)))
	case chunkZlib, chunkBzip2:
		var zr io.Reader
		if c.kind == chunkZlib {
			z, err := zlib.NewReader(bytes.NewReader(raw))
			if err != nil {
				return nil, fmt.Errorf("hfs: chunk at sector %d: %w", c.sector, err)
			}
			zr = z
		} else {
			zr = bzip2.NewReader(bytes.NewReader(raw))
		}
		if _, err := io.ReadFull(zr, out); err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("hfs: chunk at sector %d: %w", c.sector, err)
		}
	default:
		return nil, fmt.Errorf("hfs: chunk at sector %d is compressed in a way that scriptview can't read", c.sector)
	}
	d.last, d.buf = i, out
	return out, nil
}

// adc decompresses Apple Data Compression, up to n bytes: each run starts
// with a byte whose high bit means a literal of up to 128 bytes, whose
// next bit means a copy of 4 to 67 bytes from up to 65536 back, given in
// the next two bytes, and otherwise a copy of 3 to 18 bytes from up to
// 1024 back.
func adc(in []byte, n int) []byte {
	out := make([]byte, 0, n)
	back := func(dist, count int) {
		for range count {
			if dist > len(out) || len(out) >= n {
				return
			}
			out = append(out, out[len(out)-dist])
		}
	}
	for i := 0; i < len(in) && len(out) < n; {
		b := in[i]
		switch {
		case b&0x80 != 0:
			k := min(int(b&0x7f)+1, len(in)-i-1, n-len(out))
			out = append(out, in[i+1:i+1+k]...)
			i += 1 + int(b&0x7f) + 1
		case b&0x40 != 0:
			if i+2 >= len(in) {
				return out
			}
			back(int(in[i+1])<<8|int(in[i+2])+1, int(b&0x3f)+4)
			i += 3
		default:
			if i+1 >= len(in) {
				return out
			}
			back((int(b&3)<<8|int(in[i+1]))+1, int(b>>2&0x0f)+3)
			i += 2
		}
	}
	return out
}

// OpenWithResources opens a disk image like Open, given its resource fork
// too: a Disk Copy 6 image (NDIF) keeps its layout in its resource fork.
func OpenWithResources(r io.ReaderAt, size int64, rsrc []byte) ([]*Volume, error) {
	// The layout comes first: an NDIF image's data can start with its
	// disk's first sectors, uncompressed, which look like a bare volume.
	bcem, ok := classic.Resources(rsrc, "bcem")[128]
	if !ok {
		return Open(r, size)
	}
	chunks, err := ndifChunks(bcem, size)
	if err != nil {
		return nil, err
	}
	d, err := newChunkedDisk(r, chunks)
	if err != nil {
		return nil, err
	}
	return open(d, d.size, 1)
}

// ndifChunks reads a Disk Copy 6 image's 'bcem' resource: the number of
// sectors at 0x44, the number of chunks at 0x7C, and from 0x80, 12 bytes
// for each chunk, its first sector, 3 bytes, its kind, its data's offset,
// and its data's length.
func ndifChunks(b []byte, size int64) ([]chunk, error) {
	if len(b) < 0x80 {
		return nil, errors.New("hfs: Disk Copy 6 layout too short")
	}
	sectors := int64(be.Uint32(b[0x44:]))
	n := int(be.Uint32(b[0x7C:]))
	type entry struct {
		start    int64
		kind     byte
		off, len int64
	}
	var es []entry
	for k := 0; k < n && 0x80+12*k+12 <= len(b); k++ {
		e := b[0x80+12*k:]
		es = append(es, entry{int64(e[0])<<16 | int64(e[1])<<8 | int64(e[2]), e[3], int64(be.Uint32(e[4:])), int64(be.Uint32(e[8:]))})
		if e[3] == 0xFF {
			break
		}
	}
	var chunks []chunk
	for i, e := range es {
		if e.kind == 0xFF {
			break
		}
		end := sectors
		if i+1 < len(es) {
			end = es[i+1].start
		}
		if end <= e.start {
			continue
		}
		c := chunk{sector: e.start, count: end - e.start, off: e.off, n: e.len}
		switch e.kind {
		case 0x00, 0x01:
			c.kind = chunkZero
		case 0x02:
			c.kind = chunkRaw
		case 0x83:
			c.kind = chunkADC
		default:
			c.kind = chunkUnknown
		}
		if c.kind != chunkZero && c.off+c.n > size {
			return nil, errors.New("hfs: Disk Copy 6 chunk past the end of the image")
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

// udif opens a .dmg image (UDIF): a 512-byte trailer at the end, "koly",
// with the data fork's offset at 24, and the offset and length of an XML
// property list at 216 and 224. The list's "blkx" entries are base64 chunk
// tables, "mish", each with its first sector at 8, its data's offset at
// 24, its number of chunks at 200, and from 204, 40 bytes for each chunk:
// its kind, its first sector at 8 and its count at 16, both counted from
// the table's first sector, and its data's offset and length at 24 and 32.
func udif(r io.ReaderAt, size int64, koly []byte) (*chunkedDisk, error) {
	dataOff := int64(be.Uint64(koly[24:]))
	xmlOff, xmlLen := int64(be.Uint64(koly[216:])), int64(be.Uint64(koly[224:]))
	if xmlOff < 0 || xmlLen <= 0 || xmlLen > 64<<20 || xmlOff+xmlLen > size {
		return nil, errors.New("hfs: .dmg without a chunk table")
	}
	plist := make([]byte, xmlLen)
	if _, err := r.ReadAt(plist, xmlOff); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var chunks []chunk
	for _, m := range dataRe.FindAllSubmatch(plist, -1) {
		mish, err := base64.StdEncoding.DecodeString(string(bytes.Join(bytes.Fields(m[1]), nil)))
		if err != nil || len(mish) < 204 || string(mish[:4]) != "mish" {
			continue
		}
		first := int64(be.Uint64(mish[8:]))
		base := int64(be.Uint64(mish[24:]))
		n := int(be.Uint32(mish[200:]))
		for k := 0; k < n && 204+40*k+40 <= len(mish); k++ {
			e := mish[204+40*k:]
			c := chunk{
				sector: first + int64(be.Uint64(e[8:])),
				count:  int64(be.Uint64(e[16:])),
				off:    dataOff + base + int64(be.Uint64(e[24:])),
				n:      int64(be.Uint64(e[32:])),
			}
			switch be.Uint32(e) {
			case 0x00000000, 0x00000002:
				c.kind = chunkZero
			case 0x00000001:
				c.kind = chunkRaw
			case 0x80000004:
				c.kind = chunkADC
			case 0x80000005:
				c.kind = chunkZlib
			case 0x80000006:
				c.kind = chunkBzip2
			case 0x7FFFFFFE, 0xFFFFFFFF: // a comment, the end
				continue
			default: // LZFSE and LZMA, from later versions of macOS
				c.kind = chunkUnknown
			}
			if c.count > 0 {
				chunks = append(chunks, c)
			}
		}
	}
	if len(chunks) == 0 {
		return nil, errors.New("hfs: .dmg without chunks")
	}
	return newChunkedDisk(r, chunks)
}

var dataRe = regexp.MustCompile(`(?s)<data>(.*?)</data>`)
