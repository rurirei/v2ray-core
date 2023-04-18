package shadowsocks

import (
	"time"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/shadowsocks"
	udp_proto "v2ray.com/core/common/protocol/udp"
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
	User    shadowsocks.User
}

type client struct {
	address net.Address
	user    shadowsocks.User
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		address: setting.Address,
		user:    setting.User,
	}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, dialTCPFunc internet.DialTCPFunc, dialUDPFunc internet.DialUDPFunc) error {
	tcpHandler := func(link transport.Link, requestHeader protocol.RequestHeader) []error {
		conn, err := func() (net.Conn, error) {
			ib, _ := content.GetInbound()

			address := c.address
			address.Network = ib.Source.Network

			return dialTCPFunc(ib.Source, address)
		}()
		if err != nil {
			return []error{err}
		}
		defer func() {
			_ = conn.Close()
		}()

		connWriter, connReader := buffer.NewBufferedWriter(buffer.NewAllToBytesWriter(conn)), buffer.NewBufferedReader(buffer.NewIOReader(conn))

		requestDone := func() error {
			bodyWriter, err := WriteTCPRequest(requestHeader, connWriter)
			if err != nil {
				return newError("failed to write request").WithError(err)
			}

			if err := buffer.Copy(bodyWriter, buffer.NewTimeoutReader(link.Reader, timeoutFirstPayload)); err != nil && buffer.IsReadError(err) && buffer.CauseReadError(err) != buffer.ErrReadTimeout {
				return newError("failed to write first payload").WithError(err)
			}

			if err := connWriter.SetBuffered(false); err != nil {
				return err
			}

			return buffer.Copy(bodyWriter, link.Reader)
		}

		responseDone := func() error {
			responseReader, err := ReadTCPResponse(requestHeader.User.Shadowsocks, connReader)
			if err != nil {
				return err
			}

			return buffer.Copy(link.Writer, responseReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	udpHandler := func(link transport.Link, requestHeader protocol.RequestHeader) []error {
		conn, err := func() (net.Conn, error) {
			ib, _ := content.GetInbound()

			conn, err := dialUDPFunc(ib.Source)
			if err != nil {
				return nil, err
			}

			address2 := c.address
			address2.Network = ib.Source.Network

			return &udp_proto.ConnSymmetric{
				PacketConn: &udp_proto.PacketConnSymmetric{
					PacketConn: conn,
					Address:    address2,
				},
				Address: address2,
			}, nil
		}()
		if err != nil {
			return []error{err}
		}
		defer func() {
			_ = conn.Close()
		}()

		connWriter, connReader := buffer.NewSequentialWriter(&udpWriter{
			Writer:  conn,
			Request: requestHeader,
		}), &udpReader{
			Reader: conn,
			User:   requestHeader.User.Shadowsocks,
		}

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

	command := func() protocol.RequestCommand {
		ib, _ := content.GetInbound()

		return protocol.RequestCommand{
			Shadowsocks: shadowsocks.RequestCommandFromNetwork(ib.Source.Network, false),
		}
	}()

	requestHeader := func() protocol.RequestHeader {
		return protocol.RequestHeader{
			Command: command,
			Version: protocol.RequestVersion{
				Shadowsocks: shadowsocks.VersionName,
			},
			User: protocol.RequestUser{
				Shadowsocks: c.user,
			},
			Address: protocol.RequestAddress{
				Address: address,
			},
		}
	}()

	if errs := func() []error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return tcpHandler(link, requestHeader)
		case net.Network_UDP:
			return udpHandler(link, requestHeader)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}
