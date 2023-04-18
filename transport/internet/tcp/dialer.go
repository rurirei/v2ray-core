package tcp

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

// Dial dials a new TCP connection to the given destination.
func Dial(src, dst net.Address) (net.Conn, error) {
	return internet.DialTCPSystem(src, dst)
}
