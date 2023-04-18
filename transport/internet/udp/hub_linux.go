//go:build linux

package udp

import (
	"syscall"

	"golang.org/x/sys/unix"

	"v2ray.com/core/common/net"
)

func RetrieveOriginalDest(oob []byte) (net.Address, error) {
	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return net.Address{}, err
	}

	for _, msg := range msgs {
		if msg.Header.Level == syscall.SOL_IP && msg.Header.Type == syscall.IP_RECVORIGDSTADDR {
			ip := net.IP(msg.Data[4:8])
			port := net.PortFromBytes(msg.Data[2:4])
			return net.ParseAddress(net.Network_UDP, net.JoinHostPort(ip.String(), port.String()))
		} else if msg.Header.Level == syscall.SOL_IPV6 && msg.Header.Type == unix.IPV6_RECVORIGDSTADDR {
			ip := net.IP(msg.Data[8:24])
			port := net.PortFromBytes(msg.Data[2:4])
			return net.ParseAddress(net.Network_UDP, net.JoinHostPort(ip.String(), port.String()))
		}
	}
	return net.Address{}, newError("unknown error")
}

func ReadUDPMsg(conn *net.GoUDPConn, payload []byte, oob []byte) (int, int, int, *net.UDPAddr, error) {
	return conn.ReadMsgUDP(payload, oob)
}
