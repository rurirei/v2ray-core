package internet

import (
	"v2ray.com/core/common/net"
)

type HubFunc = func(net.Address) (Hub, error)

type ListenerBindIfaceToListenConfigFunc = func(*net.ListenConfig, net.Network, net.Address) (net.Address, error)

type ListenerFunc func(net.Address) (Listener, error)

type Hub interface {
	Receive() <-chan net.Conn
	Close() error
}

type Listener interface {
	Receive() <-chan net.Conn
	Addr() net.Addr
	Close() error
}
