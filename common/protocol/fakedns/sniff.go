package fakedns

import (
	"v2ray.com/core/app/dns"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/sniffer"
)

type fakeSniffer struct {
	fakePool dns.FakePool
}

func NewSniffer(fakePool dns.FakePool) sniffer.Sniffer {
	return &fakeSniffer{
		fakePool: fakePool,
	}
}

func (s *fakeSniffer) Protocol() sniffer.Protocol {
	return sniffer.Fake
}

func (s *fakeSniffer) Sniff(_ *buffer.Buffer, ip net.IP) (sniffer.SniffResult, error) {
	if host, ok := s.fakePool.Lookback(ip); ok {
		return sniffer.SniffResult{
			Protocol: s.Protocol(),
			Domain:   host,
		}, nil
	}
	return sniffer.SniffResult{}, newError("not found")
}
