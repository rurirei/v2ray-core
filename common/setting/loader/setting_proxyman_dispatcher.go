package loader

import (
	"v2ray.com/core/app/proxyman/dispatcher"
)

func RegisterMuxDispatcher() {
	// localInstance.Dispatcher = mux.NewServer(localInstance.Dispatcher)
}

func RegisterDispatcher() {
	localInstance.Dispatcher = dispatcher.NewDispatcher(localInstance.OutboundManager, localInstance.OutboundMatcher)
}
