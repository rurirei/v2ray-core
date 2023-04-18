package mux

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport"
)

type server struct {
	dispatcher proxyman.Dispatcher

	sessionsManager *sessionsManager
}

func NewServer(dispatcher proxyman.Dispatcher) proxyman.Dispatcher {
	return &server{
		dispatcher:      dispatcher,
		sessionsManager: newSessionsManager(),
	}
}

func (s *server) Dispatch(content session.Content, address net.Address) (transport.Link, error) {
	if mux, ok := content.GetMux(); !ok || !mux.Enabled {
		return s.dispatcher.Dispatch(content, address)
	}

	inboundLink, outboundLink := transport.NewLink()

	transferType := func() protocol.TransferType {
		ib, _ := content.GetInbound()

		return protocol.TransferTypeFromNetwork(ib.Source.Network)
	}()

	manager := func() *sessionManager {
		manager := s.sessionsManager.New(address, outboundLink)

		return manager
	}()

	go func() {
		if err := s.handleInput(manager, sessionWriterMetadata{
			target:       address,
			transferType: transferType,
		}, content, s.dispatcher); err != nil {
			newError("failed to handle Input").WithError(err).AtDebug().Logging()
		}
	}()

	return inboundLink, nil
}

func (s *server) handleInput(manager *sessionManager, meta sessionWriterMetadata, content session.Content, dispatcher proxyman.Dispatcher) error {
	reader := buffer.NewBufferedReader(manager.link.Reader)

	for {
		meta2, err := unmarshalFromReader(reader)
		if err != nil && err != errNotNew {
			return newError("failed to read metadata").WithError(err)
		}

		if err := func() error {
			switch meta2.status {
			case sessionStatusKeepAlive:
				return s.handleStatueKeepAlive(meta2, reader)
			case sessionStatusEnd:
				return s.handleStatusEnd(manager, meta2, reader)
			case sessionStatusNew:
				return s.handleStatusNew(manager, meta, meta2, reader, content, dispatcher)
			case sessionStatusKeep:
				return s.handleStatusKeep(manager, meta2, reader)
			default:
				return common.ErrUnknownNetwork
			}
		}(); err != nil {
			return newError("failed to process metadata").WithError(err)
		}
	}
}

func (s *server) handleStatusEnd(manager *sessionManager, meta frameMetadata, reader buffer.BufferedReader) error {
	if body, ok := manager.Get(meta.id); ok {
		defer func() {
			_ = body.link.Writer.Close()
		}()

		if meta.option.Has(sessionOptionError) {
			return nil
		}
	}

	if meta.option.Has(sessionOptionData) {
		return buffer.Copy(buffer.Discard, NewStreamReader(reader))
	}
	return nil
}

func (s *server) handleStatueKeepAlive(meta frameMetadata, reader buffer.BufferedReader) error {
	if meta.option.Has(sessionOptionData) {
		return buffer.Copy(buffer.Discard, NewStreamReader(reader))
	}
	return nil
}

func (s *server) handleStatusKeep(manager *sessionManager, meta frameMetadata, reader buffer.BufferedReader) error {
	if !meta.option.Has(sessionOptionData) {
		return nil
	}

	sendClosing := func() error {
		// Notify remote peer to close this session.
		closingWriter := &sessionWriter{
			writer:   manager.link.Writer,
			followup: true,
			metadata: sessionWriterMetadata{
				id:           meta.id,
				transferType: protocol.TransferTypeStream,
			},
		}
		_ = closingWriter.Close()

		return buffer.Copy(buffer.Discard, NewStreamReader(reader))
	}

	handleKeep := func(body sessionBody) error {
		rr := NewReader(reader, protocol.TransferTypeFromNetwork(body.target.Network))

		err := buffer.Copy(body.link.Writer, rr)
		if buffer.IsReadError(err) && buffer.CauseReadError(err) == io.EOF {
			err = nil
		}

		if err != nil && buffer.IsWriteError(err) {
			/*defer func() {
				_ = body.link.Reader.Interrupt()
			}()*/

			_ = sendClosing()

			return buffer.Copy(buffer.Discard, rr)
		}

		return err
	}

	if body, ok := manager.Get(meta.id); ok {
		return handleKeep(body)
	}
	return sendClosing()
}

func (s *server) handleStatusNew(manager *sessionManager, meta sessionWriterMetadata, meta2 frameMetadata, reader buffer.BufferedReader, content session.Content, dispatcher proxyman.Dispatcher) error {
	link, err := func() (transport.Link, error) {
		link, err := dispatcher.Dispatch(content, net.Address{})
		if err != nil {
			return transport.Link{}, err
		}

		if meta2.option.Has(sessionOptionData) {
			_ = buffer.Copy(buffer.Discard, NewStreamReader(reader))
		}

		return link, nil
	}()
	if err != nil {
		return err
	}

	body := func() sessionBody {
		body := sessionBody{
			link:   link,
			target: meta2.target,
			id:     meta2.id,
		}
		manager.Set(body.id, body)

		return body
	}()

	go func() {
		writer := &sessionWriter{
			writer:   manager.link.Writer,
			followup: true,
			metadata: sessionWriterMetadata{
				id:           meta2.id,
				transferType: meta.transferType,
			},
		}
		defer func() {
			_ = writer.Close()
		}()

		if err := buffer.Copy(writer, body.link.Reader); err != nil {
			/*defer func() {
				_ = body.link.Reader.Interrupt()
			}()*/

			writer.hasError = true
			// return err
		}
	}()

	if meta2.option.Has(sessionOptionData) {
		rr := NewReader(reader, protocol.TransferTypeFromNetwork(meta2.target.Network))

		err := buffer.Copy(body.link.Writer, rr)
		if buffer.IsReadError(err) && buffer.CauseReadError(err) == io.EOF {
			err = nil
		}

		if err != nil {
			return buffer.Copy(buffer.Discard, rr)
		}
	}

	return nil
}
