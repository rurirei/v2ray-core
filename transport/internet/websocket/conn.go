package websocket

import (
	"sync"
	"time"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/errors"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"

	"github.com/gorilla/websocket"
)

// httpConn is a wrapper for net.Conn over WebSocket httpConn.
type httpConn struct {
	sync.Mutex

	conn       *websocket.Conn
	remoteAddr net.Addr

	reader io.Reader
}

func (c *httpConn) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	mb = buffer.Compact(mb)
	_, err := buffer.WriteMultiTo(c, mb)
	return err
}

func (c *httpConn) Write(b []byte) (int, error) {
	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *httpConn) Read(b []byte) (int, error) {
	for {
		reader, err := c.getReader()
		if err != nil {
			return 0, err
		}

		nBytes, err := reader.Read(b)
		if errors.Cause(err) == io.EOF {
			c.reader = nil
			continue
		}
		return nBytes, err
	}
}

func (c *httpConn) getReader() (io.Reader, error) {
	c.Lock()
	defer c.Unlock()

	if c.reader != nil {
		return c.reader, nil
	}

	_, reader, err := c.conn.NextReader()
	if err != nil {
		return nil, err
	}

	c.reader = reader

	return reader, nil
}

func (c *httpConn) Close() error {
	errs := make([]error, 0, 2)

	if err := c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(5*time.Second)); err != nil {
		errs = append(errs, err)
	}

	if err := c.conn.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return newError("failed to close httpConn").WithError(errs)
	}
	return nil
}

func (c *httpConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *httpConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *httpConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}

	return nil
}

func (c *httpConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *httpConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
