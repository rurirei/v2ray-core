package validator

import (
	"crypto/hmac"
	"crypto/sha256"
	"hash/crc64"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"v2ray.com/core/common/dice"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/vmess"
	"v2ray.com/core/common/serial"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy/vmess/aead"
)

var (
	errNotFound = newError("Not Found")
	errTainted  = newError("ErrTainted")
)

const (
	updateInterval   = 10 * time.Second
	cacheDurationSec = 120
)

type user struct {
	user    vmess.User
	lastSec protocol.Timestamp
}

// TimedUserValidator is a user Validator based on time.
type TimedUserValidator struct {
	sync.RWMutex

	users    []*user
	userHash map[[16]byte]indexTimePair
	hasher   vmess.IDHash
	baseTime protocol.Timestamp
	task     task.Timer

	behaviorSeed  uint64
	behaviorFused bool

	aeadDecoderHolder *aead.AuthIDDecoderHolder

	legacyWarnShown bool
}

type indexTimePair struct {
	user    *user
	timeInc uint32

	taintedFuse *uint32
}

// NewTimedUserValidator creates a new TimedUserValidator.
func NewTimedUserValidator(hasher vmess.IDHash) *TimedUserValidator {
	tuv := &TimedUserValidator{
		users:             make([]*user, 0, 16),
		userHash:          make(map[[16]byte]indexTimePair, 1024),
		hasher:            hasher,
		baseTime:          protocol.Timestamp(time.Now().Unix() - cacheDurationSec*2),
		task:              task.NewTimer(),
		aeadDecoderHolder: aead.NewAuthIDDecoderHolder(),
	}

	go tuv.task.Period(tuv.updateUserHash, updateInterval)

	return tuv
}

func (v *TimedUserValidator) generateNewHashes(nowSec protocol.Timestamp, user *user) {
	var hashValue [16]byte
	genEndSec := nowSec + cacheDurationSec
	genHashForID := func(id vmess.ID) {
		idHash := v.hasher(id.Bytes())
		genBeginSec := user.lastSec
		if genBeginSec < nowSec-cacheDurationSec {
			genBeginSec = nowSec - cacheDurationSec
		}
		for ts := genBeginSec; ts <= genEndSec; ts++ {
			_, _ = serial.WriteUint64(idHash, uint64(ts))
			idHash.Sum(hashValue[:0])
			idHash.Reset()

			v.userHash[hashValue] = indexTimePair{
				user:        user,
				timeInc:     uint32(ts - v.baseTime),
				taintedFuse: new(uint32),
			}
		}
	}

	genHashForID(user.user.ID)
	for _, id := range user.user.AlterIDs {
		genHashForID(id)
	}
	user.lastSec = genEndSec
}

func (v *TimedUserValidator) removeExpiredHashes(expire uint32) {
	for key, pair := range v.userHash {
		if pair.timeInc < expire {
			delete(v.userHash, key)
		}
	}
}

func (v *TimedUserValidator) updateUserHash() {
	now := time.Now()
	nowSec := protocol.Timestamp(now.Unix())

	v.Lock()
	defer v.Unlock()

	for _, user := range v.users {
		v.generateNewHashes(nowSec, user)
	}

	expire := protocol.Timestamp(now.Unix() - cacheDurationSec)
	if expire > v.baseTime {
		v.removeExpiredHashes(uint32(expire - v.baseTime))
	}
}

func (v *TimedUserValidator) Add(u vmess.User) error {
	v.Lock()
	defer v.Unlock()

	nowSec := time.Now().Unix()

	uu := &user{
		user:    u,
		lastSec: protocol.Timestamp(nowSec - cacheDurationSec),
	}
	v.users = append(v.users, uu)
	v.generateNewHashes(protocol.Timestamp(nowSec), uu)

	if !v.behaviorFused {
		hashkdf := hmac.New(sha256.New, []byte("VMESSBSKDF"))
		hashkdf.Write(uu.user.ID.Bytes())
		v.behaviorSeed = crc64.Update(v.behaviorSeed, crc64.MakeTable(crc64.ECMA), hashkdf.Sum(nil))
	}

	var cmdkeyfl [16]byte
	copy(cmdkeyfl[:], uu.user.ID.CmdKey())
	v.aeadDecoderHolder.AddUser(cmdkeyfl, u)

	return nil
}

func (v *TimedUserValidator) Get(userHash []byte) (vmess.User, protocol.Timestamp, bool, error) {
	v.RLock()
	defer v.RUnlock()

	v.behaviorFused = true

	var fixedSizeHash [16]byte
	copy(fixedSizeHash[:], userHash)
	pair, found := v.userHash[fixedSizeHash]
	if found {
		uu := pair.user.user
		if atomic.LoadUint32(pair.taintedFuse) == 0 {
			return uu, protocol.Timestamp(pair.timeInc) + v.baseTime, true, nil
		}
		return vmess.User{}, 0, false, errTainted
	}
	return vmess.User{}, 0, false, errNotFound
}

func (v *TimedUserValidator) GetAEAD(userHash []byte) (vmess.User, bool, error) {
	v.RLock()
	defer v.RUnlock()

	var userHashFL [16]byte
	copy(userHashFL[:], userHash)

	userd, err := v.aeadDecoderHolder.Match(userHashFL)
	if err != nil {
		return vmess.User{}, false, err
	}
	return userd.(vmess.User), true, err
}

func (v *TimedUserValidator) Delete(email string) bool {
	v.Lock()
	defer v.Unlock()

	email = strings.ToLower(email)
	idx := -1
	for i, u := range v.users {
		if strings.EqualFold(u.user.Email(), email) {
			idx = i
			var cmdkeyfl [16]byte
			copy(cmdkeyfl[:], u.user.ID.CmdKey())
			v.aeadDecoderHolder.RemoveUser(cmdkeyfl)
			break
		}
	}
	if idx == -1 {
		return false
	}
	ulen := len(v.users)

	v.users[idx] = v.users[ulen-1]
	v.users[ulen-1] = nil
	v.users = v.users[:ulen-1]

	return true
}

// Close implements common.Closable.
func (v *TimedUserValidator) Close() error {
	return v.task.Close()
}

func (v *TimedUserValidator) GetBehaviorSeed() uint64 {
	v.Lock()
	defer v.Unlock()

	v.behaviorFused = true
	if v.behaviorSeed == 0 {
		v.behaviorSeed = dice.RollUint64()
	}
	return v.behaviorSeed
}

func (v *TimedUserValidator) BurnTaintFuse(userHash []byte) error {
	v.RLock()
	defer v.RUnlock()

	var userHashFL [16]byte
	copy(userHashFL[:], userHash)

	pair, found := v.userHash[userHashFL]
	if found {
		if atomic.CompareAndSwapUint32(pair.taintedFuse, 0, 1) {
			return nil
		}
		return errTainted
	}
	return errNotFound
}

// ShouldShowLegacyWarn will return whether a Legacy Warning should be shown
func (v *TimedUserValidator) ShouldShowLegacyWarn() bool {
	if v.legacyWarnShown {
		return false
	}
	v.legacyWarnShown = true
	return true
}

//go:generate go run v2ray.com/core/common/errors/errorgen
