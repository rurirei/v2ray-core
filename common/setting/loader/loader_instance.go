package loader

import (
	dns_app "v2ray.com/core/app/dns"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/app/proxyman/inbound"
	"v2ray.com/core/app/proxyman/outbound"
	router_app "v2ray.com/core/app/router"
)

var (
	localInstance = &Instance{
		InboundManager:  inbound.NewManager(),
		OutboundManager: outbound.NewManager(),
	}
)

type Instance struct {
	Dispatcher proxyman.Dispatcher

	Nameserver        dns_app.ConditionServer
	NameserverMatcher router_app.Matcher

	InboundManager inbound.Manager

	OutboundManager outbound.Manager
	OutboundMatcher router_app.Matcher
}

func RequireInstance() *Instance {
	return localInstance
}
