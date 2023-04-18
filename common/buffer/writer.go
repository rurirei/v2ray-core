package buffer

import (
	"sync"

	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
)

// BufferedWriter todo: possible new usage
type BufferedWriter interface {
	SetBuffered(bool) error

	io.Writer
	Writer
}

// bufferedWriter is a Writer with internal buffer.
type bufferedWriter struct {
	sync.Mutex

	writer   Writer
	buffer   *Buffer
	buffered bool
}

// NewBufferedWriter creates a new bufferedWriter.
func NewBufferedWriter(writer Writer) BufferedWriter {
	return &bufferedWriter{
		writer:   writer,
		buffer:   New(),
		buffered: true,
	}
}

// WriteByte implements io.ByteWriter.
func (w *bufferedWriter) WriteByte(c byte) error {
	_, err := w.Write([]byte{c})
	return err
}

// Write implements io.Writer.
func (w *bufferedWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	w.Lock()
	defer w.Unlock()

	if !w.buffered {
		if writer, ok := w.writer.(io.Writer); ok {
			return writer.Write(b)
		}
	}

	totalBytes := 0
	for len(b) > 0 {
		if w.buffer == nil {
			w.buffer = New()
		}

		nBytes, err := w.buffer.Write(b)
		totalBytes += nBytes
		if err != nil {
			return totalBytes, err
		}
		if !w.buffered || w.buffer.IsFull() {
			if err := w.flushInternal(); err != nil {
				return totalBytes, err
			}
		}
		b = b[nBytes:]
	}

	return totalBytes, nil
}

// WriteMultiBuffer implements Writer.
// It takes ownership of the given MultiBuffer.
func (w *bufferedWriter) WriteMultiBuffer(mb MultiBuffer) error {
	w.Lock()
	defer w.Unlock()

	if !w.buffered {
		return w.writer.WriteMultiBuffer(mb)
	}

	reader := multiBufferWrapper{
		mb: mb,
	}
	defer func() {
		_ = reader.Close()
	}()

	for reader.mb != nil && !reader.mb.IsEmpty() {
		if w.buffer == nil {
			w.buffer = New()
		}
		_, _ = w.buffer.ReadFrom(&reader)
		if w.buffer.IsFull() {
			if err := w.flushInternal(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Flush flushes buffered content into underlying writer.
func (w *bufferedWriter) Flush() error {
	w.Lock()
	defer w.Unlock()

	return w.flushInternal()
}

func (w *bufferedWriter) flushInternal() error {
	if w.buffer == nil || w.buffer.IsEmpty() {
		return nil
	}

	b := w.buffer
	w.buffer = nil

	if writer, ok := w.writer.(io.Writer); ok {
		err := WriteAllBytes(writer, b.Bytes())
		b.Release()
		return err
	}

	return w.writer.WriteMultiBuffer(MultiBuffer{b})
}

// SetBuffered sets whether the internal buffer is used.
// If set to false, Flush() will be called to clear the buffer.
func (w *bufferedWriter) SetBuffered(f bool) error {
	w.Lock()
	defer w.Unlock()

	w.buffered = f
	if !f {
		return w.flushInternal()
	}
	return nil
}

// ReadFrom implements io.ReaderFrom.
/*func (w *bufferedWriter) ReadFrom(reader io.Reader) (int64, error) {
	if err := w.SetBuffered(false); err != nil {
		return 0, err
	}

	var sc SizeCounter
	err := Copy(w, NewReader(reader), CountSize(&sc))
	return sc.Size, err
}*/

// Writer extends io.Writer with MultiBuffer.
type Writer interface {
	// WriteMultiBuffer writes a MultiBuffer into underlying writer.
	WriteMultiBuffer(MultiBuffer) error
}

// allToBytesWriter is a Writer that writes alloc.buffer into underlying writer.
type allToBytesWriter struct {
	io.Writer

	cache net.Buffers
}

func NewAllToBytesWriter(writer io.Writer) Writer {
	return &allToBytesWriter{
		Writer: writer,
	}
}

// WriteMultiBuffer implements Writer.
// This method takes ownership of the given buffer.
func (w *allToBytesWriter) WriteMultiBuffer(mb MultiBuffer) error {
	defer ReleaseMulti(mb)

	size := mb.Len()
	if size == 0 {
		return nil
	}

	if len(mb) == 1 {
		return WriteAllBytes(w.Writer, mb[0].Bytes())
	}

	if cap(w.cache) < len(mb) {
		w.cache = make([][]byte, 0, len(mb))
	}

	bs := w.cache
	for _, b := range mb {
		bs = append(bs, b.Bytes())
	}

	defer func() {
		for idx := range bs {
			bs[idx] = nil
		}
	}()

	for size > 0 {
		n, err := bs.WriteTo(w.Writer)
		if err != nil {
			return err
		}
		size -= int(n)
	}

	return nil
}

// ReadFrom implements io.ReaderFrom.
/*func (w *allToBytesWriter) ReadFrom(reader io.Reader) (int64, error) {
	var sc SizeCounter
	err := Copy(w, NewReader(reader), CountSize(&sc))
	return sc.Size, err
}*/

// sequentialWriter is a Writer that writes MultiBuffer sequentially into the underlying io.Writer.
type sequentialWriter struct {
	writer io.Writer
}

func NewSequentialWriter(writer io.Writer) Writer {
	return &sequentialWriter{
		writer: writer,
	}
}

func (w *sequentialWriter) WriteMultiBuffer(mb MultiBuffer) error {
	mb, err := WriteMultiTo(w.writer, mb)
	ReleaseMulti(mb)
	return err
}

type noOpWriter struct {
}

func (noOpWriter) WriteMultiBuffer(mb MultiBuffer) error {
	ReleaseMulti(mb)
	return nil
}

func (noOpWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (noOpWriter) ReadFrom(reader io.Reader) (int64, error) {
	b := New()
	defer b.Release()

	totalBytes := int64(0)
	for {
		b.Clear()
		_, err := b.ReadFrom(reader)
		totalBytes += int64(b.Len())
		if err != nil {
			if errors.Cause(err) == io.EOF {
				return totalBytes, nil
			}
			return totalBytes, err
		}
	}
}

var (
	// Discard is a Writer that swallows all contents written in.
	Discard Writer = noOpWriter{}

	// DiscardBytes is an io.Writer that swallows all contents written in.
	DiscardBytes io.Writer = noOpWriter{}
)
