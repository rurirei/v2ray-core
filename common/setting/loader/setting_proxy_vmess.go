package loader

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/protocol/vmess"
	"v2ray.com/core/common/uuid"
)

type VmessUserSetting struct {
	Security string
	UUID     string
}

func BuildVmessUser(setting VmessUserSetting) (vmess.User, error) {
	security, err := ParseVmessUserSecurity(setting.Security)
	if err != nil {
		return vmess.User{}, err
	}

	id, err := ParseVmessUserUUID(setting.UUID)
	if err != nil {
		return vmess.User{}, err
	}

	return vmess.User{
		Security: security,
		ID:       id,
	}, nil
}

func ParseVmessUserUUID(s string) (vmess.ID, error) {
	id, err := uuid.ParseString(s)
	if err != nil {
		return vmess.ID{}, err
	}

	uid, err := vmess.NewID(id)
	if err != nil {
		return vmess.ID{}, err
	}
	return uid, nil
}

const (
	Vmess_Security_UNKNOWN           = "unknown"
	Vmess_Security_LEGACY            = "legacy"
	Vmess_Security_AUTO              = "auto"
	Vmess_Security_AES128_GCM        = "aes_128_gcm"
	Vmess_Security_CHACHA20_POLY1305 = "chacha20_poly1305"
	Vmess_Security_NONE              = "none"
	Vmess_Security_ZERO              = "zero"
)

func ParseVmessUserSecurity(s string) (vmess.Security, error) {
	switch s {
	case Vmess_Security_UNKNOWN:
		return vmess.Security_UNKNOWN, nil
	case Vmess_Security_LEGACY:
		return vmess.Security_LEGACY, nil
	case Vmess_Security_AUTO:
		return vmess.AutoSecurity(), nil
	case Vmess_Security_AES128_GCM:
		return vmess.Security_AES_128_GCM, nil
	case Vmess_Security_CHACHA20_POLY1305:
		return vmess.Security_CHACHA20_POLY1305, nil
	case Vmess_Security_NONE:
		return vmess.Security_NONE, nil
	case Vmess_Security_ZERO:
		return vmess.Security_ZERO, nil
	default:
		return vmess.Security_UNKNOWN, common.ErrUnknownNetwork
	}
}
