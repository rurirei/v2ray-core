package trojan

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
)

// RequestCommand is a custom command in a proxy request.
type RequestCommand byte

const (
	RequestCommandTCP = RequestCommand(1)
	RequestCommandUDP = RequestCommand(3)
)

func (c RequestCommand) TransferType() int {
	switch c {
	case RequestCommandTCP:
		return 1
	case RequestCommandUDP:
		return 2
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (c RequestCommand) Network() net.Network {
	switch c {
	case RequestCommandTCP:
		return net.Network_TCP
	case RequestCommandUDP:
		return net.Network_UDP
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func RequestCommandFromNetwork(network net.Network, _ bool) RequestCommand {
	switch network {
	case net.Network_TCP:
		return RequestCommandTCP
	case net.Network_UDP:
		return RequestCommandUDP
	default:
		panic(common.ErrUnknownNetwork)
	}
}

const (
	MaxLength = 8192
)
