package setting

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/router"
)

type ListenSetting struct {
	Address         net.Address
	Protocol        ListenName
	ProtocolSetting interface{}
}

type ProxySetting struct {
	Address         net.Address
	Protocol        ProxyName
	ProtocolSetting interface{}
	StreamSetting   StreamSetting
}

type StreamSetting struct {
	TransportName    TransportName
	TransportSetting interface{}
	SecurityName     SecurityName
	SecuritySetting  interface{}
}

type RouteSetting struct {
	ConditionSetting []router.Rule
}
