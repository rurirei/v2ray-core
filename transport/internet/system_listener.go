package internet

import (
	"strings"
	"time"

	"v2ray.com/core/common/net"
)

// ListenPacketSystem listens on a local address for incoming UDP connections.
func ListenPacketSystem(address net.Address) (net.PacketConn, error) {
	return net.LocalListenPacketFunc(address)
}

type systemListener struct {
	net.Listener

	ch chan net.Conn
}

func (l *systemListener) Receive() <-chan net.Conn {
	return l.ch
}

func (l *systemListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.ch:
		return conn, nil
	}
}

func (l *systemListener) keepAccepting() {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			newError("failed to accept raw connections").WithError(err).AtDebug().Logging()
			if errStr := err.Error(); strings.Contains(errStr, "closed") {
				break
			}
			if errStr := err.Error(); strings.Contains(errStr, "too many") {
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		select {
		case l.ch <- conn:
		}
	}
}

// ListenSystem listens on a local address for incoming TCP connections.
func ListenSystem(address net.Address) (Listener, error) {
	listener, err := net.Listen(address.Network.This(), address.IPAddress())
	if err != nil {
		return nil, err
	}

	l := &systemListener{
		Listener: listener,
		ch:       make(chan net.Conn),
	}

	go l.keepAccepting()

	return l, nil
}
