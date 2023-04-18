package http

import (
	"net/http"
	"net/url"
	"time"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/bufio"
	"v2ray.com/core/common/bytespool"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

const (
	timeoutFirstPayload = 100 * time.Millisecond
)

type ClientSetting struct {
	Address net.Address
}

type client struct {
	address net.Address
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		address: setting.Address,
	}
}

// Process implements proxyman.Client.Process.
// We first create a socket tunnel via HTTP CONNECT method,
// then redirect all inbound traffic to that tunnel.
func (c *client) Process(content session.Content, address net.Address, link transport.Link, dialTCPFunc internet.DialTCPFunc, _ internet.DialUDPFunc) error {
	tcpHandler := func(address net.Address, conn net.Conn, link transport.Link) []error {
		connWriter, connReader := buffer.NewAllToBytesWriter(conn), buffer.NewIOReader(conn)

		requestDone := func() error {
			return buffer.Copy(connWriter, link.Reader)
		}

		responseDone := func() error {
			return buffer.Copy(link.Writer, connReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	defer func() {
		_ = link.Writer.Close()
	}()

	conn, err := setupHTTPTunnel(content, address, dialTCPFunc, c.address)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	if err := func() error {
		mb, err := buffer.NewTimeoutReader(link.Reader, timeoutFirstPayload).ReadMultiBuffer()
		if err != nil && buffer.IsReadError(err) && buffer.CauseReadError(err) != buffer.ErrReadTimeout {
			return newError("failed to read first payload").WithError(err)
		}

		mbLen := mb.Len()
		firstPayload := bytespool.Alloc(mbLen)
		mb, _ = buffer.SplitBytes(mb, firstPayload)
		firstPayload = firstPayload[:mbLen]

		defer buffer.ReleaseMulti(mb)
		defer bytespool.Free(firstPayload)

		_, err = conn.Write(firstPayload)
		return newError("failed to write first payload").WithError(err)
	}(); err != nil {
		return err
	}

	if errs := func() []error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return tcpHandler(address, conn, link)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}

// setupHTTPTunnel will create a socket tunnel via HTTP CONNECT method
// connectHTTP1 supported only
func setupHTTPTunnel(content session.Content, target net.Address, dialTCPFunc internet.DialTCPFunc, address net.Address) (net.Conn, error) {
	ib, _ := content.GetInbound()

	conn, err := dialTCPFunc(ib.Source, address)
	if err != nil {
		return nil, err
	}

	request := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: target.DomainPreferredAddress()},
		Header: make(http.Header),
		Host:   target.DomainPreferredAddress(),
	}

	connectHTTP1 := func(req *http.Request, conn net.Conn) (net.Conn, error) {
		req.Header.Set("Client-Connection", "Keep-Alive")

		if err := req.Write(conn); err != nil {
			defer func() {
				_ = conn.Close()
			}()
			return nil, err
		}

		resp, err := http.ReadResponse(bufio.NewReader(conn), req)
		if err != nil {
			defer func() {
				_ = conn.Close()
			}()
			return nil, err
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			defer func() {
				_ = conn.Close()
			}()
			return nil, newError("proxy responded with non 200 code: %s", resp.Status)
		}

		return conn, nil
	}

	return connectHTTP1(request, conn)
}
