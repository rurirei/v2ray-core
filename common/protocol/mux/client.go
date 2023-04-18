package mux

import (
	"time"

	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport"
)

const (
	timeoutFirstPayload = 100 * time.Millisecond
)

type client struct {
	proxyman.Outbound

	sessionsManager *sessionsManager
}

func NewClient(outbound proxyman.Outbound) proxyman.Outbound {
	return &client{
		Outbound:        outbound,
		sessionsManager: newSessionsManager(),
	}
}

func (c *client) Dispatch(content session.Content, address net.Address, link transport.Link) error {
	transferType := func() protocol.TransferType {
		ib, _ := content.GetInbound()

		// newError("receiving request [%d] [%s] [%s] [%s (%s)]", body.id, ib.Tag, ib.Source.NetworkAndDomainPreferredAddress(), body.target.NetworkAndDomainPreferredAddress(), targetDomain.This()).AtDebug().Logging()

		return protocol.TransferTypeFromNetwork(ib.Source.Network)
	}()

	manager := func() *sessionManager {
		manager, ok := c.sessionsManager.Require(address)

		if !ok {
			inboundLink, outboundLink := transport.NewLink()

			manager = c.sessionsManager.New(address, inboundLink)

			go func() {
				if err := func() error {
					content.SetMux(session.Mux{
						Enabled: true,
					})

					return c.Outbound.Dispatch(content, address, outboundLink)
				}(); err != nil {
					newError("failed to dispatch").WithError(err).AtDebug().Logging()
				}
			}()

			go func() {
				if err := func() error {
					defer func() {
						_ = manager.link.Writer.Close()
						c.sessionsManager.Delete(address)
					}()

					return c.handleOutput(manager)
				}(); err != nil {
					newError("failed to handle Output").WithError(err).AtDebug().Logging()
				}
			}()
		}

		return manager
	}()

	body := func() sessionBody {
		body := sessionBody{
			link:   link,
			target: address,
			id:     c.sessionsManager.IDGen(),
		}

		manager.Set(body.id, body)

		return body
	}()

	if err := c.handleInput(manager, body, sessionWriterMetadata{
		target:       body.target,
		id:           body.id,
		transferType: transferType,
	}); err != nil {
		return newError("failed to handle Input").WithError(err)
	}
	return nil
}

func (c *client) handleInput(manager *sessionManager, body sessionBody, meta sessionWriterMetadata) error {
	writer := &sessionWriter{
		writer:   manager.link.Writer,
		metadata: meta,
	}
	defer func() {
		_ = writer.Close()
	}()

	if err := func() error {
		err := buffer.Copy(writer, buffer.NewTimeoutReader(body.link.Reader, timeoutFirstPayload))
		if buffer.IsReadError(err) && buffer.CauseReadError(err) == buffer.ErrReadTimeout {
			return writer.WriteMultiBuffer(buffer.MultiBuffer{})
		}
		return err
	}(); err != nil {
		writer.hasError = true
		return err
	}

	if err := buffer.Copy(writer, body.link.Reader); err != nil {
		writer.hasError = true
		return err
	}

	return nil
}

func (c *client) handleOutput(manager *sessionManager) error {
	reader := buffer.NewBufferedReader(buffer.NewTimeoutReader(manager.link.Reader, SessionOption.Timeout))

	for {
		meta, err := unmarshalFromReader(reader)
		if err != nil && err != errNotNew {
			return newError("failed to read metadata").WithError(err)
		}

		if err := func() error {
			switch meta.status {
			case sessionStatusKeepAlive:
				return c.handleStatueKeepAlive(meta, reader)
			case sessionStatusEnd:
				return c.handleStatusEnd(manager, meta, reader)
			case sessionStatusNew:
				return c.handleStatusNew(meta, reader)
			case sessionStatusKeep:
				return c.handleStatusKeep(manager, meta, reader)
			default:
				return common.ErrUnknownNetwork
			}
		}(); err != nil {
			return newError("failed to process metadata").WithError(err)
		}
	}
}

func (c *client) handleStatusKeep(manager *sessionManager, meta frameMetadata, reader buffer.BufferedReader) error {
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
			_ = sendClosing()

			return buffer.Copy(buffer.Discard, rr)
		}

		return err
	}

	if !meta.option.Has(sessionOptionData) {
		return nil
	}

	if body, ok := manager.Get(meta.id); ok {
		return handleKeep(body)
	}

	return sendClosing()
}

func (c *client) handleStatusEnd(manager *sessionManager, meta frameMetadata, reader buffer.BufferedReader) error {
	if body, ok := manager.Get(meta.id); ok {
		defer func() {
			_ = body.link.Writer.Close()
			manager.Delete(meta.id)
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

func (c *client) handleStatueKeepAlive(meta frameMetadata, reader buffer.BufferedReader) error {
	if meta.option.Has(sessionOptionData) {
		return buffer.Copy(buffer.Discard, NewStreamReader(reader))
	}
	return nil
}

func (c *client) handleStatusNew(meta frameMetadata, reader buffer.BufferedReader) error {
	if meta.option.Has(sessionOptionData) {
		return buffer.Copy(buffer.Discard, NewStreamReader(reader))
	}
	return nil
}
