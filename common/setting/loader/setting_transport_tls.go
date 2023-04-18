package loader

import (
	"v2ray.com/core/transport/internet/tls"
)

type TLSetting struct {
	ServerName       string
	Certificate, Key string
}

func BuildTLSetting(setting TLSetting) tls.Config {
	return tls.Config{
		ServerName: setting.ServerName,
		Certificate: tls.Certificate{
			Certificate: []byte(setting.Certificate),
			Key:         []byte(setting.ServerName),
		},
	}
}
