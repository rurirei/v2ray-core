package loader

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/protocol/shadowsocks"
)

type ShadowsocksUserSetting struct {
	Security string
	Password string
	IvCheck  bool
}

func BuildShadowsocksUser(setting ShadowsocksUserSetting) (shadowsocks.User, error) {
	security, err := ParseShadowsocksUserSecurity(setting.Security)
	if err != nil {
		return shadowsocks.User{}, err
	}

	return shadowsocks.User{
		Password: setting.Password,
		Security: security,
		IvCheck:  setting.IvCheck,
	}, nil
}

const (
	Shadowsocks_Security_UNKNOWN           = "unknown"
	Shadowsocks_Security_AES_128_GCM       = "aes_128_gcm"
	Shadowsocks_Security_AES_256_GCM       = "aes_256_gcm"
	Shadowsocks_Security_CHACHA20_POLY1305 = "chacha20_poly1305"
	Shadowsocks_Security_NONE              = "none"
)

func ParseShadowsocksUserSecurity(s string) (shadowsocks.Security, error) {
	switch s {
	case Shadowsocks_Security_UNKNOWN:
		return shadowsocks.Security_UNKNOWN, nil
	case Shadowsocks_Security_AES_128_GCM:
		return shadowsocks.Security_AES_128_GCM, nil
	case Shadowsocks_Security_AES_256_GCM:
		return shadowsocks.Security_AES_256_GCM, nil
	case Shadowsocks_Security_CHACHA20_POLY1305:
		return shadowsocks.Security_CHACHA20_POLY1305, nil
	case Shadowsocks_Security_NONE:
		return shadowsocks.Security_NONE, nil
	default:
		return shadowsocks.Security_UNKNOWN, common.ErrUnknownNetwork
	}
}
