package mux

import (
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/crypto"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
)

// NewReader creates a buffer.Reader based on the transfer type of this sessionBody.
func NewReader(reader buffer.BufferedReader, transferType protocol.TransferType) Reader {
	switch transferType {
	case protocol.TransferTypeStream:
		return NewStreamReader(reader)
	case protocol.TransferTypePacket:
		return NewPacketReader(reader)
	default:
		return nil
	}
}

type Reader interface {
	buffer.Reader
}

// packetReader is an io.Reader that reads whole chunk of Mux frames every time.
type packetReader struct {
	reader buffer.BufferedReader
	eof    bool
}

// NewPacketReader creates a new packetReader.
func NewPacketReader(reader buffer.BufferedReader) Reader {
	return &packetReader{
		reader: reader,
		eof:    false,
	}
}

// ReadMultiBuffer implements buffer.Reader.
func (r *packetReader) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	if r.eof {
		return nil, io.EOF
	}

	size, err := serial.ReadUint16(r.reader)
	if err != nil {
		return nil, err
	}

	if size > buffer.Size {
		return nil, newError("packet size too large %d", size)
	}

	b := buffer.New()
	if _, err := b.ReadFullFrom(r.reader, int(size)); err != nil {
		defer b.Release()
		return nil, err
	}

	r.eof = true
	return buffer.MultiBuffer{b}, nil
}

// NewStreamReader creates a new StreamReader.
func NewStreamReader(reader buffer.BufferedReader) Reader {
	return crypto.NewChunkStreamReaderWithChunkCount(crypto.PlainChunkSizeParser{}, reader, 1)
}
