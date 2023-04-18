package crypto

import (
	"crypto/cipher"

	"golang.org/x/crypto/chacha20poly1305"

	"v2ray.com/core/common/crypto/internal"
)

// NewChaCha20Stream creates a new Chacha20 encryption/descryption stream based on give key and IV.
// Caller must ensure the length of key is 32 bytes, and length of IV is either 8 or 12 bytes.
func NewChaCha20Stream(key []byte, iv []byte) cipher.Stream {
	return internal.NewChaCha20Stream(key, iv, 20)
}

func NewChaCha20Poly1305(key []byte) (cipher.AEAD, error) {
	return chacha20poly1305.New(key)
}
