package cipher

import (
	"crypto/cipher"
	"crypto/sha1"

	"golang.org/x/crypto/hkdf"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/crypto"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/protocol"
)

// Cipher is an interface for all Shadowsocks ciphers.
type Cipher interface {
	KeySize() int32
	IVSize() int32
	NewEncryptionWriter(key []byte, iv []byte, writer buffer.Writer) (buffer.Writer, error)
	NewDecryptionReader(key []byte, iv []byte, reader buffer.BufferedReader) (buffer.Reader, error)
	IsAEAD() bool
	EncodePacket(key []byte, b *buffer.Buffer) error
	DecodePacket(key []byte, b *buffer.Buffer) error
}

type aeadCipher struct {
	KeyBytes        int32
	IVBytes         int32
	AEADAuthCreator func(key []byte) (cipher.AEAD, error)
}

func (*aeadCipher) IsAEAD() bool {
	return true
}

func (c *aeadCipher) KeySize() int32 {
	return c.KeyBytes
}

func (c *aeadCipher) IVSize() int32 {
	return c.IVBytes
}

func (c *aeadCipher) createAuthenticator(key []byte, iv []byte) *crypto.AEADAuthenticator {
	nonce := crypto.GenerateInitialAEADNonce()
	subkey := make([]byte, c.KeyBytes)
	hkdfSHA1(key, iv, subkey)
	return &crypto.AEADAuthenticator{
		AEAD: func() cipher.AEAD {
			aead, _ := c.AEADAuthCreator(subkey)
			return aead
		}(),
		NonceGenerator: nonce,
	}
}

func (c *aeadCipher) NewEncryptionWriter(key []byte, iv []byte, writer buffer.Writer) (buffer.Writer, error) {
	auth := c.createAuthenticator(key, iv)
	return crypto.NewAuthenticationWriter(auth, &crypto.AEADChunkSizeParser{
		Auth: auth,
	}, writer, protocol.TransferTypeStream, nil), nil
}

func (c *aeadCipher) NewDecryptionReader(key []byte, iv []byte, reader buffer.BufferedReader) (buffer.Reader, error) {
	auth := c.createAuthenticator(key, iv)
	return crypto.NewAuthenticationReader(auth, reader, &crypto.AEADChunkSizeParser{
		Auth: auth,
	}, protocol.TransferTypeStream, nil), nil
}

func (c *aeadCipher) EncodePacket(key []byte, b *buffer.Buffer) error {
	ivLen := int(c.IVBytes)
	payloadLen := b.Len()
	auth := c.createAuthenticator(key, b.BytesTo(ivLen))

	b.Extend(auth.Overhead())
	_, err := auth.Seal(b.BytesTo(ivLen), b.BytesRange(ivLen, payloadLen))
	return err
}

func (c *aeadCipher) DecodePacket(key []byte, b *buffer.Buffer) error {
	if int32(b.Len()) <= c.IVBytes {
		return newError("insufficient data")
	}
	ivLen := int(c.IVBytes)
	payloadLen := b.Len()
	auth := c.createAuthenticator(key, b.BytesTo(ivLen))

	bbb, err := auth.Open(b.BytesTo(ivLen), b.BytesRange(ivLen, payloadLen))
	if err != nil {
		return err
	}
	b.Resize(ivLen, len(bbb))
	return nil
}

type noneCipher struct{}

func (noneCipher) KeySize() int32 {
	return 0
}

func (noneCipher) IVSize() int32 {
	return 0
}

func (noneCipher) IsAEAD() bool {
	return false
}

func (noneCipher) NewDecryptionReader(_ []byte, _ []byte, reader buffer.BufferedReader) (buffer.Reader, error) {
	return reader, nil
}

func (noneCipher) NewEncryptionWriter(_ []byte, _ []byte, writer buffer.Writer) (buffer.Writer, error) {
	return writer, nil
}

func (noneCipher) EncodePacket(_ []byte, _ *buffer.Buffer) error {
	return nil
}

func (noneCipher) DecodePacket(_ []byte, _ *buffer.Buffer) error {
	return nil
}

func hkdfSHA1(secret, salt, outKey []byte) {
	r := hkdf.New(sha1.New, secret, salt, []byte("ss-subkey"))
	_, _ = io.ReadFull(r, outKey)
}

//go:generate go run v2ray.com/core/common/errors/errorgen
