package crypto

import (
	"encoding/binary"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
)

// ChunkSizeDecoder is a utility class to decode size value from bytes.
type ChunkSizeDecoder interface {
	SizeBytes() int32
	Decode([]byte) (uint16, error)
}

// ChunkSizeEncoder is a utility class to encode size value into bytes.
type ChunkSizeEncoder interface {
	SizeBytes() int32
	Encode(uint16, []byte) []byte
}

type PaddingLengthGenerator interface {
	MaxPaddingLen() uint16
	NextPaddingLen() uint16
}

type PlainChunkSizeParser struct{}

func (PlainChunkSizeParser) SizeBytes() int32 {
	return 2
}

func (PlainChunkSizeParser) Encode(size uint16, b []byte) []byte {
	binary.BigEndian.PutUint16(b, size)
	return b[:2]
}

func (PlainChunkSizeParser) Decode(b []byte) (uint16, error) {
	return binary.BigEndian.Uint16(b), nil
}

type AEADChunkSizeParser struct {
	Auth *AEADAuthenticator
}

func (p *AEADChunkSizeParser) SizeBytes() int32 {
	return 2 + int32(p.Auth.Overhead())
}

func (p *AEADChunkSizeParser) Encode(size uint16, b []byte) []byte {
	binary.BigEndian.PutUint16(b, size-uint16(p.Auth.Overhead()))
	b, _ = p.Auth.Seal(b[:0], b[:2])
	return b
}

func (p *AEADChunkSizeParser) Decode(b []byte) (uint16, error) {
	b, err := p.Auth.Open(b[:0], b)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b) + uint16(p.Auth.Overhead()), nil
}

type ChunkStreamReader struct {
	sizeDecoder ChunkSizeDecoder
	reader      buffer.BufferedReader

	buffer       []byte
	leftOverSize int32
	maxNumChunk  uint32
	numChunk     uint32
}

func NewChunkStreamReader(sizeDecoder ChunkSizeDecoder, reader buffer.BufferedReader) *ChunkStreamReader {
	return NewChunkStreamReaderWithChunkCount(sizeDecoder, reader, 0)
}

func NewChunkStreamReaderWithChunkCount(sizeDecoder ChunkSizeDecoder, reader buffer.BufferedReader, maxNumChunk uint32) *ChunkStreamReader {
	return &ChunkStreamReader{
		sizeDecoder: sizeDecoder,
		reader:      reader,
		buffer:      make([]byte, sizeDecoder.SizeBytes()),
		maxNumChunk: maxNumChunk,
	}
}

func (r *ChunkStreamReader) readSize() (uint16, error) {
	if _, err := io.ReadFull(r.reader, r.buffer); err != nil {
		return 0, err
	}
	return r.sizeDecoder.Decode(r.buffer)
}

func (r *ChunkStreamReader) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	size := r.leftOverSize
	if size == 0 {
		r.numChunk++
		if r.maxNumChunk > 0 && r.numChunk > r.maxNumChunk {
			return nil, io.EOF
		}
		nextSize, err := r.readSize()
		if err != nil {
			return nil, err
		}
		if nextSize == 0 {
			return nil, io.EOF
		}
		size = int32(nextSize)
	}
	r.leftOverSize = size

	mb, err := r.reader.ReadAtMost(int(size))
	if mb != nil && !mb.IsEmpty() {
		r.leftOverSize -= int32(mb.Len())
		return mb, nil
	}
	return nil, err
}

type ChunkStreamWriter struct {
	sizeEncoder ChunkSizeEncoder
	writer      buffer.Writer
}

func NewChunkStreamWriter(sizeEncoder ChunkSizeEncoder, writer buffer.Writer) *ChunkStreamWriter {
	return &ChunkStreamWriter{
		sizeEncoder: sizeEncoder,
		writer:      writer,
	}
}

func (w *ChunkStreamWriter) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	const sliceSize = 8192
	mbLen := mb.Len()
	mb2Write := make(buffer.MultiBuffer, 0, mbLen/buffer.Size+mbLen/sliceSize+2)

	for {
		mb0, slice := buffer.SplitSize(mb, sliceSize)
		mb = mb0

		b := buffer.New()
		w.sizeEncoder.Encode(uint16(slice.Len()), b.Extend(int(w.sizeEncoder.SizeBytes())))
		mb2Write = append(mb2Write, b)
		mb2Write = append(mb2Write, slice...)

		if mb.IsEmpty() {
			break
		}
	}

	return w.writer.WriteMultiBuffer(mb2Write)
}
