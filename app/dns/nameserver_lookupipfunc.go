package dns

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/dns"
)

type LookupIPFuncDNSetting struct {
	LookupIPFunc net.LookupIPFunc
	TTL          int64
}

type lookupIPFuncDNS struct {
	lookupIPFunc net.LookupIPFunc
	ttl          int64

	pool *baseRecordPool
}

func NewLookupIPFuncDNS(pool *baseRecordPool, setting LookupIPFuncDNSetting) Provider {
	return &lookupIPFuncDNS{
		lookupIPFunc: setting.LookupIPFunc,
		ttl:          setting.TTL,
		pool:         pool,
	}
}

func (p *lookupIPFuncDNS) LookupIP(network, host string) ([]net.IP, error) {
	fqdn := dns.GetFqdn(host)

	if ips, err := p.pool.get(network, fqdn); err == nil {
		return ips, nil
	}

	ips, err := p.lookupIPFunc(network, host)
	if err != nil {
		return nil, err
	}
	if err := p.updateSet(fqdn, ips); err != nil {
		return nil, err
	}

	return p.pool.get(network, fqdn)
}

func (p *lookupIPFuncDNS) updateSet(fqdn string, ips []net.IP) error {
	ips6 := make([]net.IP, 0, len(ips))
	ips4 := make([]net.IP, 0, len(ips))

	for _, ip := range ips {
		switch len(ip) {
		case net.IPv6len:
			ips6 = append(ips6, ip)
		case net.IPv4len:
			ips4 = append(ips4, ip)
		default:
			return common.ErrUnknownNetwork
		}
	}

	p.pool.set(fqdn, ipRecordComp{
		ip6: ipRecord{
			ttl: p.ttl,
			ip:  ips6,
		},
		ip4: ipRecord{
			ttl: p.ttl,
			ip:  ips4,
		},
	})

	return nil
}

func (p *lookupIPFuncDNS) Name() string {
	return "LookupIPFunc DNS"
}
