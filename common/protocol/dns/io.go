package dns

import (
	"encoding/binary"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/serial"
)

const (
	msgMaxSize = 2048
)

type MessageWriter interface {
	WriteMessage(*buffer.Buffer) error
}

type udpWriter struct {
	writer buffer.Writer
}

func NewUDPWriter(writer buffer.Writer) MessageWriter {
	return &udpWriter{
		writer: writer,
	}
}

func (w *udpWriter) WriteMessage(b *buffer.Buffer) error {
	return w.writer.WriteMultiBuffer(buffer.MultiBuffer{b})
}

type tcpWriter struct {
	writer buffer.Writer
}

func NewTCPWriter(writer buffer.Writer) MessageWriter {
	return &tcpWriter{
		writer: writer,
	}
}

func (w *tcpWriter) WriteMessage(b *buffer.Buffer) error {
	mb := make(buffer.MultiBuffer, 0, 2)
	size := buffer.New()
	binary.BigEndian.PutUint16(size.Extend(2), uint16(b.Len()))
	mb = append(mb, size, b)
	return w.writer.WriteMultiBuffer(mb)
}

type MessageReader interface {
	ReadMessage() (*buffer.Buffer, error)
}

type udpReader struct {
	reader buffer.Reader

	cache buffer.MultiBuffer
}

func NewUDPReader(reader buffer.Reader) MessageReader {
	return &udpReader{
		reader: reader,
	}
}

func (r *udpReader) ReadMessage() (*buffer.Buffer, error) {
	for {
		b := r.readCache()
		if b != nil {
			return b, nil
		}
		if err := r.refill(); err != nil {
			return nil, err
		}
	}
}

func (r *udpReader) readCache() *buffer.Buffer {
	mb, b := buffer.SplitFirst(r.cache)
	r.cache = mb
	return b
}

func (r *udpReader) refill() error {
	mb, err := r.reader.ReadMultiBuffer()
	if err != nil {
		return err
	}

	r.cache = mb
	return nil
}

type tcpReader struct {
	reader buffer.BufferedReader
}

func NewTCPReader(reader buffer.BufferedReader) MessageReader {
	return &tcpReader{
		reader: reader,
	}
}

func (r *tcpReader) ReadMessage() (*buffer.Buffer, error) {
	size, err := serial.ReadUint16(r.reader)
	if err != nil {
		return nil, err
	}
	if size > msgMaxSize {
		return nil, newError("message size too large: %d", size)
	}

	buf := buffer.New()
	if _, err := buf.ReadFullFrom(r.reader, int(size)); err != nil {
		defer buf.Release()
		return nil, err
	}
	return buf, nil
}
