package tor

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/proxy"
)

type server struct {
}

func NewServer() proxy.Server {
	return &server{}
}

func (s *server) Process(_ session.Content, _ net.Conn, _ proxyman.Dispatcher) error {
	return newError("not implemented")
}
