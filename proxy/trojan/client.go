package trojan

import (
	"time"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/trojan"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/proxy/trojan/cipher"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

const (
	timeoutFirstPayload = 100 * time.Millisecond
)

type ClientSetting struct {
	Address net.Address
	User    trojan.User
}

type client struct {
	address net.Address
	user    cipher.User
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		address: setting.Address,
		user:    cipher.BuildUser(setting.User),
	}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, dialTCPFunc internet.DialTCPFunc, _ internet.DialUDPFunc) error {
	tcpHandler := func(address net.Address, conn net.Conn, link transport.Link) []error {
		connWriter, connReader := buffer.NewBufferedWriter(buffer.NewAllToBytesWriter(conn)), buffer.NewIOReader(conn)

		requestDone := func() error {
			if err := WriteRequestHeader(connWriter, address, c.user); err != nil {
				return newError("failed to write first header").WithError(err)
			}

			if err := buffer.Copy(connWriter, buffer.NewTimeoutReader(link.Reader, timeoutFirstPayload)); err != nil && buffer.IsReadError(err) && buffer.CauseReadError(err) != buffer.ErrReadTimeout {
				return newError("failed to write first payload").WithError(err)
			}

			if err := connWriter.SetBuffered(false); err != nil {
				return err
			}

			return buffer.Copy(connWriter, link.Reader)
		}

		responseDone := func() error {
			return buffer.Copy(link.Writer, connReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	udpHandler := func(address net.Address, conn net.Conn, link transport.Link) []error {
		// TODO question udp over tcp conn uses NewAllToBytesWriter?
		connWriter, connReader := buffer.NewBufferedWriter(buffer.NewSequentialWriter(conn)), &udpReader{
			Reader: conn,
		}

		bodyWriter := buffer.NewSequentialWriter(&udpWriter{
			Writer: connWriter,
		})

		requestDone := func() error {
			if err := WriteRequestHeader(connWriter, address, c.user); err != nil {
				return newError("failed to write first header").WithError(err)
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
			return buffer.Copy(link.Writer, connReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	defer func() {
		_ = link.Writer.Close()
	}()

	conn, err := func() (net.Conn, error) {
		ib, _ := content.GetInbound()

		return dialTCPFunc(ib.Source, c.address)
	}()
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	if errs := func() []error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return tcpHandler(address, conn, link)
		case net.Network_UDP:
			return udpHandler(address, conn, link)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}
