package sniffer

import (
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/http"
	"v2ray.com/core/common/protocol/quic"
	"v2ray.com/core/common/protocol/sniffer"
	"v2ray.com/core/common/protocol/tls"
)

var (
	errIsIP  = newError("is ip")
	errOnAll = newError("failed on all")
)

var (
	fakeSniffers = make([]sniffer.Sniffer, 0)

	dataSniffers = map[net.Network][]sniffer.Sniffer{
		net.Network_TCP: {
			http.NewSniffer(),
			tls.NewSniffer(),
		},
		net.Network_UDP: {
			quic.NewSniffer(),
		},
	}
)

func Sniff(b *buffer.Buffer, ip net.IP, network net.Network) (sniffer.SniffResult, error) {
	for _, s := range fakeSniffers {
		if result, err := s.Sniff(b, ip); err == nil {
			return result, nil
		}
	}

	for _, s := range dataSniffers[network] {
		if result, err := s.Sniff(b, ip); err == nil {
			if isDomainHost(result.Domain) {
				return result, nil
			}
			return sniffer.SniffResult{}, errIsIP
		}
	}

	return sniffer.SniffResult{}, errOnAll
}

// check if sniffer (like http_proto) returns a ip
func isDomainHost(host string) bool {
	a, err := net.ParseHost(host)
	if err != nil {
		return false
	}
	return a.IsDomainHost()
}

func RegisterFakeSniffer(s sniffer.Sniffer) {
	fakeSniffers = append(fakeSniffers, s)
}

//go:generate go run v2ray.com/core/common/errors/errorgen
