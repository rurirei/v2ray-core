package dns

import (
	"v2ray.com/core/common/net"
)

type ConditionProvider struct {
	Provider

	Tag string
}

type Provider interface {
	LookupIP(string, string) ([]net.IP, error)
	Name() string
}
