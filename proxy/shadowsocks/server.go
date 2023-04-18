package shadowsocks

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/shadowsocks"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet/udp"
)

type ServerSetting struct {
	User shadowsocks.User
}

type server struct {
	user shadowsocks.User
}

func NewServer(setting ServerSetting) proxy.Server {
	return &server{
		user: setting.User,
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
	connWriter, connReader := buffer.NewBufferedWriter(buffer.NewAllToBytesWriter(conn)), buffer.NewBufferedReader(buffer.NewIOReader(conn))

	requestHeader, bodyReader, err := ReadTCPSession(s.user, connReader)
	if err != nil {
		return newError("failed to read request").WithError(err)
	}

	dst := requestHeader.Address.AsAddress(requestHeader.Command.Shadowsocks.Network())

	link, err := dispatcher.Dispatch(content, dst)
	if err != nil {
		return err
	}

	newError("receiving request [%s] [%s]", conn.RemoteAddr().String(), dst.NetworkAndDomainPreferredAddress()).AtInfo().Logging()

	requestDone := func() error {
		defer func() {
			_ = link.Writer.Close()
		}()

		return buffer.Copy(link.Writer, bodyReader)
	}

	responseDone := func() error {
		responseWriter, err := WriteTCPResponse(requestHeader, connWriter)
		if err != nil {
			return newError("failed to write response").WithError(err)
		}

		/* data, err := link.Reader.ReadMultiBuffer()
		if err != nil {
			return err
		}
		if err := responseWriter.WriteMultiBuffer(data); err != nil {
			return err
		} */

		if err := buffer.Copy(responseWriter, buffer.NewTimeoutReader(link.Reader, timeoutFirstPayload)); err != nil && buffer.IsReadError(err) && buffer.CauseReadError(err) != buffer.ErrReadTimeout {
			return newError("failed to write first payload").WithError(err)
		}

		if err := connWriter.SetBuffered(false); err != nil {
			return err
		}

		return buffer.Copy(responseWriter, link.Reader)
	}

	if errs := task.Parallel(requestDone, responseDone); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
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
			requestHeader, err := DecodeUDPPacket(s.user, payload)
			if err != nil {
				newError("failed to parse UDP requestHeader").WithError(err).AtDebug().Logging()
				payload.Release()
				continue
			}

			dst := requestHeader.Address.AsAddress(requestHeader.Command.Shadowsocks.Network())

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
