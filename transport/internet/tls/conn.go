package tls

import (
	"crypto/tls"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
)

type Conn interface {
	buffer.Writer

	HandshakeServerName() (net.Domain, error)
}

type conn struct {
	*tls.Conn
}

func (c *conn) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	mb = buffer.Compact(mb)
	_, err := buffer.WriteMultiTo(c, mb)
	return err
}

func (c *conn) HandshakeServerName() (net.Domain, error) {
	if err := c.Handshake(); err != nil {
		return net.EmptyDomain, err
	}

	if state := c.ConnectionState(); len(state.ServerName) > 0 {
		return net.Domain(state.ServerName), nil
	}
	return net.EmptyDomain, newError("state.ServerName is nil")
}

// Client initiates a TLS client handshake on the given connection.
func Client(c net.Conn, config Config) (net.Conn, error) {
	config2 := config.BuildTLSClient()

	return &conn{
		Conn: tls.Client(c, config2),
	}, nil
}

// Server initiates a TLS server handshake on the given connection.
func Server(c net.Conn, config Config) (net.Conn, error) {
	config2, err := config.BuildTLSServer()
	if err != nil {
		return nil, err
	}

	return &conn{
		Conn: tls.Server(c, config2),
	}, nil
}
