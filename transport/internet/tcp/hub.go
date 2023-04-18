package tcp

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

func Listen(address net.Address) (internet.Listener, error) {
	return internet.ListenSystem(address)
}
