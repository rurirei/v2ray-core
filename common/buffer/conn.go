package buffer

import (
	"bytes"
	"time"

	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/signal"
)

func NewBufferedConn(writer Writer, reader BufferedReader) net.Conn {
	return &bufferedConn{
		reader: reader,
		writer: writer,
		done:   signal.NewDone(),
	}
}

type bufferedConn struct {
	reader BufferedReader
	writer Writer
	done   signal.Done
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *bufferedConn) Write(p []byte) (int, error) {
	if c.done.Done() {
		return 0, io.ErrClosedPipe
	}

	mb, err := ReadMultiFrom(bytes.NewReader(p))
	if err != nil {
		return 0, err
	}

	return mb.Len(), c.writer.WriteMultiBuffer(mb)
}

func (c *bufferedConn) Close() error {
	return c.done.Close()
}

func (c *bufferedConn) LocalAddr() net.Addr {
	return nil
}

func (c *bufferedConn) RemoteAddr() net.Addr {
	return nil
}

func (c *bufferedConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *bufferedConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *bufferedConn) SetWriteDeadline(_ time.Time) error {
	return nil
}
