package udp

import (
	"time"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/bytespool"
	"v2ray.com/core/common/cache"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	udp_proto "v2ray.com/core/common/protocol/udp"
	"v2ray.com/core/common/signal"
	"v2ray.com/core/transport/internet"
)

var (
	HubOption = struct {
		RetOrigDest bool
	}{
		RetOrigDest: false,
	}
)

type hub struct {
	address net.Address
	conn    net.PacketConn
	ch      chan net.Conn
	pool    cache.Pool
}

func (h *hub) Receive() <-chan net.Conn {
	return h.ch
}

func (h *hub) Close() error {
	_ = h.pool.Close()

	close(h.ch)

	return h.conn.Close()
}

func (h *hub) handle() {
	receive := func() (udp_proto.Packet, error) {
		buf := bytespool.Alloc(bytespool.Size)
		defer bytespool.Free(buf)

		oobBytes := buf[:256]

		b := buffer.New()
		rb := b.Extend(buffer.Size)

		_, noob, _, addr, err := ReadUDPMsg(h.conn.(*net.GoUDPConn), rb, oobBytes)
		if err != nil {
			return udp_proto.Packet{}, err
		}

		pkt := udp_proto.Packet{
			Payload: b,
			Source:  net.AddressFromAddr(addr),
		}

		// TODO udp original destination
		if HubOption.RetOrigDest && noob > 0 {
			if dst, err := RetrieveOriginalDest(oobBytes[:noob]); err == nil {
				pkt.Target = dst
			}
		}

		return pkt, nil
	}

	write := func(pkt udp_proto.Packet) error {
		newConn := func() *udpConn {
			conn := newUDPConn(func(p []byte) (int, error) {
				return h.conn.WriteTo(p, pkt.Source.AddrWithIPAddress())
			}, func() error {
				h.pool.Delete(pkt.Source.IPAddress())
				return nil
			}, h.address, pkt.Source, pkt.Target)

			h.pool.Set(pkt.Source.IPAddress(), conn)

			return conn
		}

		var conn *udpConn
		if conn0, ok := h.pool.Get(pkt.Source.IPAddress()); ok {
			conn = conn0.(*udpConn)
		} else {
			conn = newConn()

			select {
			case h.ch <- conn:
			}
		}

		return conn.callback(pkt)
	}

	for {
		pkt, err := receive()
		if err != nil {
			newError("failed to read UDP conn").WithError(err).AtDebug().Logging()
			continue
		}

		if err := write(pkt); err != nil {
			newError("failed to write packet").WithError(err).AtDebug().Logging()
		}
	}
}

func Listen(address net.Address) (internet.Hub, error) {
	conn, err := internet.ListenPacketSystem(address)
	if err != nil {
		return nil, err
	}

	h := &hub{
		address: address,
		conn:    conn,
		ch:      make(chan net.Conn),
		pool:    cache.NewPool(),
	}

	go h.handle()

	return h, nil
}

var (
	PipeOption = struct {
		Timeout time.Duration
	}{
		Timeout: 60 * time.Second,
	}
)

type udpConn struct {
	localAddr, remoteAddr net.Addr

	output      io.WriteFunc
	closer      io.CloseFunc
	pending     udp_proto.PipeReadWriteCloser
	closeSignal signal.Notifier
}

func newUDPConn(writeFunc io.WriteFunc, closeFunc io.CloseFunc, lis, src, _ net.Address) *udpConn {
	conn := &udpConn{
		localAddr: &net.UDPAddr{
			IP:   lis.IP,
			Port: int(lis.Port),
		},
		remoteAddr: &net.UDPAddr{
			IP:   src.IP,
			Port: int(src.Port),
		},
		output:      writeFunc,
		closer:      closeFunc,
		pending:     udp_proto.NewPipe(),
		closeSignal: signal.NewNotifier(),
	}

	go func() {
		_ = conn.keepCloser()
	}()

	return conn
}

func (c *udpConn) Write(p []byte) (int, error) {
	return c.WriteTo(p, nil)
}

func (c *udpConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	return c.output(p)
}

func (c *udpConn) callback(pkt udp_proto.Packet) error {
	defer c.closeSignal.Signal()

	return c.pending.WritePacket(pkt)
}

func (c *udpConn) keepCloser() error {
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

func (c *udpConn) Read(p []byte) (int, error) {
	n, _, err := c.ReadFrom(p)
	return n, err
}

func (c *udpConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, err := c.pending.Read(p)
	return n, nil, err
}

func (c *udpConn) Close() error {
	_ = c.closeSignal.Close()

	// _ = c.pending.Close()

	return c.closer()
}

func (c *udpConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *udpConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *udpConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *udpConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *udpConn) SetWriteDeadline(_ time.Time) error {
	return nil
}
