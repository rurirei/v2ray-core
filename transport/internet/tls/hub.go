package tls

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

type tlsListener struct {
	internet.Listener

	ch chan net.Conn

	config Config
}

func (l *tlsListener) Receive() <-chan net.Conn {
	return l.ch
}

func (l *tlsListener) Close() error {
	close(l.ch)

	return l.Listener.Close()
}

func (l *tlsListener) keepAccepting() {
	for {
		select {
		case c := <-l.Listener.Receive():
			c2, err := Server(c, l.config)
			if err != nil {
				newError("failed to accept raw connections").WithError(err).AtDebug().Logging()
				continue
			}

			select {
			case l.ch <- c2:
			}
		}
	}
}

type ListenSetting struct {
	Config Config
}

func Listen(setting ListenSetting, listenerFunc internet.ListenerFunc) internet.ListenerFunc {
	return func(address net.Address) (internet.Listener, error) {
		l0, err := listenerFunc(address)
		if err != nil {
			return nil, err
		}

		l := &tlsListener{
			Listener: l0,
			config:   setting.Config,
		}

		go l.keepAccepting()

		return l, nil
	}
}
