package vmess

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/vmess"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/proxy/vmess/encoding"
	"v2ray.com/core/proxy/vmess/validator"
)

const (
	aeadEnforced     = true
	securityEnforced = true
)

type ServerSetting struct {
	User vmess.User
}

type server struct {
	clients        *validator.TimedUserValidator
	sessionHistory *encoding.SessionHistory
}

func NewServer(setting ServerSetting) proxy.Server {
	s := &server{
		clients:        validator.NewTimedUserValidator(vmess.DefaultIDHash),
		sessionHistory: encoding.NewSessionHistory(),
	}

	if err := s.clients.Add(setting.User); err != nil {
		panic(err)
	}

	return s
}

func (s *server) Process(content session.Content, conn net.Conn, dispatcher proxyman.Dispatcher) error {
	connWriter, connReader, err := func() (buffer.BufferedWriter, buffer.BufferedReader, error) {
		ib, _ := content.GetInbound()

		switch ib.Source.Network {
		case net.Network_TCP:
			return buffer.NewBufferedWriter(buffer.NewAllToBytesWriter(conn)), buffer.NewBufferedReader(buffer.NewIOReader(conn)), nil
		// case net.Network_UDP:
		//	return buffer.NewBufferedWriter(buffer.NewSequentialWriter(conn)), buffer.NewBufferedReader(buffer.NewPacketConnReader(conn)), nil
		default:
			return nil, nil, common.ErrUnknownNetwork
		}
	}()
	if err != nil {
		return err
	}

	serverSession := func() *encoding.ServerSession {
		serverSession := encoding.NewServerSession(s.clients, s.sessionHistory)
		serverSession.SetAEADForced(aeadEnforced)
		return serverSession
	}()

	requestHeader, err := func() (protocol.RequestHeader, error) {
		return serverSession.DecodeRequestHeader(connReader)
	}()
	if err != nil {
		return newError("invalid request").WithError(err)
	}

	dst := requestHeader.Address.AsAddress(requestHeader.Command.Vmess.Network())

	link, err := dispatcher.Dispatch(content, dst)
	if err != nil {
		return err
	}

	newError("receiving request [%s] [%s]", conn.RemoteAddr().String(), dst.NetworkAndDomainPreferredAddress()).AtInfo().Logging()

	requestDone := func() error {
		defer func() {
			_ = link.Writer.Close()
		}()

		bodyReader, err := serverSession.DecodeRequestBody(requestHeader, connReader)
		if err != nil {
			return newError("failed to start decoding").WithError(err)
		}

		return buffer.Copy(link.Writer, bodyReader)
	}

	responseDone := func() error {
		responseHeader := protocol.ResponseHeader{}
		if err := serverSession.EncodeResponseHeader(responseHeader, connWriter); err != nil {
			return err
		}

		bodyWriter, err := serverSession.EncodeResponseBody(requestHeader, connWriter)
		if err != nil {
			return newError("failed to start decoding responseHeader").WithError(err)
		}

		// Optimize for small responseHeader packet
		/* data, err := link.Reader.ReadMultiBuffer()
		if err != nil {
			return err
		}
		if err := bodyWriter.WriteMultiBuffer(data); err != nil {
			return err
		} */

		if err := buffer.Copy(bodyWriter, buffer.NewTimeoutReader(link.Reader, timeoutFirstPayload)); err != nil && buffer.IsReadError(err) && buffer.CauseReadError(err) != buffer.ErrReadTimeout {
			return newError("failed to write first payload").WithError(err)
		}

		if err := connWriter.SetBuffered(false); err != nil {
			return err
		}

		if err := buffer.Copy(bodyWriter, link.Reader); err != nil {
			return err
		}

		if requestHeader.Option.Vmess.Has(vmess.RequestOptionChunkStream) && !noTerminationSignalExperiment {
			return bodyWriter.WriteMultiBuffer(buffer.MultiBuffer{})
		}
		return nil
	}

	if errs := task.Parallel(requestDone, responseDone); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}

// Close TODO call this
func (s *server) Close() error {
	_ = s.clients.Close()
	_ = s.sessionHistory.Close()

	return nil
}

func isInsecureEncryption(s vmess.Security) bool {
	return s == vmess.Security_NONE || s == vmess.Security_LEGACY || s == vmess.Security_UNKNOWN
}
