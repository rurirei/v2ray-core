package crypto

import (
	"crypto/aes"
	"crypto/cipher"
)

// NewAesDecryptionStream creates a new AES encryption stream based on given key and IV.
// Caller must ensure the length of key and IV is either 16, 24 or 32 bytes.
func NewAesDecryptionStream(key []byte, iv []byte) (cipher.Stream, error) {
	return NewAesStreamMethod(key, iv, cipher.NewCFBDecrypter)
}

// NewAesEncryptionStream creates a new AES description stream based on given key and IV.
// Caller must ensure the length of key and IV is either 16, 24 or 32 bytes.
func NewAesEncryptionStream(key []byte, iv []byte) (cipher.Stream, error) {
	return NewAesStreamMethod(key, iv, cipher.NewCFBEncrypter)
}

func NewAesStreamMethod(key []byte, iv []byte, f func(cipher.Block, []byte) cipher.Stream) (cipher.Stream, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return f(aesBlock, iv), nil
}

// NewAesCTRStream creates a stream cipher based on AES-CTR.
func NewAesCTRStream(key []byte, iv []byte) (cipher.Stream, error) {
	return NewAesStreamMethod(key, iv, cipher.NewCTR)
}

// NewAesGcm creates a AEAD cipher based on AES-GCM.
func NewAesGcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead, nil
}
