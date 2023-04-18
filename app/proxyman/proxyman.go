package proxyman

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport"
)

type Inbound interface {
	Close() error

	Tag() string
}

type Outbound interface {
	Dispatch(session.Content, net.Address, transport.Link) error

	Tag() string
}

type Dispatcher interface {
	Dispatch(session.Content, net.Address) (transport.Link, error)
}
