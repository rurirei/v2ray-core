package tun

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet/udp"
)

type server struct {
}

func NewServer() proxy.Server {
	return &server{}
}

func (s *server) Process(content session.Content, conn net.Conn, dispatcher proxyman.Dispatcher) error {
	return func() error {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return s.processTCP(content, conn, dispatcher)
		case net.Network_UDP:
			return s.handleUDPPayload(content, conn, dispatcher)
		default:
			return common.ErrUnknownNetwork
		}
	}()
}

func (s *server) processTCP(content session.Content, conn net.Conn, dispatcher proxyman.Dispatcher) error {
	content = func() session.Content {
		ib, _ := content.GetInbound()

		ib.Source = net.AddressFromAddr(conn.LocalAddr())
		content.SetInbound(ib)

		return content
	}()

	connWriter, connReader := buffer.NewAllToBytesWriter(conn), buffer.NewIOReader(conn)

	dst := net.AddressFromAddr(conn.RemoteAddr())

	link, err := dispatcher.Dispatch(content, dst)
	if err != nil {
		return err
	}

	newError("receiving request [%s] [%s]", conn.LocalAddr().String(), dst.NetworkAndDomainPreferredAddress()).AtInfo().Logging()

	requestDone := func() error {
		defer func() {
			_ = link.Writer.Close()
		}()

		return buffer.Copy(link.Writer, connReader)
	}

	responseDone := func() error {
		return buffer.Copy(connWriter, link.Reader)
	}

	if errs := task.Parallel(requestDone, responseDone); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}

func (s *server) handleUDPPayload(content session.Content, conn net.Conn, dispatcher proxyman.Dispatcher) error {
	content = func() session.Content {
		ib, _ := content.GetInbound()

		ib.Source = net.AddressFromAddr(conn.LocalAddr())
		content.SetInbound(ib)

		return content
	}()

	connWriter, connReader := buffer.NewSequentialWriter(conn), buffer.NewPacketConnReader(conn)

	callback := func(setting udp.CallbackSetting) error {
		payload := setting.Packet.Payload
		defer payload.Release()

		return connWriter.WriteMultiBuffer(buffer.MultiBuffer{payload})
	}

	udpServer := udp.NewSymmetricDispatcher(dispatcher, callback)
	defer func() {
		_ = udpServer.Close()
	}()

	for {
		mb, err := connReader.ReadMultiBuffer()
		if err != nil {
			return err
		}

		for _, payload := range mb {
			dst := net.AddressFromAddr(conn.RemoteAddr())

			newError("receiving request [%s] [%s]", conn.LocalAddr().String(), dst.NetworkAndDomainPreferredAddress()).AtInfo().Logging()

			if err := udpServer.Dispatch(udp.DispatchSetting{
				Content: content,
				Address: dst,
			}, buffer.MultiBuffer{payload}); err != nil {
				newError("failed to dispatch UDP output").WithError(err).AtDebug().Logging()
			}
		}
	}
}
