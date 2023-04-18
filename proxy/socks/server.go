package socks

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/socks"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet/udp"
)

type ServerSetting struct {
	ResponseAddress net.Address
}

type server struct {
	responseAddress net.Address
}

func NewServer(setting ServerSetting) proxy.Server {
	return &server{
		responseAddress: setting.ResponseAddress,
	}
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
	connWriter, connReader := buffer.NewAllToBytesWriter(conn), buffer.NewBufferedReader(buffer.NewIOReader(conn))

	serverSession := func() *ServerSession {
		ib, _ := content.GetInbound()

		return &ServerSession{
			Address: ServerAddress{
				Listen: ib.Gateway,
				Client: ib.Source,
				Conf:   s.responseAddress,
			},
		}
	}()

	requestHeader, err := serverSession.Handshake(conn, connReader)
	if err != nil {
		return newError("failed to read request").WithError(err)
	}

	switch requestHeader.Command.Socks {
	case socks.RequestCommandTCP:
		return func() error {
			dst := requestHeader.Address.AsAddress(requestHeader.Command.Socks.Network())

			link, err := dispatcher.Dispatch(content, dst)
			if err != nil {
				return err
			}

			newError("receiving request [%s] [%s]", conn.RemoteAddr().String(), dst.NetworkAndDomainPreferredAddress()).AtInfo().Logging()

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
		}()
	case socks.RequestCommandUDP:
		return func() error {
			// The TCP connection closes after this method returns. We need to wait until
			// the client closes it.
			_, err = io.Discard(conn)
			return err
		}()
	default:
		return common.ErrUnknownNetwork
	}
}

func (s *server) handleUDPPayload(content session.Content, conn net.Conn, dispatcher proxyman.Dispatcher) error {
	connWriter, connReader := buffer.NewSequentialWriter(conn), buffer.NewPacketConnReader(conn)

	callback := func(setting udp.CallbackSetting) error {
		payload := setting.Packet.Payload
		defer payload.Release()

		data, err := EncodeUDPPacket(setting.RequestHeader, payload.Bytes())
		if err != nil {
			return err
		}
		defer data.Release()

		return connWriter.WriteMultiBuffer(buffer.MultiBuffer{data})
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
			requestHeader, err := DecodeUDPPacket(payload)
			if err != nil {
				newError("failed to parse UDP requestHeader").WithError(err).AtDebug().Logging()
				payload.Release()
				continue
			}

			dst := requestHeader.Address.AsAddress(requestHeader.Command.Socks.Network())

			newError("receiving request [%s] [%s]", conn.RemoteAddr().String(), dst.NetworkAndDomainPreferredAddress()).AtInfo().Logging()

			if err := udpServer.Dispatch(udp.DispatchSetting{
				Content:       content,
				Address:       dst,
				RequestHeader: requestHeader,
			}, buffer.MultiBuffer{payload}); err != nil {
				newError("failed to dispatch UDP output").WithError(err).AtDebug().Logging()
			}
		}
	}
}
