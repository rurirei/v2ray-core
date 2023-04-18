package tor

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

type ClientSetting struct {
	Dialer net.Dialer
}

type client struct {
	dialer net.Dialer
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		dialer: setting.Dialer,
	}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, _ internet.DialTCPFunc, _ internet.DialUDPFunc) error {
	tcpHandler := func(dialTCPFunc internet.DialTCPFunc, address net.Address, link transport.Link) []error {
		conn, err := func() (net.Conn, error) {
			ib, _ := content.GetInbound()

			return dialTCPFunc(ib.Source, address)
		}()
		if err != nil {
			return []error{err}
		}
		defer func() {
			_ = conn.Close()
		}()

		connWriter, connReader := buffer.NewAllToBytesWriter(conn), buffer.NewIOReader(conn)

		requestDone := func() error {
			return buffer.Copy(connWriter, link.Reader)
		}

		responseDone := func() error {
			return buffer.Copy(link.Writer, connReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	udpHandler := func(dialTCPFunc internet.DialTCPFunc, address net.Address, link transport.Link) []error {
		conn, err := func() (net.Conn, error) {
			ib, _ := content.GetInbound()

			return dialTCPFunc(ib.Source, address)
		}()
		if err != nil {
			return []error{err}
		}
		defer func() {
			_ = conn.Close()
		}()

		connWriter, connReader := buffer.NewSequentialWriter(conn), buffer.NewPacketConnReader(conn)

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

	dialTCPFunc := func(_, dst net.Address) (net.Conn, error) {
		return c.dialer.Dial(dst.Network.This(), dst.DomainPreferredAddress())
	}

	if errs := func() []error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return tcpHandler(dialTCPFunc, address, link)
		case net.Network_UDP:
			return udpHandler(dialTCPFunc, address, link)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}
