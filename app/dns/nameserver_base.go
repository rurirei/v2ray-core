package dns

import (
	"sync/atomic"

	"golang.org/x/net/dns/dnsmessage"

	"v2ray.com/core/common"
	"v2ray.com/core/common/cache"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/dns"
)

var (
	errNil = newError("ips is nil")
)

type sendRequestFunc = func([]byte) ([]byte, error)

type baseDNS struct {
	pool *baseRecordPool

	reqID atomic.Uint32
}

func newBaseDNS() *baseDNS {
	return &baseDNS{
		pool: newBaseRecordPool(),
	}
}

func (p *baseDNS) LookupIP(network, host string, sendRequestFunc sendRequestFunc) ([]net.IP, error) {
	fqdn := dns.GetFqdn(host)

	// lookup cache
	if ips, err := p.pool.get(network, fqdn); err == nil {
		return ips, nil
	}

	// send query
	rec, err := p.doRequest(buildRequest(fqdn, p.reqIDGen), sendRequestFunc)
	if err != nil {
		return nil, err
	}
	p.pool.set(fqdn, rec)

	// lookup again
	return p.pool.get(network, fqdn)
}

func (p *baseDNS) doRequest(req []ipRequest, sendRequestFunc sendRequestFunc) (ipRecordComp, error) {
	rec := ipRecordComp{}

	for _, r := range req {
		b, err := packMessage(r.msg)
		if err != nil {
			return rec, newError("failed to pack dns query").WithError(err)
		}
		resp, err := sendRequestFunc(b.Bytes())
		if err != nil {
			return rec, newError("failed to retrieve response").WithError(err)
		}
		record, err := parseResponse(resp)
		if err != nil {
			return rec, newError("failed to handle DOH response").WithError(err)
		}
		switch r.reqType {
		case dnsmessage.TypeAAAA:
			rec.ip6 = record
		case dnsmessage.TypeA:
			rec.ip4 = record
		default:
			return ipRecordComp{}, common.ErrUnknownNetwork
		}
	}

	return rec, nil
}

func (p *baseDNS) reqIDGen() uint16 {
	return uint16(p.reqID.Add(1))
}

type baseRecordPool struct {
	pool cache.Pool
}

func newBaseRecordPool() *baseRecordPool {
	return &baseRecordPool{
		pool: cache.NewPool(),
	}
}

func (p *baseRecordPool) set(fqdn string, rec ipRecordComp) {
	if ttl := rec.getTTL(); ttl > 0 {
		p.pool.SetExpire(fqdn, rec, ttl)
	} else {
		p.pool.Set(fqdn, rec)
	}
}

func (p *baseRecordPool) get(network, fqdn string) ([]net.IP, error) {
	ips := make([]net.IP, 0)

	f6 := func() {
		if value, ok := p.pool.Get(fqdn); ok {
			if rec, ok := value.(ipRecordComp); ok {
				ips = append(ips, rec.ip6.ip...)
			}
		}
	}
	f4 := func() {
		if value, ok := p.pool.Get(fqdn); ok {
			if rec, ok := value.(ipRecordComp); ok {
				ips = append(ips, rec.ip4.ip...)
			}
		}
	}

	switch network {
	case net.Network_IP:
		f6()
		f4()
	case net.Network_IP6:
		f6()
	case net.Network_IP4:
		f4()
	default:
		return nil, common.ErrUnknownNetwork
	}

	if len(ips) > 0 {
		return ips, nil
	}
	return nil, errNil
}

type baseIPPool struct {
	pool6, pool4 cache.Pool

	ttl int64
}

func newBaseIPPool() *baseIPPool {
	return &baseIPPool{
		pool6: cache.NewPool(),
		pool4: cache.NewPool(),
		ttl:   net.LookupIPOption.TTL,
	}
}

func (p *baseIPPool) set(network, host string, ips []net.IP) error {
	f6 := func() {
		p.pool6.SetExpire(host, ips, p.ttl)
	}
	f4 := func() {
		p.pool4.SetExpire(host, ips, p.ttl)
	}

	switch network {
	case net.Network_IP6:
		f6()
	case net.Network_IP4:
		f4()
	default:
		return common.ErrUnknownNetwork
	}

	return nil
}

func (p *baseIPPool) get(network, host string) ([]net.IP, error) {
	ips := make([]net.IP, 0)

	f6 := func() {
		if value, ok := p.pool6.Get(host); ok {
			ips = append(ips, value.([]net.IP)...)
		}
	}
	f4 := func() {
		if value, ok := p.pool4.Get(host); ok {
			ips = append(ips, value.([]net.IP)...)
		}
	}

	switch network {
	case net.Network_IP:
		f6()
		f4()
	case net.Network_IP6:
		f6()
	case net.Network_IP4:
		f4()
	default:
		return nil, common.ErrUnknownNetwork
	}

	if len(ips) > 0 {
		return ips, nil
	}
	return nil, errNil
}
