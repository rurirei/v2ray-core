package udp

import (
	"v2ray.com/core/common/bytespool"
	"v2ray.com/core/common/net"
)

var (
	errSymmetric = newError("udp symmetric dropped")
)

type ConnSymmetric struct {
	net.PacketConn

	Address net.Address
}

func (c *ConnSymmetric) Write(p []byte) (int, error) {
	return c.WriteTo(p, c.RemoteAddr())
}

func (c *ConnSymmetric) Read(p []byte) (int, error) {
	n, _, err := c.ReadFrom(p)
	return n, err
}

func (c *ConnSymmetric) RemoteAddr() net.Addr {
	return c.Address.AddrWithIPAddress()
}

type PacketConnSymmetric struct {
	net.PacketConn

	Address net.Address
}

func (c *PacketConnSymmetric) ReadFrom(p []byte) (int, net.Addr, error) {
	buf := bytespool.Alloc(bytespool.Size)
	defer bytespool.Free(buf)

	n, addr, err := c.PacketConn.ReadFrom(buf)

	if addr == nil || !net.AddressFromAddr(addr).Equal(c.Address) {
		return 0, nil, errSymmetric
	}

	return copy(p, buf[:n]), addr, err
}
