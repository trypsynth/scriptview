package scpt

import (
	"encoding/binary"
	"io"
)

// reader is a cursor over a byte slice with big-endian read helpers.
type reader struct {
	data []byte
	pos  int
}

func newReader(data []byte) *reader { return &reader{data: data} }

func (r *reader) remaining() int { return len(r.data) - r.pos }
func (r *reader) offset() int    { return r.pos }
func (r *reader) eof() bool      { return r.pos >= len(r.data) }

func (r *reader) peek() (byte, bool) {
	if r.eof() {
		return 0, false
	}
	return r.data[r.pos], true
}

func (r *reader) readByte() (byte, error) {
	if r.eof() {
		return 0, io.ErrUnexpectedEOF
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *reader) readBytes(n int) ([]byte, error) {
	if r.remaining() < n {
		return nil, io.ErrUnexpectedEOF
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *reader) readU16BE() (uint16, error) {
	b, err := r.readBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (r *reader) readI16BE() (int16, error) {
	v, err := r.readU16BE()
	return int16(v), err
}

func (r *reader) skip(n int) error {
	if r.remaining() < n {
		return io.ErrUnexpectedEOF
	}
	r.pos += n
	return nil
}
