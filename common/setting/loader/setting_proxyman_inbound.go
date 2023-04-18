package loader

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/app/proxyman/inbound"
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet"
)

func RegisterInboundHandler(handler proxyman.Inbound) {
	localInstance.InboundManager.Add(handler.Tag(), handler)
}

type InboundHandlerSetting struct {
	Tag          string
	Address      net.Address
	Server       proxy.Server
	ListenerFunc internet.ListenerFunc
	HubFunc      internet.HubFunc
}

func NewInboundHandler(setting InboundHandlerSetting) (proxyman.Inbound, error) {
	switch setting.Address.Network {
	case net.Network_TCP:
		return inbound.NewTCPInbound(localInstance.Dispatcher, inbound.TCPSetting{
			Tag:          setting.Tag,
			Address:      setting.Address,
			Server:       setting.Server,
			ListenerFunc: setting.ListenerFunc,
		})
	case net.Network_UDP:
		return inbound.NewUDPInbound(localInstance.Dispatcher, inbound.UDPSetting{
			Tag:     setting.Tag,
			Address: setting.Address,
			Server:  setting.Server,
			HubFunc: setting.HubFunc,
		})
	default:
		return nil, common.ErrUnknownNetwork
	}
}
