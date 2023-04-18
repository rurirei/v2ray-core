//go:build !linux && !freebsd

package udp

import (
	"v2ray.com/core/common/net"
)

func RetrieveOriginalDest(_ []byte) (net.Address, error) {
	return net.Address{}, newError("not supported")
}

func ReadUDPMsg(conn *net.GoUDPConn, payload []byte, _ []byte) (int, int, int, *net.UDPAddr, error) {
	nBytes, addr, err := conn.ReadFromUDP(payload)
	return nBytes, 0, 0, addr, err
}
