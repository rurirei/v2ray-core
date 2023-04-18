package cipher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"v2ray.com/core/common/protocol/trojan"
)

type Key = []byte

type User struct {
	User trojan.User
	Key  Key
}

func BuildUser(user trojan.User) User {
	key := hexSha224(user.Password)

	return User{
		User: user,
		Key:  key,
	}
}

func hexSha224(password string) []byte {
	buf := make([]byte, 56)
	hash := sha256.New224()
	_, _ = hash.Write([]byte(password))
	hex.Encode(buf, hash.Sum(nil))
	return buf
}

func hexString(data []byte) string {
	str := ""
	for _, v := range data {
		str += fmt.Sprintf("%02x", v)
	}
	return str
}
