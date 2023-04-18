package crypto

import (
	"crypto/cipher"
	"crypto/rand"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/bytespool"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/protocol"
)

type BytesGenerator func() []byte

func GenerateEmptyBytes() BytesGenerator {
	var b [1]byte
	return func() []byte {
		return b[:0]
	}
}

func GenerateStaticBytes(content []byte) BytesGenerator {
	return func() []byte {
		return content
	}
}

func GenerateIncreasingNonce(nonce []byte) BytesGenerator {
	c := append([]byte{}, nonce...)
	return func() []byte {
		for i := range c {
			c[i]++
			if c[i] != 0 {
				break
			}
		}
		return c
	}
}

func GenerateInitialAEADNonce() BytesGenerator {
	return GenerateIncreasingNonce([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
}

type Authenticator interface {
	NonceSize() int
	Overhead() int
	Open(dst, cipherText []byte) ([]byte, error)
	Seal(dst, plainText []byte) ([]byte, error)
}

type AEADAuthenticator struct {
	cipher.AEAD
	NonceGenerator          BytesGenerator
	AdditionalDataGenerator BytesGenerator
}

func (v *AEADAuthenticator) Open(dst, cipherText []byte) ([]byte, error) {
	iv := v.NonceGenerator()
	if len(iv) != v.AEAD.NonceSize() {
		return nil, newError("invalid AEAD nonce size: ", len(iv))
	}

	var additionalData []byte
	if v.AdditionalDataGenerator != nil {
		additionalData = v.AdditionalDataGenerator()
	}
	return v.AEAD.Open(dst, iv, cipherText, additionalData)
}

func (v *AEADAuthenticator) Seal(dst, plainText []byte) ([]byte, error) {
	iv := v.NonceGenerator()
	if len(iv) != v.AEAD.NonceSize() {
		return nil, newError("invalid AEAD nonce size: ", len(iv))
	}

	var additionalData []byte
	if v.AdditionalDataGenerator != nil {
		additionalData = v.AdditionalDataGenerator()
	}
	return v.AEAD.Seal(dst, iv, plainText, additionalData), nil
}

type AuthenticationReader struct {
	auth         Authenticator
	reader       buffer.BufferedReader
	sizeParser   ChunkSizeDecoder
	sizeBytes    []byte
	transferType protocol.TransferType
	padding      PaddingLengthGenerator
	size         uint16
	paddingLen   uint16
	hasSize      bool
	done         bool
}

func NewAuthenticationReader(auth Authenticator, reader buffer.BufferedReader, sizeParser ChunkSizeDecoder, transferType protocol.TransferType, paddingLen PaddingLengthGenerator) *AuthenticationReader {
	return &AuthenticationReader{
		auth:         auth,
		reader:       reader,
		sizeParser:   sizeParser,
		transferType: transferType,
		padding:      paddingLen,
		sizeBytes:    make([]byte, sizeParser.SizeBytes()),
	}
}

func (r *AuthenticationReader) readSize() (uint16, uint16, error) {
	if r.hasSize {
		r.hasSize = false
		return r.size, r.paddingLen, nil
	}
	if _, err := io.ReadFull(r.reader, r.sizeBytes); err != nil {
		return 0, 0, err
	}
	var padding uint16
	if r.padding != nil {
		padding = r.padding.NextPaddingLen()
	}
	size, err := r.sizeParser.Decode(r.sizeBytes)
	return size, padding, err
}

var errSoft = newError("waiting for more data")

func (r *AuthenticationReader) readBuffer(size int, padding int32) (*buffer.Buffer, error) {
	b := buffer.New()

	if _, err := b.ReadFullFrom(r.reader, size); err != nil {
		defer b.Release()
		return nil, err
	}

	size -= int(padding)
	rb, err := r.auth.Open(b.BytesTo(0), b.BytesTo(size))
	if err != nil {
		defer b.Release()
		return nil, err
	}

	b.Resize(0, len(rb))

	return b, nil
}

func (r *AuthenticationReader) readInternal(soft bool, mb buffer.MultiBuffer) (buffer.MultiBuffer, error) {
	if soft && int32(r.reader.BufferedBytes()) < r.sizeParser.SizeBytes() {
		return mb, errSoft
	}

	if r.done {
		return mb, io.EOF
	}

	size, padding, err := r.readSize()
	if err != nil {
		return mb, err
	}

	if size == uint16(r.auth.Overhead())+padding {
		r.done = true
		return mb, io.EOF
	}

	if soft && size > uint16(r.reader.BufferedBytes()) {
		r.size = size
		r.paddingLen = padding
		r.hasSize = true
		return mb, errSoft
	}

	if size <= buffer.Size {
		b, err := r.readBuffer(int(size), int32(padding))
		if err != nil {
			return mb, nil
		}
		mb = append(mb, b)
		return mb, nil
	}

	payload := bytespool.Alloc(int(size))
	defer bytespool.Free(payload)

	if _, err := io.ReadFull(r.reader, payload[:size]); err != nil {
		return mb, err
	}

	size -= padding

	rb, err := r.auth.Open(payload[:0], payload[:size])
	if err != nil {
		return mb, err
	}

	mb = buffer.MergeBytes(mb, rb)
	return mb, nil
}

func (r *AuthenticationReader) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	const readSize = 16
	mb := make(buffer.MultiBuffer, 0, readSize)

	mb, err := r.readInternal(false, mb)
	if err != nil {
		buffer.ReleaseMulti(mb)
		return nil, err
	}

	for i := 1; i < readSize; i++ {
		mb, err = r.readInternal(true, mb)
		if err == errSoft || err == io.EOF {
			break
		}
		if err != nil {
			buffer.ReleaseMulti(mb)
			return nil, err
		}
	}

	return mb, nil
}

type AuthenticationWriter struct {
	auth         Authenticator
	writer       buffer.Writer
	sizeParser   ChunkSizeEncoder
	transferType protocol.TransferType
	padding      PaddingLengthGenerator
}

func NewAuthenticationWriter(auth Authenticator, sizeParser ChunkSizeEncoder, writer buffer.Writer, transferType protocol.TransferType, padding PaddingLengthGenerator) *AuthenticationWriter {
	w := &AuthenticationWriter{
		auth:         auth,
		writer:       writer,
		sizeParser:   sizeParser,
		transferType: transferType,
	}
	if padding != nil {
		w.padding = padding
	}
	return w
}

func (w *AuthenticationWriter) seal(b []byte) (*buffer.Buffer, error) {
	encryptedSize := int32(len(b) + w.auth.Overhead())
	var paddingSize int32
	if w.padding != nil {
		paddingSize = int32(w.padding.NextPaddingLen())
	}

	sizeBytes := w.sizeParser.SizeBytes()
	totalSize := sizeBytes + encryptedSize + paddingSize
	if totalSize > buffer.Size {
		return nil, newError("size too large: ", totalSize)
	}

	eb := buffer.New()
	w.sizeParser.Encode(uint16(encryptedSize+paddingSize), eb.Extend(int(sizeBytes)))
	if _, err := w.auth.Seal(eb.Extend(int(encryptedSize))[:0], b); err != nil {
		defer eb.Release()
		return nil, err
	}
	if paddingSize > 0 {
		// These paddings will send in clear text.
		// To avoid leakage of PRNG internal state, a cryptographically Secure PRNG should be used.
		// With size of the chunk and padding length encrypted, the content of padding doesn't matter much.
		paddingBytes := eb.Extend(int(paddingSize))
		_, _ = rand.Read(paddingBytes)
	}

	return eb, nil
}

func (w *AuthenticationWriter) writeStream(mb buffer.MultiBuffer) error {
	defer buffer.ReleaseMulti(mb)

	var maxPadding int32
	if w.padding != nil {
		maxPadding = int32(w.padding.MaxPaddingLen())
	}

	payloadSize := buffer.Size - int32(w.auth.Overhead()) - w.sizeParser.SizeBytes() - maxPadding
	if len(mb)+10 > 64*1024*1024 {
		return newError("value too large")
	}
	sliceSize := len(mb) + 10
	mb2Write := make(buffer.MultiBuffer, 0, sliceSize)

	temp := buffer.New()
	defer temp.Release()

	rawBytes := temp.Extend(int(payloadSize))

	for {
		nb, nBytes := buffer.SplitBytes(mb, rawBytes)
		mb = nb

		eb, err := w.seal(rawBytes[:nBytes])
		if err != nil {
			buffer.ReleaseMulti(mb2Write)
			return err
		}
		mb2Write = append(mb2Write, eb)
		if mb.IsEmpty() {
			break
		}
	}

	return w.writer.WriteMultiBuffer(mb2Write)
}

func (w *AuthenticationWriter) writePacket(mb buffer.MultiBuffer) error {
	defer buffer.ReleaseMulti(mb)

	if len(mb)+1 > 64*1024*1024 {
		return newError("value too large")
	}
	sliceSize := len(mb) + 1
	mb2Write := make(buffer.MultiBuffer, 0, sliceSize)

	for _, b := range mb {
		if b.IsEmpty() {
			continue
		}

		eb, err := w.seal(b.Bytes())
		if err != nil {
			continue
		}

		mb2Write = append(mb2Write, eb)
	}

	if mb2Write.IsEmpty() {
		return nil
	}

	return w.writer.WriteMultiBuffer(mb2Write)
}

// WriteMultiBuffer implements buf.Writer.
func (w *AuthenticationWriter) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	if mb.IsEmpty() {
		eb, _ := w.seal([]byte{})
		return w.writer.WriteMultiBuffer(buffer.MultiBuffer{eb})
	}

	if w.transferType == protocol.TransferTypeStream {
		return w.writeStream(mb)
	}

	return w.writePacket(mb)
}
