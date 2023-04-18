package dns

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

// DialTCPFunc you must specify the routed outbound within the internal content
func DialTCPFunc(src net.Address, dialFunc internet.DialTCPFunc) net.DialFunc {
	return func(network, address string) (net.Conn, error) {
		dst, err := net.ParseAddress(network, address)
		if err != nil {
			return nil, err
		}

		return dialFunc(src, dst)
	}
}
