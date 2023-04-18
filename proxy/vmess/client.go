package vmess

import (
	"crypto/hmac"
	"crypto/sha256"
	"hash/crc64"
	"time"

	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/vmess"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
	"v2ray.com/core/proxy/vmess/encoding"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

const (
	timeoutFirstPayload = 100 * time.Millisecond
)

const (
	authenticatedLengthExperiment = true
	noTerminationSignalExperiment = false
)

type ClientSetting struct {
	Address net.Address
	User    vmess.User
}

type client struct {
	address net.Address
	user    vmess.User
}

func NewClient(setting ClientSetting) proxy.Client {
	return &client{
		address: setting.Address,
		user:    setting.User,
	}
}

func (c *client) Process(content session.Content, address net.Address, link transport.Link, dialTCPFunc internet.DialTCPFunc, _ internet.DialUDPFunc) error {
	handler := func(conn net.Conn, requestHeader protocol.RequestHeader, clientSession *encoding.ClientSession) []error {
		connWriter, connReader := buffer.NewBufferedWriter(buffer.NewAllToBytesWriter(conn)), buffer.NewBufferedReader(buffer.NewIOReader(conn))

		requestDone := func() error {
			if err := clientSession.EncodeRequestHeader(requestHeader, connWriter); err != nil {
				return newError("failed to encode requestHeader").WithError(err)
			}

			// todo caution vmess does not support a buffer.WriterTo writer
			// leaves a empty wrapper at buffer.CopyFromTo below
			bodyWriter, err := clientSession.EncodeRequestBody(requestHeader, connWriter)
			if err != nil {
				return newError("failed to start encoding").WithError(err)
			}

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

		responseDone := func() error {
			responseHeader, err := clientSession.DecodeResponseHeader(connReader)
			if err != nil {
				return newError("failed to read header").WithError(err)
			}
			if err := handleCommand(responseHeader.Command.Vmess); err != nil {
				return newError("failed to handle command").WithError(err)
			}

			bodyReader, err := clientSession.DecodeResponseBody(requestHeader, connReader)
			if err != nil {
				return newError("failed to start encoding response").WithError(err)
			}

			return buffer.Copy(link.Writer, bodyReader)
		}

		return task.Parallel(requestDone, responseDone)
	}

	defer func() {
		_ = link.Writer.Close()
	}()

	command := func() protocol.RequestCommand {
		ib, _ := content.GetInbound()

		return protocol.RequestCommand{
			Vmess: vmess.RequestCommandFromNetwork(ib.Source.Network, func() bool {
				if mux, ok := content.GetMux(); ok {
					return mux.Enabled
				}
				return false
			}()),
		}
	}()

	requestHeader := func() protocol.RequestHeader {
		request := protocol.RequestHeader{
			Command: command,
			Option: protocol.RequestOption{
				Vmess: vmess.RequestOptionChunkStream,
			},
			Version: protocol.RequestVersion{
				Vmess: vmess.VersionName,
			},
			User: protocol.RequestUser{
				Vmess: c.user,
			},
			Address: protocol.RequestAddress{
				Address: address,
			},
		}

		if request.User.Vmess.Security == vmess.Security_AES_128_GCM || request.User.Vmess.Security == vmess.Security_NONE || request.User.Vmess.Security == vmess.Security_CHACHA20_POLY1305 {
			request.Option.Vmess.Set(vmess.RequestOptionChunkMasking)
		}

		if shouldEnablePadding(request.User.Vmess.Security) && request.Option.Vmess.Has(vmess.RequestOptionChunkMasking) {
			request.Option.Vmess.Set(vmess.RequestOptionGlobalPadding)
		}

		if request.User.Vmess.Security == vmess.Security_ZERO {
			request.User.Vmess.Security = vmess.Security_NONE
			request.Option.Vmess.Clear(vmess.RequestOptionChunkStream)
			request.Option.Vmess.Clear(vmess.RequestOptionChunkMasking)
		}

		if authenticatedLengthExperiment {
			request.Option.Vmess.Set(vmess.RequestOptionAuthenticatedLength)
		}

		return request
	}()

	clientSession := func() *encoding.ClientSession {
		hashkdf := hmac.New(sha256.New, []byte("VMessBF"))
		hashkdf.Write(requestHeader.User.Vmess.ID.Bytes())

		behaviorSeed := crc64.Checksum(hashkdf.Sum(nil), crc64.MakeTable(crc64.ISO))

		return encoding.NewClientSession(vmess.DefaultIDHash, int64(behaviorSeed))
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
		case net.Network_TCP, net.Network_UDP:
			return handler(conn, requestHeader, clientSession)
		default:
			return []error{common.ErrUnknownNetwork}
		}
	}(); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}

func handleCommand(_ vmess.ResponseCommand) error {
	return nil
}

func shouldEnablePadding(s vmess.Security) bool {
	return s == vmess.Security_AES_128_GCM || s == vmess.Security_CHACHA20_POLY1305 || s == vmess.Security_AUTO
}
