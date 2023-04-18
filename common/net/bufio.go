package net

import (
	"v2ray.com/core/common/bufio"
	"v2ray.com/core/common/io"
)

type bufConn struct {
	Conn

	pending io.ReadWriteCloser
}

func NewBufConn(conn Conn) Conn {
	return &bufConn{
		Conn:    conn,
		pending: bufio.NewPipe(conn),
	}
}

func (c *bufConn) Write(p []byte) (int, error) {
	return c.pending.Write(p)
}

func (c *bufConn) Read(p []byte) (int, error) {
	return c.pending.Read(p)
}

func (c *bufConn) Close() error {
	_ = c.pending.Close()

	return c.Conn.Close()
}
