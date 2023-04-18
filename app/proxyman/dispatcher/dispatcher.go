package dispatcher

import (
	"sync"
	"time"

	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/app/proxyman/outbound"
	"v2ray.com/core/app/router"
	sniffer_app "v2ray.com/core/app/sniffer"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	sniffer_proto "v2ray.com/core/common/protocol/sniffer"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport"
)

type dispatcher struct {
	handlers outbound.Manager
	router   router.Matcher
}

func NewDispatcher(handlers outbound.Manager, router router.Matcher) proxyman.Dispatcher {
	return &dispatcher{
		handlers: handlers,
		router:   router,
	}
}

func (d *dispatcher) Dispatch(content session.Content, address net.Address) (transport.Link, error) {
	route := func(content session.Content, address net.Address) (string, error) {
		ib, _ := content.GetInbound()

		if tag, ok := d.router.MatchContent(content, address); ok {
			newError("taking detour [%s] [%s] for [%s] [%s]", ib.Tag, tag, ib.Source.NetworkAndDomainPreferredAddress(), address.NetworkAndDomainPreferredAddress()).AtInfo().Logging()
			return tag, nil
		}
		return "", newError("no matched outbound for [%s] [%s]", ib.Tag, ib.Source.NetworkAndDomainPreferredAddress())
	}

	handle := func(tag string, address net.Address, link transport.Link) error {
		handler, ok := d.handlers.Get(tag)
		if !ok {
			return newError("outbound handler not found [%s]", tag)
		}

		return handler.Dispatch(content, address, link)
	}

	dispatch := func(address net.Address, outboundLink transport.Link, cReadWriter *cachedReadWriter) error {
		address = cReadWriter.Sniff(address)

		tag, err := route(content, address)
		if err != nil {
			return err
		}

		return handle(tag, address, outboundLink)
	}

	inboundLink, outboundLink, cReadWriter := newLink()

	go func() {
		if err := dispatch(address, outboundLink, cReadWriter); err != nil {
			newError("failed to dispatch").WithError(err).AtDebug().Logging()
		}
	}()

	return inboundLink, nil
}

func newLink() (transport.Link, transport.Link, *cachedReadWriter) {
	inboundLink, outboundLink := transport.NewLink()

	cReadWriter := &cachedReadWriter{
		reader:          outboundLink.Reader,
		PipeWriteCloser: outboundLink.Writer,
	}

	outboundLink.Reader = cReadWriter
	outboundLink.Writer = cReadWriter

	return inboundLink, outboundLink, cReadWriter
}

type cachedReadWriter struct {
	sync.Mutex

	data buffer.MultiBuffer

	reader transport.PipeReader
	transport.PipeWriteCloser
}

func (r *cachedReadWriter) ReadMultiBufferTimeout(timeout time.Duration) (buffer.MultiBuffer, error) {
	if mb := r.readCache(); mb != nil && !mb.IsEmpty() {
		return mb, nil
	}

	return r.reader.ReadMultiBufferTimeout(timeout)
}

func (r *cachedReadWriter) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	if mb := r.readCache(); mb != nil && !mb.IsEmpty() {
		return mb, nil
	}

	return r.reader.ReadMultiBuffer()
}

func (r *cachedReadWriter) readCache() buffer.MultiBuffer {
	r.Lock()
	defer r.Unlock()

	mb := r.data
	r.data = nil
	return mb
}

func (r *cachedReadWriter) Sniff(address net.Address) net.Address {
	r.Lock()
	defer r.Unlock()

	result, err := func() (sniffer_proto.SniffResult, error) {
		cache := func(b []byte) error {
			mb, err := r.reader.ReadMultiBuffer()
			if err != nil {
				defer buffer.ReleaseMulti(mb)
				return err
			}

			n := mb.CopyBytes(b)
			b = b[:n]

			if r.data != nil {
				r.data = buffer.MergeMulti(r.data, mb)
			} else {
				r.data = mb
			}

			return nil
		}

		b := buffer.New()
		defer b.Release()

		rb := b.Extend(buffer.Size)

		if err := cache(rb); err != nil {
			return sniffer_proto.SniffResult{}, err
		}

		return sniffer_app.Sniff(b, address.IP, address.Network)
	}()

	if err != nil {
		newError("failed to sniff domain of [%s]", address.NetworkAndDomainPreferredAddress()).WithError(err).AtDebug().Logging()
	} else {
		newError("sniffed domain [%s] [%d] of [%s]", result.Domain, result.Protocol, address.NetworkAndDomainPreferredAddress()).AtInfo().Logging()
		address = result.AsAddress(address)
	}

	return address
}

func (r *cachedReadWriter) CloseWrite() error {
	r.Lock()
	defer r.Unlock()

	r.data = buffer.ReleaseMulti(r.data)

	return r.PipeWriteCloser.Close()
}

//go:generate go run v2ray.com/core/common/errors/errorgen
