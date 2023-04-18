package proxy

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

// An Server processes inbound connections.
type Server interface {
	Process(session.Content, net.Conn, proxyman.Dispatcher) error
}

// An Client processes outbound connections.
type Client interface {
	Process(session.Content, net.Address, transport.Link, internet.DialTCPFunc, internet.DialUDPFunc) error
}
