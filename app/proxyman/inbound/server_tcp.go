package inbound

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet"
)

type TCPSetting struct {
	Tag          string
	Address      net.Address
	Server       proxy.Server
	ListenerFunc internet.ListenerFunc
}

type tcpInbound struct {
	dispatcher proxyman.Dispatcher
	tag        string
	address    net.Address
	server     proxy.Server
	hub        internet.Listener
}

func NewTCPInbound(dispatcher proxyman.Dispatcher, setting TCPSetting) (proxyman.Inbound, error) {
	hub, err := setting.ListenerFunc(setting.Address)
	if err != nil {
		return nil, err
	}

	h := &tcpInbound{
		dispatcher: dispatcher,
		tag:        setting.Tag,
		address:    setting.Address,
		server:     setting.Server,
		hub:        hub,
	}

	go h.handle()

	newError("listening on %s", h.address.NetworkAndIPAddress()).AtInfo().Logging()

	return h, nil
}

func (h *tcpInbound) Close() error {
	return h.hub.Close()
}

func (h *tcpInbound) Tag() string {
	return h.tag
}

func (h *tcpInbound) handle() {
	for {
		select {
		case conn := <-h.hub.Receive():
			go func() {
				if err := h.callback(conn); err != nil {
					newError("failed to handle tcp conn").WithError(err).AtDebug().Logging()
				}
			}()
		}
	}
}

func (h *tcpInbound) callback(conn net.Conn) error {
	dispatch := func(conn net.Conn) error {
		defer func() {
			_ = conn.Close()
			newError("connection closed %s", conn.RemoteAddr().String()).AtDebug().Logging()
		}()

		content := session.NewContent()
		content.SetID(session.NewID())
		content.SetInbound(session.Inbound{
			Source:  net.AddressFromAddr(conn.RemoteAddr()),
			Gateway: h.address,
			Tag:     h.tag,
		})
		defer func() {
			_ = content.Close()
		}()

		return h.server.Process(content, conn, h.dispatcher)
	}

	return dispatch(conn)
}
