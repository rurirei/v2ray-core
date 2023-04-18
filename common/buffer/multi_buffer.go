package buffer

import (
	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/serial"
)

// WriteAllBytes ensures all bytes are written into the given writer.
func WriteAllBytes(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if err != nil {
			return err
		}
		payload = payload[n:]
	}
	return nil
}

// ReadAllToBytes reads all content from the reader into a byte array, until EOF.
func ReadAllToBytes(reader io.Reader) ([]byte, error) {
	mb, err := ReadMultiFrom(reader)
	if err != nil {
		return nil, err
	}
	if mb.Len() == 0 {
		return nil, nil
	}

	b := make([]byte, mb.Len())
	mb, _ = SplitBytes(mb, b)
	ReleaseMulti(mb)

	return b, nil
}

// MultiBuffer is a list of Buffers. The order of Buffer matters.
type MultiBuffer []*Buffer

// Len returns the total number of bytes in the MultiBuffer.
func (mb MultiBuffer) Len() int {
	size := 0
	for _, b := range mb {
		size += b.Len()
	}
	return size
}

// IsEmpty return true if the MultiBuffer has no content.
func (mb MultiBuffer) IsEmpty() bool {
	for _, b := range mb {
		if b != nil && !b.IsEmpty() {
			return false
		}
	}
	return true
}

// CopyBytes copied the beginning part of the MultiBuffer into the given byte array.
func (mb MultiBuffer) CopyBytes(b []byte) int {
	total := 0

	for _, b0 := range mb {
		nBytes := copy(b[total:], b0.Bytes())
		// nBytes, _ := b0.Read(b[total:])
		total += nBytes
		if nBytes < b0.Len() {
			break
		}
	}

	return total
}

// String returns the content of the MultiBuffer in string.
func (mb MultiBuffer) String() string {
	v := make([]interface{}, len(mb))
	for i, b := range mb {
		v[i] = b
	}
	return serial.Concat(v...)
}

// MergeBytes merges the given bytes into MultiBuffer and return the new address of the merged MultiBuffer.
func MergeBytes(dst MultiBuffer, src []byte) MultiBuffer {
	n := len(dst)
	if n > 0 && !(dst)[n-1].IsFull() {
		nBytes, _ := (dst)[n-1].Write(src)
		src = src[nBytes:]
	}

	for len(src) > 0 {
		b := New()
		nBytes, _ := b.Write(src)
		src = src[nBytes:]
		dst = append(dst, b)
	}

	return dst
}

// MergeMulti merges content from src to dest, and returns the new address of dest and src
func MergeMulti(dst, src MultiBuffer) MultiBuffer {
	for idx := range src {
		dst = append(dst, src[idx])
		src[idx] = nil
	}
	return dst
}

// ReleaseMulti release all content of the MultiBuffer, and returns an empty MultiBuffer.
func ReleaseMulti(mb MultiBuffer) MultiBuffer {
	for i := range mb {
		if mb[i] != nil {
			mb[i].Release()
			mb[i] = nil
		}
	}
	mb = nil
	return mb
}

// ReadMultiFrom reads all content from reader until EOF.
func ReadMultiFrom(reader io.Reader) (MultiBuffer, error) {
	mb := make(MultiBuffer, 0, 16)
	for {
		b := New()
		_, err := b.ReadFullFrom(reader, Size)
		if b == nil || b.IsEmpty() {
			b.Release()
		} else {
			mb = append(mb, b)
		}
		if err != nil {
			if errors.Cause(err) == io.EOF || errors.Cause(err) == io.ErrUnexpectedEOF {
				return mb, nil
			}
			return mb, err
		}
	}
}

// WriteMultiTo writes all buffers from the MultiBuffer to the Writer one by one, and return error if any, and release the leftover MultiBuffer.
func WriteMultiTo(writer io.Writer, mb MultiBuffer) (MultiBuffer, error) {
	for {
		mb0, b := SplitFirst(mb)
		mb = mb0

		if b == nil {
			break
		}

		_, err := b.ReadToWriter(writer, true)

		b.Release()

		if err != nil {
			return mb, err
		}
	}

	return mb, nil
}

