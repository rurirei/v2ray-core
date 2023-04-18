package loader

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/app/proxyman/outbound"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/tcp"
	"v2ray.com/core/transport/internet/tls"
)

func RegisterOutboundHandler(handler proxyman.Outbound) {
	localInstance.OutboundManager.Add(handler.Tag(), handler)
}

type OutboundForwardDialTCPFuncSetting struct {
	Handlers  outbound.Manager
	Tag       string
	TLSConfig tls.Config
}

func NewForwardDialTCPFunc(setting OutboundForwardDialTCPFuncSetting) outbound.ForwardDialTCPFunc {
	return func(content session.Content, address net.Address) internet.DialTCPFunc {
		return func(_, _ net.Address) (net.Conn, error) {
			inboundLink, outboundLink := transport.NewLink()

			handler, ok := setting.Handlers.Get(setting.Tag)
			if !ok {
				return nil, newError("outbound handler not found [%s]", setting.Tag)
			}

			go func() {
				if err := handler.Dispatch(content, address, outboundLink); err != nil {
					newError("failed to dispatch proxied").WithError(err).AtDebug().Logging()
				}
			}()

			conn := tcp.DialDispatch(inboundLink)

			if len(setting.TLSConfig.ServerName) > 0 {
				return tls.Client(conn, setting.TLSConfig)
			}
			return conn, nil
		}
	}
}
