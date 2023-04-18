package shadowsocks

import (
	"v2ray.com/core/common/antireplay"
)

type Password = string

type User struct {
	Security Security
	Password Password
	IvCheck  bool

	replayFilter antireplay.GeneralizedReplayFilter
}

func (u User) ReplayFilter() antireplay.GeneralizedReplayFilter {
	if u.IvCheck {
		if u.replayFilter == nil {
			u.replayFilter = antireplay.NewBloomRing()
		}
		return u.replayFilter
	}
	return nil
}

func (u User) CheckIV(iv []byte) error {
	replayFilter := u.ReplayFilter()

	if replayFilter == nil {
		return nil
	}
	if replayFilter.Check(iv) {
		return nil
	}
	return newError("IV is not unique")
}
