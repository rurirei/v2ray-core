package buffer

import (
	"v2ray.com/core/common/bytespool"
	"v2ray.com/core/common/io"
)

const (
	// Size of a regular buffer.
	Size = 2048
)

// Buffer is a recyclable allocation of a byte array. Buffer.Release() recycles
// the buffer into an internal buffer pool, in order to recreate a buffer more
// quickly.
type Buffer struct {
	v     []byte
	start int
	end   int
}

// New creates a Buffer with 0 length and 2K capacity.
func New() *Buffer {
	return &Buffer{
		v: bytespool.Alloc(Size),
	}
}

// NewSize creates a Buffer with 0 length and capacity with at least the given size.
func NewSize(size int) *Buffer {
	return &Buffer{
		v: bytespool.Alloc(size),
	}
}

// Release recycles the buffer into an internal buffer pool.
func (b *Buffer) Release() {
	b2 := b.v
	bytespool.Free(b2)

	b.v = nil
	b.Clear()
}

// Clear clears the content of the buffer, results an empty buffer with
// Len() = 0.
func (b *Buffer) Clear() {
	b.start = 0
	b.end = 0
}

// Byte returns the bytes at index.
func (b *Buffer) Byte(index int) byte {
	return b.v[b.start+index]
}

// SetByte sets the byte value at index.
func (b *Buffer) SetByte(index int, value byte) {
	b.v[b.start+index] = value
}

// Bytes returns the content bytes of this Buffer.
func (b *Buffer) Bytes() []byte {
	return b.v[b.start:b.end]
}

// Extend increases the buffer size by n bytes, and returns the extended part.
// It panics if result size is larger than buffer.Size.
func (b *Buffer) Extend(n int) []byte {
	end := b.end + n
	if end > len(b.v) {
		panic("extend out of bound")
	}
	ext := b.v[b.end:end]
	b.end = end
	return ext
}

// BytesRange returns a slice of this buffer with given from and to boundary.
func (b *Buffer) BytesRange(from, to int) []byte {
	if from < 0 {
		from += b.Len()
	}
	if to < 0 {
		to += b.Len()
	}
	return b.v[b.start+from : b.start+to]
}

// BytesFrom returns a slice of this Buffer starting from the given position.
func (b *Buffer) BytesFrom(from int) []byte {
	if from < 0 {
		from += b.Len()
	}
	return b.v[b.start+from : b.end]
}

// BytesTo returns a slice of this Buffer from start to the given position.
func (b *Buffer) BytesTo(to int) []byte {
	if to < 0 {
		to += b.Len()
	}
	return b.v[b.start : b.start+to]
}

// Resize cuts the buffer at the given position.
func (b *Buffer) Resize(from, to int) {
	if from < 0 {
		from += b.Len()
	}
	if to < 0 {
		to += b.Len()
	}
	if to < from {
		panic("Invalid slice")
	}
	b.end = b.start + to
	b.start += from
}

// Advance cuts the buffer at the given position.
func (b *Buffer) Advance(from int) {
	if from < 0 {
		from += b.Len()
	}
	b.start += from
}

// Cap returns the capacity of the buffer content.
func (b *Buffer) Cap() int {
	return len(b.v)
}

// Len returns the length of the buffer content.
func (b *Buffer) Len() int {
	return b.end - b.start
}

// IsEmpty returns true if the buffer is empty.
func (b *Buffer) IsEmpty() bool {
	return b.Len() == 0
}

// IsFull returns true if the buffer has no more room to grow.
func (b *Buffer) IsFull() bool {
	return b.end == len(b.v)
}

// Write implements Write method in io.Writer.
func (b *Buffer) Write(data []byte) (int, error) {
	nBytes := copy(b.v[b.end:], data)
	b.end += nBytes
	return nBytes, nil
}

// WriteByte writes a single byte into the buffer.
func (b *Buffer) WriteByte(v byte) error {
	if b.IsFull() {
		return newError("buffer full")
	}
	b.v[b.end] = v
	b.end++
	return nil
}

// WriteString implements io.StringWriter.
func (b *Buffer) WriteString(s string) (int, error) {
	return b.Write([]byte(s))
}

// Read implements io.reader.Read().
func (b *Buffer) Read(data []byte) (int, error) {
	if b.Len() == 0 {
		return 0, io.EOF
	}
	nBytes := copy(data, b.v[b.start:b.end])
	if nBytes == b.Len() {
		b.Clear()
	} else {
		b.start += nBytes
	}
	return nBytes, nil
}

// ReadToWriter reads all data and writes to the given writer
func (b *Buffer) ReadToWriter(writer io.Writer, peek bool) (int, error) {
	if b.Len() == 0 {
		return 0, io.EOF
	}
	nBytes, err := writer.Write(b.v[b.start:b.end])
	if peek {
		if nBytes == b.Len() {
			b.Clear()
		} else {
			b.start += nBytes
		}
	}
	return nBytes, err
}

// ReadFrom implements io.ReaderFrom.
func (b *Buffer) ReadFrom(reader io.Reader) (int64, error) {
	n, err := reader.Read(b.v[b.end:])
	b.end += n
	return int64(n), err
}

// ReadFullFrom reads exact size of bytes from given reader, or until error occurs.
func (b *Buffer) ReadFullFrom(reader io.Reader, size int) (int64, error) {
	end := b.end + size
	if end > len(b.v) {
		v := end
		return 0, newError("out of bound: ", v)
	}
	n, err := io.ReadFull(reader, b.v[b.end:end])
	b.end += n
	return int64(n), err
}

// String returns the string form of this Buffer.
func (b *Buffer) String() string {
	return string(b.Bytes())
}

// FromBytes creates a Buffer with an existed bytearray
// underlying bytes is under unmanaged as do not call Release() on this.
func FromBytes(data []byte) *Buffer {
	return &Buffer{
		v:   data,
		end: len(data),
	}
}

// ReadByte implements io.ByteReader
func (b *Buffer) ReadByte() (byte, error) {
	if b.start == b.end {
		return 0, io.EOF
	}

	nb := b.v[b.start]
	b.start++
	return nb, nil
}

// ReadBytes implements bufio.Reader.ReadBytes
func (b *Buffer) ReadBytes(length int) ([]byte, error) {
	if b.end-b.start < length {
		return nil, io.EOF
	}

	nb := b.v[b.start : b.start+length]
	b.start += length
	return nb, nil
}

// Package buffer provides a light-weight memory allocation mechanism.

//go:generate go run v2ray.com/core/common/errors/errorgen
