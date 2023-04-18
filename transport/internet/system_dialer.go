package internet

import (
	"v2ray.com/core/common/net"
)

func DialUDPSystem(src net.Address) (net.PacketConn, error) {
	src.Port = net.Port(0)

	return ListenPacketSystem(src)
}

func DialTCPSystem(src, dst net.Address) (net.Conn, error) {
	dialSystem := func(dst net.Address, dialer net.Dialer) (net.Conn, error) {
		return dialer.Dial(dst.Network.This(), dst.DomainPreferredAddress())
	}

	dialer := func() net.Dialer {
		src2 := src
		src2.Port = net.Port(0)

		dialer := &net.GoDialer{
			LocalAddr: src2.AddrWithIPAddress(),
		}

		return net.NewDialer(net.LocalDialFunc(src, dialer), net.LocalLookupIPFunc)
	}()

	return dialSystem(dst, dialer)
}
