package outbound

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
)

type ForwardDialTCPFunc = func(session.Content, net.Address) internet.DialTCPFunc

type Setting struct {
	Tag                string
	Client             proxy.Client
	TCPDialFunc        internet.DialTCPFunc
	TCPForwardDialFunc ForwardDialTCPFunc
	UDPDialFunc        internet.DialUDPFunc
}

type outbound struct {
	tag                string
	client             proxy.Client
	tcpDialFunc        internet.DialTCPFunc
	tcpForwardDialFunc ForwardDialTCPFunc
	udpDialFunc        internet.DialUDPFunc
}

func NewOutbound(setting Setting) proxyman.Outbound {
	return &outbound{
		tag:                setting.Tag,
		client:             setting.Client,
		tcpDialFunc:        setting.TCPDialFunc,
		tcpForwardDialFunc: setting.TCPForwardDialFunc,
		udpDialFunc:        setting.UDPDialFunc,
	}
}

func (h *outbound) Dispatch(content session.Content, address net.Address, link transport.Link) error {
	tcpDialFunc, udpDialFunc, err := h.DialFunc(content, address)
	if err != nil {
		return err
	}

	return h.client.Process(content, address, link, tcpDialFunc, udpDialFunc)
}

func (h *outbound) DialFunc(content session.Content, address net.Address) (internet.DialTCPFunc, internet.DialUDPFunc, error) {
	if dialTCP := h.tcpForwardDialFunc; dialTCP != nil {
		return dialTCP(content, address), h.udpDialFunc, nil
	}

	return h.tcpDialFunc, h.udpDialFunc, nil
}

func (h *outbound) Tag() string {
	return h.tag
}
