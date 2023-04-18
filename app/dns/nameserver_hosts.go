package dns

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/dns"
)

var (
	errHosts = newError("is hosts")
)

type HostsDNSetting struct {
	Domain         string
	Hosts6, Hosts4 []net.IP
}

type hostsDNS struct {
	lookupIPFuncDNS Provider
}

func NewHostsDNS(setting ...HostsDNSetting) Provider {
	pool := func() *baseRecordPool {
		pool := newBaseRecordPool()

		for _, v := range setting {
			pool.set(dns.GetFqdn(v.Domain), ipRecordComp{
				ip6: ipRecord{
					ip: v.Hosts6,
				},
				ip4: ipRecord{
					ip: v.Hosts4,
				},
			})
		}

		return pool
	}()

	p := &hostsDNS{}

	p.lookupIPFuncDNS = NewLookupIPFuncDNS(pool, LookupIPFuncDNSetting{
		LookupIPFunc: p.lookupIP,
	})

	return p
}

func (p *hostsDNS) LookupIP(network, host string) ([]net.IP, error) {
	return p.lookupIPFuncDNS.LookupIP(network, host)
}

func (p *hostsDNS) lookupIP(_, _ string) ([]net.IP, error) {
	return nil, errHosts
}

func (p *hostsDNS) Name() string {
	return "hosts DNS"
}
