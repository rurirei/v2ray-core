package buffer

import (
	"time"

	"v2ray.com/core/common/io"
)

type SpliterFunc = func(MultiBuffer, []byte) (MultiBuffer, int)

// BufferedReader todo: possible new usage
type BufferedReader interface {
	BufferedBytes() int
	ReadAtMost(int) (MultiBuffer, error)

	io.Reader
	Reader
}

// bufferedReader is a Reader that keeps its internal buffer.
type bufferedReader struct {
	// reader is the underlying reader to be read from
	reader Reader
	// spliter is a function to read bytes from MultiBuffer
	spliter SpliterFunc
	// buffer is the internal buffer to be read from first
	buffer MultiBuffer
}

func NewBufferedReader(reader Reader) BufferedReader {
	return &bufferedReader{
		reader:  reader,
		spliter: SplitBytes,
	}
}

// BufferedBytes returns the number of bytes that is cached in this reader.
func (r *bufferedReader) BufferedBytes() int {
	return r.buffer.Len()
}

// ReadByte implements io.ByteReader.
func (r *bufferedReader) ReadByte() (byte, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	return b[0], err
}

// Read implements io.reader.
// It reads from internal buffer first (if available) and then reads from the underlying reader.
func (r *bufferedReader) Read(p []byte) (int, error) {
	if !r.buffer.IsEmpty() {
		buffer, nBytes := r.spliter(r.buffer, p)
		r.buffer = buffer
		if r.buffer.IsEmpty() {
			r.buffer = nil
		}
		return nBytes, nil
	}

	mb, err := r.reader.ReadMultiBuffer()
	if err != nil {
		return 0, err
	}

	mb, nBytes := r.spliter(mb, p)
	if mb != nil && !mb.IsEmpty() {
		r.buffer = mb
	}
	return nBytes, nil
}

// ReadAtMost returns a MultiBuffer with at most size.
func (r *bufferedReader) ReadAtMost(size int) (MultiBuffer, error) {
	if r.buffer.IsEmpty() {
		mb, err := r.reader.ReadMultiBuffer()
		if mb == nil || mb.IsEmpty() && err != nil {
			return nil, err
		}
		r.buffer = mb
	}

	rb, mb := SplitSize(r.buffer, size)
	r.buffer = rb
	if r.buffer.IsEmpty() {
		r.buffer = nil
	}
	return mb, nil
}

// ReadMultiBuffer implements Reader.
func (r *bufferedReader) ReadMultiBuffer() (MultiBuffer, error) {
	if !r.buffer.IsEmpty() {
		mb := r.buffer
		r.buffer = nil
		return mb, nil
	}

	return r.reader.ReadMultiBuffer()
}

// ReadMultiBufferSize returns a MultiBuffer with at most size.
func (r *bufferedReader) ReadMultiBufferSize(size int) (MultiBuffer, error) {
	if r.buffer.IsEmpty() {
		mb, err := r.reader.ReadMultiBuffer()
		if mb == nil || mb.IsEmpty() && err != nil {
			return nil, err
		}
		r.buffer = mb
	}

	rb, mb := SplitSize(r.buffer, size)
	r.buffer = rb
	if r.buffer.IsEmpty() {
		r.buffer = nil
	}
	return mb, nil
}

// WriteTo implements io.WriterTo.
/*func (r *bufferedReader) WriteTo(writer io.Writer) (int64, error) {
	return r.writeToInternal(writer)
}

func (r *bufferedReader) writeToInternal(writer io.Writer) (int64, error) {
	mbWriter := NewWriter(writer)
	var sc SizeCounter
	if r.buffer != nil {
		sc.Size = int64(r.buffer.Len())
		if err := mbWriter.WriteMultiBuffer(r.buffer); err != nil {
			return 0, err
		}
		r.buffer = nil
	}

	err := Copy(mbWriter, r.reader, CountSize(&sc))
	if IsReadError(err) && CauseReadError(err) == io.EOF {
		err = nil
	}
	return sc.Size, err
}*/

// Reader extends io.reader with MultiBuffer.
type Reader interface {
	// ReadMultiBuffer reads content from underlying reader, and put it into a MultiBuffer.
	ReadMultiBuffer() (MultiBuffer, error)
}

// TimeoutReader is a reader that returns error if Read() operation takes longer than the given timeout.
type TimeoutReader interface {
	ReadMultiBufferTimeout(time.Duration) (MultiBuffer, error)
}

type timeoutReader struct {
	reader  TimeoutReader
	timeout time.Duration
}

func NewTimeoutReader(reader TimeoutReader, timeout time.Duration) Reader {
	return &timeoutReader{
		reader:  reader,
		timeout: timeout,
	}
}

func (r *timeoutReader) ReadMultiBuffer() (MultiBuffer, error) {
	return r.reader.ReadMultiBufferTimeout(r.timeout)
}

// ioReader is a Reader that read one Buffer every time.
type ioReader struct {
	io.Reader
}

func NewIOReader(reader io.Reader) Reader {
	return &ioReader{
		Reader: reader,
	}
}

func (r *ioReader) ReadMultiBuffer() (MultiBuffer, error) {
	b, err := readFrom(r)
	return MultiBuffer{b}, err
}

// readFrom reads a Buffer from the given reader.
func readFrom(reader io.Reader) (*Buffer, error) {
	b := New()
	n, err := b.ReadFrom(reader)
	if n > 0 {
		return b, err
	}

	b.Release()
	return nil, err
}

type packetConnReader struct {
	reader io.Reader
}

func NewPacketConnReader(reader io.Reader) Reader {
	return &packetConnReader{
		reader: reader,
	}
}

func (r *packetConnReader) ReadMultiBuffer() (MultiBuffer, error) {
	b, err := readOneUDP(r.reader)
	if err != nil {
		return nil, err
	}
	return MultiBuffer{b}, nil
}

func readOneUDP(reader io.Reader) (*Buffer, error) {
	b := New()
	for i := 0; i < 64; i++ {
		_, err := b.ReadFrom(reader)
		if b != nil && !b.IsEmpty() {
			return b, nil
		}
		if err != nil {
			b.Release()
			return nil, err
		}
	}

	b.Release()
	return nil, newError("reader returns too many empty payloads.")
}
