package udp

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

// Dial dials a new UDP connection to the given destination.
func Dial(src net.Address) (net.PacketConn, error) {
	return internet.DialUDPSystem(src)
}