// SplitBytes splits the given amount of bytes from the beginning of the MultiBuffer.
// It returns the new address of MultiBuffer leftover, and number of bytes written into the input byte slice.
func SplitBytes(mb MultiBuffer, b []byte) (MultiBuffer, int) {
	totalBytes := 0
	endIndex := -1

	for i := range mb {
		pBuffer := mb[i]
		nBytes, _ := pBuffer.Read(b)
		totalBytes += nBytes

		b = b[nBytes:]
		if pBuffer != nil && !pBuffer.IsEmpty() {
			endIndex = i
			break
		}

		pBuffer.Release()
		mb[i] = nil
	}

	if endIndex == -1 {
		mb = mb[:0]
	} else {
		mb = mb[endIndex:]
	}

	return mb, totalBytes
}

// SplitFirstBytes splits the first buffer from MultiBuffer, and then copy its content into the given slice.
func SplitFirstBytes(mb MultiBuffer, p []byte) (MultiBuffer, int) {
	mb, b := SplitFirst(mb)
	if b == nil {
		return mb, 0
	}

	n, _ := b.Read(p)
	b.Release()
	return mb, n
}

// Compact returns another MultiBuffer by merging all content of the given one together.
func Compact(mb MultiBuffer) MultiBuffer {
	if len(mb) == 0 {
		return mb
	}

	mb0 := make(MultiBuffer, 0, len(mb))
	last := mb[0]

	for i := 1; i < len(mb); i++ {
		curr := mb[i]
		if last.Len()+curr.Len() > Size {
			mb0 = append(mb0, last)
			last = curr
		} else {
			_, _ = last.ReadFrom(curr)
			curr.Release()
		}
	}

	mb0 = append(mb0, last)
	return mb0
}

// SplitFirst splits the first Buffer from the beginning of the MultiBuffer.
func SplitFirst(mb MultiBuffer) (MultiBuffer, *Buffer) {
	if len(mb) == 0 {
		return mb, nil
	}

	b := mb[0]
	mb[0] = nil
	mb = mb[1:]
	return mb, b
}

// SplitSize splits the beginning of the MultiBuffer into another one, for at most size bytes.
func SplitSize(mb MultiBuffer, size int) (MultiBuffer, MultiBuffer) {
	if len(mb) == 0 {
		return mb, nil
	}

	if mb[0].Len() > size {
		b := New()
		copy(b.Extend(size), mb[0].BytesTo(size))
		mb[0].Advance(size)
		return mb, MultiBuffer{b}
	}

	totalBytes := 0
	var r MultiBuffer
	endIndex := -1
	for i := range mb {
		if totalBytes+mb[i].Len() > size {
			endIndex = i
			break
		}
		totalBytes += mb[i].Len()
		r = append(r, mb[i])
		mb[i] = nil
	}
	if endIndex == -1 {
		// To reuse mb array
		mb = mb[:0]
	} else {
		mb = mb[endIndex:]
	}
	return mb, r
}

// multiBufferWrapper is a ReadWriteCloser wrapper over MultiBuffer.
type multiBufferWrapper struct {
	mb MultiBuffer
}

// Read implements io.reader.
func (mbw multiBufferWrapper) Read(p []byte) (int, error) {
	if mbw.mb == nil || mbw.mb.IsEmpty() {
		return 0, io.EOF
	}

	mb, nBytes := SplitBytes(mbw.mb, p)
	mbw.mb = mb
	return nBytes, nil
}

// ReadMultiBuffer implements Reader.
func (mbw multiBufferWrapper) ReadMultiBuffer() (MultiBuffer, error) {
	mb := mbw.mb
	mbw.mb = nil
	return mb, nil
}

// Write implements io.Writer.
func (mbw multiBufferWrapper) Write(p []byte) (int, error) {
	mb := MergeBytes(mbw.mb, p)
	mbw.mb = mb
	return len(p), nil
}

// WriteMultiBuffer WriteMultiTo implement Writer.
func (mbw multiBufferWrapper) WriteMultiBuffer(mp MultiBuffer) error {
	mbw.mb = MergeMulti(mbw.mb, mp)
	return nil
}

// Close implement io.Closer.
func (mbw multiBufferWrapper) Close() error {
	mbw.mb = ReleaseMulti(mbw.mb)

	return nil
}
