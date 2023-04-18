package block

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

type client struct {
}

func NewClient() proxy.Client {
	return &client{}
}

func (c *client) Process(_ session.Content, _ net.Address, link transport.Link, _ internet.DialTCPFunc, _ internet.DialUDPFunc) error {
	defer func() {
		_ = link.Writer.Close()
	}()

	return nil
}
