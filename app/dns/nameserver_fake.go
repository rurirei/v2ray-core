package dns

import (
	"net/netip"
	"sync"

	"v2ray.com/core/common/cache"
	"v2ray.com/core/common/net"
)

type FakeDNSetting struct {
	IPRange6, IPRange4 netip.Prefix
}

type fakeDNS struct {
	fakePool *fakePool

	lookupIPFuncDNS Provider
}

func NewFakeDNS(setting FakeDNSetting) Provider {
	p := &fakeDNS{
		fakePool: newFakePool(setting.IPRange6, setting.IPRange4),
	}

	p.lookupIPFuncDNS = NewLookupIPFuncDNS(newBaseRecordPool(), LookupIPFuncDNSetting{
		LookupIPFunc: p.lookupIP,
	})

	return p
}

func (p *fakeDNS) LookupIP(network, host string) ([]net.IP, error) {
	return p.lookupIPFuncDNS.LookupIP(network, host)
}

func (p *fakeDNS) lookupIP(_, host string) ([]net.IP, error) {
	ip6, err := p.fakePool.LookupIPv6(host)
	if err != nil {
		return nil, err
	}

	ip4, err := p.fakePool.LookupIPv4(host)
	if err != nil {
		return nil, err
	}

	return []net.IP{ip6, ip4}, nil
}

func (p *fakeDNS) Name() string {
	return "fake DNS"
}

type FakePool interface {
	Lookback(net.IP) (string, bool)
}

func NewFakePool(provider Provider) FakePool {
	return provider.(*fakeDNS).fakePool
}

type fakePool struct {
	pool6, pool4 *fakePoolElement
}

func newFakePool(iprange6, iprange4 netip.Prefix) *fakePool {
	return &fakePool{
		pool6: newFakePoolElement(iprange6),
		pool4: newFakePoolElement(iprange4),
	}
}

func (p *fakePool) LookupIPv6(host string) (net.IP, error) {
	return p.pool6.Lookup(host)
}

func (p *fakePool) LookupIPv4(host string) (net.IP, error) {
	return p.pool4.Lookup(host)
}

func (p *fakePool) Lookback(ip net.IP) (string, bool) {
	switch len(ip) {
	case net.IPv6len:
		return p.pool6.Lookback(ip)
	case net.IPv4len:
		return p.pool4.Lookback(ip)
	default:
		return "", false
	}
}

type fakePoolElement struct {
	sync.Mutex

	iprange        netip.Prefix
	ipinit, ipnext netip.Addr
	pool           cache.Pool
}

func newFakePoolElement(iprange netip.Prefix) *fakePoolElement {
	return &fakePoolElement{
		iprange: iprange,
		ipinit:  iprange.Addr(),
		ipnext:  iprange.Addr(),
		pool:    cache.NewPool(),
	}
}

func (p *fakePoolElement) Lookup(host string) (net.IP, error) {
	p.Lock()
	defer p.Unlock()

	ipnext := func() netip.Addr {
		ipnext := func() netip.Addr {
			ipnext := p.ipnext.Next()

			if !p.iprange.Contains(ipnext) {
				ipnext = p.ipinit
			}

			return ipnext
		}

		p.ipnext = ipnext()

		return p.ipnext
	}()

	p.pool.Set(ipnext.String(), host)

	return ipnext.AsSlice(), nil
}

func (p *fakePoolElement) Lookback(ip net.IP) (string, bool) {
	if host0, ok := p.pool.Get(ip.String()); ok {
		return host0.(string), true
	}
	return "", false
}
