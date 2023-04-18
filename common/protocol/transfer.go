package protocol

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
)

type TransferType byte

const (
	TransferTypeStream TransferType = 0
	TransferTypePacket TransferType = 1
)

func (c TransferType) Network() net.Network {
	switch c {
	case TransferTypeStream:
		return net.Network_TCP
	case TransferTypePacket:
		return net.Network_UDP
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func TransferTypeFromNetwork(network net.Network) TransferType {
	switch network {
	case net.Network_TCP:
		return TransferTypeStream
	case net.Network_UDP:
		return TransferTypePacket
	default:
		panic(common.ErrUnknownNetwork)
	}
}

type HostLength byte

const (
	HostLengthIPv4   HostLength = 1
	HostLengthDomain HostLength = 2
	HostLengthIPv6   HostLength = 3
)
