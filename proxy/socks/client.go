package socks

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/socks"
	udp_proto "v2ray.com/core/common/protocol/udp"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

type ClientSetting struct {
	Address net.Address
	User    socks.User
}

type client struct {
	address net.Address
	user    socks.User
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		address: setting.Address,
		user:    setting.User,
	}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, dialTCPFunc internet.DialTCPFunc, dialUDPFunc internet.DialUDPFunc) error {
	tcpHandler := func(conn net.Conn, link transport.Link, requestHeader protocol.RequestHeader) []error {
		connWriter, connReader := buffer.NewAllToBytesWriter(conn), buffer.NewIOReader(conn)

		requestDone := func() error {
			return buffer.Copy(connWriter, link.Reader)
		}

		responseDone := func() error {
			return buffer.Copy(link.Writer, connReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	udpHandler := func(_ net.Conn, link transport.Link, udpRequest, requestHeader protocol.RequestHeader) []error {
		connWriter, connReader, err := func() (*udpWriter, *udpReader, error) {
			udpConn, err := func() (net.Conn, error) {
				ib, _ := content.GetInbound()

				conn, err := dialUDPFunc(ib.Source)
				if err != nil {
					return nil, err
				}

				address2 := udpRequest.Address.AsAddress(ib.Source.Network)

				return &udp_proto.ConnSymmetric{
					PacketConn: &udp_proto.PacketConnSymmetric{
						PacketConn: conn,
						Address:    address2,
					},
					Address: address2,
				}, nil
			}()
			if err != nil {
				return nil, nil, err
			}

			return &udpWriter{
					Request: requestHeader,
					Writer:  udpConn,
				}, &udpReader{
					Reader: udpConn,
				}, nil
		}()
		if err != nil {
			return []error{err}
		}
		defer func() {
			_ = connWriter.Close()
		}()

		requestDone := func() error {
			return buffer.Copy(buffer.NewSequentialWriter(connWriter), link.Reader)
		}

		responseDone := func() error {
			return buffer.Copy(link.Writer, connReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	defer func() {
		_ = link.Writer.Close()
	}()

	conn, err := func() (net.Conn, error) {
		ib, _ := content.GetInbound()

		address := c.address
		address.Network = ib.Source.Network

		return dialTCPFunc(ib.Source, address)
	}()
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	command := func() protocol.RequestCommand {
		ib, _ := content.GetInbound()

		return protocol.RequestCommand{
			Socks: socks.RequestCommandFromNetwork(ib.Source.Network, false),
		}
	}()

	requestHeader := func() protocol.RequestHeader {
		return protocol.RequestHeader{
			Command: command,
			User: protocol.RequestUser{
				Socks: c.user,
			},
			Address: protocol.RequestAddress{
				Address: address,
			},
		}
	}()

	udpRequest, err := func() (protocol.RequestHeader, error) {
		udpRequest, err := ClientHandshake(requestHeader, conn, conn)
		if err != nil {
			return protocol.RequestHeader{}, newError("failed to establish connection to server").WithError(err)
		}

		if udpRequest.Address.IP.Equal(net.AnyIPv4) || udpRequest.Address.IP.Equal(net.AnyIPv6) {
			udpRequest.Address = protocol.RequestAddress{
				Address: c.address,
			}
		}

		return udpRequest, nil
	}()
	if err != nil {
		return err
	}

	if errs := func() []error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return tcpHandler(conn, link, requestHeader)
		case net.Network_UDP:
			return udpHandler(conn, link, udpRequest, requestHeader)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}
