package loader

import (
	"v2ray.com/core/common/protocol/trojan"
)

type TrojanUserSetting struct {
	Password string
}

func BuildTrojanUser(setting TrojanUserSetting) trojan.User {
	return trojan.User{
		Password: setting.Password,
	}
}
