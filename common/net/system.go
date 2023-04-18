package net

import (
	"net"
)

type (
	GoDialer   = net.Dialer
	GoResolver = net.Resolver

	Conn       = net.Conn
	PacketConn = net.PacketConn

	GoTCPConn = net.TCPConn
	GoUDPConn = net.UDPConn

	Addr     = net.Addr
	TCPAddr  = net.TCPAddr
	UDPAddr  = net.UDPAddr
	UnixAddr = net.UnixAddr

	IP    = net.IP
	IPNet = net.IPNet

	Listener     = net.Listener
	ListenConfig = net.ListenConfig

	Buffers = net.Buffers
)

var (
	IPv4len = net.IPv4len
	IPv6len = net.IPv6len

	ResolveTCPAddr  = net.ResolveTCPAddr
	ResolveUDPAddr  = net.ResolveUDPAddr
	ResolveUnixAddr = net.ResolveUnixAddr

	SplitHostPort = net.SplitHostPort
	JoinHostPort  = net.JoinHostPort

	ParseIP   = net.ParseIP
	ParseCIDR = net.ParseCIDR

	Dial        = net.Dial
	DialTimeout = net.DialTimeout

	Listen       = net.Listen
	ListenPacket = net.ListenPacket

	LookupIP = net.LookupIP
)
