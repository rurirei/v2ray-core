package udp

import (
	"time"

	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/cache"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	udp_proto "v2ray.com/core/common/protocol/udp"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/signal"
	"v2ray.com/core/transport"
)

type DispatcherFunc = func(proxyman.Dispatcher, CallbackFunc) Dispatcher

type CallbackFunc = func(CallbackSetting) error

type CallbackSetting struct {
	Packet        udp_proto.Packet
	RequestHeader protocol.RequestHeader
}

type DispatchSetting struct {
	Content       session.Content
	Address       net.Address
	RequestHeader protocol.RequestHeader
}

type Dispatcher interface {
	Dispatch(DispatchSetting, buffer.MultiBuffer) error
	Close() error
}

type symmetricDispatcher struct {
	dispatcher   proxyman.Dispatcher
	callbackFunc CallbackFunc

	pool cache.Pool
}

func NewSymmetricDispatcher(dispatcher proxyman.Dispatcher, callbackFunc CallbackFunc) Dispatcher {
	return &symmetricDispatcher{
		dispatcher:   dispatcher,
		callbackFunc: callbackFunc,
		pool:         cache.NewPool(),
	}
}

func (d *symmetricDispatcher) Dispatch(setting DispatchSetting, mb buffer.MultiBuffer) error {
	link, err := d.doHandleInput(setting)
	if err != nil {
		return err
	}

	return d.handleOutput(link, mb)
}

func (d *symmetricDispatcher) doHandleInput(setting DispatchSetting) (transport.Link, error) {
	newLink := func() (transport.Link, error) {
		link, err := d.dispatcher.Dispatch(setting.Content, setting.Address)
		if err != nil {
			return transport.Link{}, err
		}

		d.pool.Set(setting.Address.DomainPreferredAddress(), link)

		go func() {
			if err := d.handleInput(setting, link); err != nil {
				newError("failed to handle UDP input").WithError(err).AtDebug().Logging()
			}
		}()

		return link, nil
	}

	if link0, ok := d.pool.Get(setting.Address.DomainPreferredAddress()); ok {
		return link0.(transport.Link), nil
	}
	return newLink()
}

func (d *symmetricDispatcher) handleInput(setting DispatchSetting, link transport.Link) error {
	for {
		mb, err := link.Reader.ReadMultiBuffer()
		if err != nil {
			return err
		}

		for _, b := range mb {
			if err := d.callbackFunc(CallbackSetting{
				Packet: udp_proto.Packet{
					Payload: b,
					Source:  setting.Address,
				},
				RequestHeader: setting.RequestHeader,
			}); err != nil {
				return err
			}
		}
	}
}

func (d *symmetricDispatcher) handleOutput(link transport.Link, mb buffer.MultiBuffer) error {
	return link.Writer.WriteMultiBuffer(mb)
}

func (d *symmetricDispatcher) Close() error {
	d.pool.Range(func(_, v interface{}) bool {
		_ = v.(transport.Link).Writer.Close()

		return true
	})

	_ = d.pool.Close()

	return nil
}

func DialDispatch(content session.Content, address net.Address, dispatcher proxyman.Dispatcher) net.Conn {
	c := &dispatchConn{
		pending:     udp_proto.NewPipe(),
		closeSignal: signal.NewNotifier(),
		done:        signal.NewDone(),
		content:     content,
		address:     address,
	}

	c.dispatcher = NewSymmetricDispatcher(dispatcher, c.callback)

	go func() {
		_ = c.keepCloser()
	}()

	return c
}

func DialPacketDispatch(content session.Content, dispatcher proxyman.Dispatcher) net.PacketConn {
	c := &dispatchConn{
		pending:     udp_proto.NewPipe(),
		closeSignal: signal.NewNotifier(),
		done:        signal.NewDone(),
		content:     content,
	}

	c.dispatcher = NewSymmetricDispatcher(dispatcher, c.callback)

	go func() {
		_ = c.keepCloser()
	}()

	return c
}

type dispatchConn struct {
	dispatcher Dispatcher

	pending     udp_proto.PipeReadWriteCloser
	closeSignal signal.Notifier
	done        signal.Done

	content session.Content
	address net.Address
}

func (c *dispatchConn) Write(p []byte) (int, error) {
	return c.WriteTo(p, c.address.AddrWithIPAddress())
}

func (c *dispatchConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if c.done.Done() {
		return 0, io.ErrClosedPipe
	}

	return len(p), c.dispatcher.Dispatch(DispatchSetting{
		Content: c.content,
		Address: net.AddressFromAddr(addr),
	}, buffer.MultiBuffer{buffer.FromBytes(p)})
}

func (c *dispatchConn) callback(setting CallbackSetting) error {
	defer c.closeSignal.Signal()

	return c.pending.WritePacket(setting.Packet)
}

func (c *dispatchConn) keepCloser() error {
	timer := time.NewTimer(PipeOption.Timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return c.pending.Close()
		case <-c.closeSignal.Wait():
		}
	}
}

func (c *dispatchConn) Read(p []byte) (int, error) {
	n, _, err := c.ReadFrom(p)
	return n, err
}

func (c *dispatchConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, err := c.pending.Read(p)
	return n, nil, err
}

func (c *dispatchConn) Close() error {
	_ = c.closeSignal.Close()
	_ = c.done.Close()

	// _ = c.pending.Close()

	return nil
}

func (c *dispatchConn) LocalAddr() net.Addr {
	return nil
}

func (c *dispatchConn) RemoteAddr() net.Addr {
	return nil
}

func (c *dispatchConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *dispatchConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *dispatchConn) SetWriteDeadline(_ time.Time) error {
	return nil
}
