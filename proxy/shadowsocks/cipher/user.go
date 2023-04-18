package cipher

import (
	"crypto/md5"

	"v2ray.com/core/common"
	"v2ray.com/core/common/crypto"
	"v2ray.com/core/common/protocol/shadowsocks"
)

type Key = []byte

type User struct {
	User   shadowsocks.User
	Key    Key
	Cipher Cipher
}

func BuildUser(user shadowsocks.User) (User, error) {
	cipher, err := securityCipher(user.Security)
	if err != nil {
		return User{}, err
	}

	key := passwordKey([]byte(user.Password), cipher.KeySize())

	return User{
		User:   user,
		Key:    key,
		Cipher: cipher,
	}, nil
}

func securityCipher(c shadowsocks.Security) (Cipher, error) {
	switch c {
	case shadowsocks.Security_AES_128_GCM:
		return &aeadCipher{
			KeyBytes:        16,
			IVBytes:         16,
			AEADAuthCreator: crypto.NewAesGcm,
		}, nil
	case shadowsocks.Security_AES_256_GCM:
		return &aeadCipher{
			KeyBytes:        32,
			IVBytes:         32,
			AEADAuthCreator: crypto.NewAesGcm,
		}, nil
	case shadowsocks.Security_CHACHA20_POLY1305:
		return &aeadCipher{
			KeyBytes:        32,
			IVBytes:         32,
			AEADAuthCreator: crypto.NewChaCha20Poly1305,
		}, nil
	case shadowsocks.Security_NONE:
		return noneCipher{}, nil
	default:
		return nil, common.ErrUnknownNetwork
	}
}

func passwordKey(password []byte, keySize int32) []byte {
	key := make([]byte, 0, keySize)

	md5Sum := md5.Sum(password)
	key = append(key, md5Sum[:]...)

	for int32(len(key)) < keySize {
		md5Hash := md5.New()
		md5Hash.Write(md5Sum[:])
		md5Hash.Write(password)
		md5Hash.Sum(md5Sum[:0])

		key = append(key, md5Sum[:]...)
	}
	return key
}
