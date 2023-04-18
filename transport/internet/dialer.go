package internet

import (
	"syscall"

	"v2ray.com/core/common/net"
)

type DialerBindIfaceToDialerFunc = func(*net.GoDialer, net.Network, net.Address) error

type DialerControlFunc = func(string, string, syscall.RawConn) error

type DialUDPFunc = func(net.Address) (net.PacketConn, error)

type DialTCPFunc = func(net.Address, net.Address) (net.Conn, error)
