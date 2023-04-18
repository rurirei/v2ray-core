package antireplay

import (
	ss_bloomring "v2ray.com/core/common/antireplay/bloomring"
)

const (
	DefaultSFCapacity = 1e6
	// DefaultSFFPR FalsePositiveRate
	DefaultSFFPR  = 1e-6
	DefaultSFSlot = 10
)

type BloomRing struct {
	*ss_bloomring.BloomRing
}

func (b BloomRing) Interval() int64 {
	return 9999999
}

func (b BloomRing) Check(sum []byte) bool {
	if b.Test(sum) {
		return false
	}
	b.Add(sum)
	return true
}

func NewBloomRing() BloomRing {
	return BloomRing{
		BloomRing: ss_bloomring.NewBloomRing(DefaultSFSlot, DefaultSFCapacity, DefaultSFFPR),
	}
}
