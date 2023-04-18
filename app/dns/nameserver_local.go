package dns

import (
	"v2ray.com/core/common/net"
)

type LocalDNSetting struct {
	TTL int64
}

type localDNS struct {
	lookupIPFuncDNS Provider
}

func NewLocalDNS(setting LocalDNSetting) Provider {
	p := &localDNS{}

	p.lookupIPFuncDNS = NewLookupIPFuncDNS(newBaseRecordPool(), LookupIPFuncDNSetting{
		LookupIPFunc: p.lookupIP,
		TTL:          setting.TTL,
	})

	return p
}

func (p *localDNS) LookupIP(network, host string) ([]net.IP, error) {
	return p.lookupIPFuncDNS.LookupIP(network, host)
}

func (p *localDNS) lookupIP(network, host string) ([]net.IP, error) {
	return net.LocalLookupIPFunc(network, host)
}

func (p *localDNS) Name() string {
	return "local DNS"
}
