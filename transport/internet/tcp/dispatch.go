package tcp

import (
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport"
)

func DialDispatch(link transport.Link) net.Conn {
	return &dispatchConn{
		link: link,
		Conn: buffer.NewBufferedConn(link.Writer, buffer.NewBufferedReader(link.Reader)),
	}
}

type dispatchConn struct {
	link transport.Link

	net.Conn
}

func (c *dispatchConn) Close() error {
	_ = c.link.Writer.Close()

	return c.Conn.Close()
}
