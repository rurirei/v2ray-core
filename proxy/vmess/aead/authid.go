package aead

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	rand3 "crypto/rand"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"time"

	"v2ray.com/core/common/antireplay"
	"v2ray.com/core/common/io"
)

var (
	ErrNotFound = errors.New("user do not exist")
	ErrReplay   = errors.New("replayed request")
)

func CreateAuthID(cmdKey []byte, time int64) ([16]byte, error) {
	buf := bytes.NewBuffer(nil)
	_ = binary.Write(buf, binary.BigEndian, time)
	var zero uint32
	_, _ = io.Copy(buf, io.LimitReader(rand3.Reader, 4))
	zero = crc32.ChecksumIEEE(buf.Bytes())
	_ = binary.Write(buf, binary.BigEndian, zero)
	aesBlock, err := NewCipherFromKey(cmdKey)
	if err != nil {
		return [16]byte{}, err
	}
	if buf.Len() != 16 {
		return [16]byte{}, nil
	}
	var result [16]byte
	aesBlock.Encrypt(result[:], buf.Bytes())
	return result, nil
}

func NewCipherFromKey(cmdKey []byte) (cipher.Block, error) {
	aesBlock, err := aes.NewCipher(KDF16(cmdKey, KDFSaltConstAuthIDEncryptionKey))
	if err != nil {
		return nil, err
	}
	return aesBlock, nil
}

type AuthIDDecoder struct {
	s cipher.Block
}

func NewAuthIDDecoder(cmdKey []byte) *AuthIDDecoder {
	return &AuthIDDecoder{
		s: func() cipher.Block {
			s, _ := NewCipherFromKey(cmdKey)
			return s
		}(),
	}
}

func (aidd *AuthIDDecoder) Decode(data [16]byte) (int64, uint32, int32, []byte, error) {
	aidd.s.Decrypt(data[:], data[:])
	var t int64
	var zero uint32
	var rand int32
	reader := bytes.NewReader(data[:])
	_ = binary.Read(reader, binary.BigEndian, &t)
	_ = binary.Read(reader, binary.BigEndian, &rand)
	_ = binary.Read(reader, binary.BigEndian, &zero)
	return t, zero, rand, data[:], nil
}

func NewAuthIDDecoderHolder() *AuthIDDecoderHolder {
	return &AuthIDDecoderHolder{make(map[string]*AuthIDDecoderItem), antireplay.NewReplayFilter(120)}
}

type AuthIDDecoderHolder struct {
	decoders map[string]*AuthIDDecoderItem
	filter   *antireplay.ReplayFilter
}

type AuthIDDecoderItem struct {
	dec    *AuthIDDecoder
	ticket interface{}
}

func NewAuthIDDecoderItem(key [16]byte, ticket interface{}) *AuthIDDecoderItem {
	return &AuthIDDecoderItem{
		dec:    NewAuthIDDecoder(key[:]),
		ticket: ticket,
	}
}

func (a *AuthIDDecoderHolder) AddUser(key [16]byte, ticket interface{}) {
	a.decoders[string(key[:])] = NewAuthIDDecoderItem(key, ticket)
}

func (a *AuthIDDecoderHolder) RemoveUser(key [16]byte) {
	delete(a.decoders, string(key[:]))
}

func (a *AuthIDDecoderHolder) Match(authID [16]byte) (interface{}, error) {
	for _, v := range a.decoders {
		t, z, _, d, err := v.dec.Decode(authID)
		if err != nil {
			return nil, err
		}
		if z != crc32.ChecksumIEEE(d[:12]) {
			continue
		}

		if t < 0 {
			continue
		}

		if math.Abs(math.Abs(float64(t))-float64(time.Now().Unix())) > 120 {
			continue
		}

		if !a.filter.Check(authID[:]) {
			return nil, ErrReplay
		}

		return v.ticket, nil
	}
	return nil, ErrNotFound
}
