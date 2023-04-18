package vmess

import (
	"crypto/hmac"
	"crypto/md5"
	"hash"

	"v2ray.com/core/common/uuid"
)

const (
	IDBytesLen = 16
)

type IDHash func(key []byte) hash.Hash

func DefaultIDHash(key []byte) hash.Hash {
	return hmac.New(md5.New, key)
}

// The ID of en entity, in the form of a UUID.
type ID struct {
	uuid   uuid.UUID
	cmdKey [IDBytesLen]byte
}

// Equals returns true if this ID equals to the other one.
func (id ID) Equals(another ID) bool {
	return id.uuid.Equals(another.uuid)
}

func (id ID) Bytes() []byte {
	return id.uuid.Bytes()
}

func (id ID) String() string {
	return id.uuid.String()
}

func (id ID) UUID() uuid.UUID {
	return id.uuid
}

func (id ID) CmdKey() []byte {
	return id.cmdKey[:]
}

// NewID returns an ID with given UUID.
func NewID(uid uuid.UUID) (ID, error) {
	id := ID{uuid: uid}
	md5hash := md5.New()
	_, _ = md5hash.Write(uid.Bytes())
	_, _ = md5hash.Write([]byte("c48619fe-8f02-49e0-b9e9-edf763e17e21"))
	md5hash.Sum(id.cmdKey[:0])
	return id, nil
}

func nextID(uid uuid.UUID) (uuid.UUID, error) {
	md5hash := md5.New()
	_, _ = md5hash.Write(uid.Bytes())
	_, _ = md5hash.Write([]byte("16167dc8-16b6-4e6d-b8bb-65dd68113a81"))

	var newID uuid.UUID
	for {
		md5hash.Sum(newID[:0])
		if !newID.Equals(uid) {
			return newID, nil
		}
		_, _ = md5hash.Write([]byte("533eff8a-4113-4b10-b5ce-0f5d76b98cd2"))
	}
}

func NewAlterIDs(primary ID, alterIDCount uint16) ([]ID, error) {
	alterIDs := make([]ID, alterIDCount)
	prevID := primary.UUID()
	for idx := range alterIDs {
		newID, err := nextID(prevID)
		if err != nil {
			return nil, err
		}
		newID2, err := nextID(newID)
		if err != nil {
			return nil, err
		}
		newID2ID, err := NewID(newID2)
		if err != nil {
			return nil, err
		}
		alterIDs[idx] = newID2ID
		prevID = newID
	}
	return alterIDs, nil
}
