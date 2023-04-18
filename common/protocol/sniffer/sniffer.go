package sniffer

import (
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
)

type Protocol byte

const (
	Fake Protocol = iota
	HTTP
	TLS
	QUIC
)

type Sniffer interface {
	Protocol() Protocol
	Sniff(*buffer.Buffer, net.IP) (SniffResult, error)
}

type SniffResult struct {
	Protocol Protocol
	Domain   string
}

func (s SniffResult) AsAddress(address net.Address) net.Address {
	address.Domain = net.Domain(s.Domain)
	address.IP = nil

	return address
}

func (s SniffResult) IsValid() bool {
	return len(s.Domain) > 0
}
