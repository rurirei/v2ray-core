package tls

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

type DialSetting struct {
	Config Config
}

func Dial(setting DialSetting, dialTCPFunc internet.DialTCPFunc) internet.DialTCPFunc {
	return func(src, dst net.Address) (net.Conn, error) {
		c, err := dialTCPFunc(src, dst)
		if err != nil {
			return nil, err
		}

		return Client(c, setting.Config)
	}
}
