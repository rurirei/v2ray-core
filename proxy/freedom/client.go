package freedom

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	udp_proto "v2ray.com/core/common/protocol/udp"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

type client struct {
}

func NewClient() proxy.Client {
	return &client{}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, dialTCPFunc internet.DialTCPFunc, dialUDPFunc internet.DialUDPFunc) error {
	tcpHandler := func(address net.Address, link transport.Link) []error {
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

	udpHandler := func(link transport.Link) []error {
		conn, err := func() (net.Conn, error) {
			ib, _ := content.GetInbound()

			conn, err := dialUDPFunc(ib.Source)
			if err != nil {
				return nil, err
			}

			return &udp_proto.ConnSymmetric{
				PacketConn: &udp_proto.PacketConnSymmetric{
					PacketConn: conn,
					Address:    address,
				},
				Address: address,
			}, nil
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

	if errs := func() []error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return tcpHandler(address, link)
		case net.Network_UDP:
			return udpHandler(link)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}
