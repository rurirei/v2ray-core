package loader

import (
	"net/url"

	dns_app "v2ray.com/core/app/dns"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
	"v2ray.com/core/transport/internet/tcp"
)

var (
	NameserverDialFuncOption = struct {
		TCPSource, UDPSource net.Address
	}{
		TCPSource: net.LocalhostTCPAddress,
		UDPSource: net.LocalhostUDPAddress,
	}
)

type DoHSetting struct {
	Url string
	Tag string
}

func BuildDoHDNS(setting DoHSetting) (dns_app.Provider, error) {
	u, err := ParseNameserverUrl(setting.Url)
	if err != nil {
		return nil, err
	}

	dialFunc := NewNameserverDialTCPFunc(NewNameserverDialContent(session.Inbound{
		Tag: setting.Tag,
	}))

	doh := dns_app.NewDoHDNS(dns_app.DoHDNSetting{
		Url:      u,
		DialFunc: dialFunc,
	})

	return doh, nil
}

// TODO dns_app.DialUDPFunc
/*func NewNameserverDialUDPFunc(content session.Content) net.DialFunc {
	return dns_app.DialFunc(NameserverDialFuncOption.UDPSource, func(src, dst net.Address) (net.Conn, error) {
		ib, _ := content.GetInbound()
		ib.Source = src
		content.SetInbound(ib)

		conn, err := udp.DialDispatch(content, localInstance.Dispatcher, udp.NewSymmetricDispatcher)
		if err != nil {
			return nil, err
		}

		return udp_proto.NewNonPacketAddrConn(conn, dst)
	})
}*/

func NewNameserverDialTCPFunc(content session.Content) net.DialFunc {
	return dns_app.DialTCPFunc(NameserverDialFuncOption.TCPSource, func(src, dst net.Address) (net.Conn, error) {
		ib, _ := content.GetInbound()
		ib.Source = src
		content.SetInbound(ib)

		link, err := RequireInstance().Dispatcher.Dispatch(content, dst)
		if err != nil {
			return nil, err
		}

		return tcp.DialDispatch(link), nil
	})
}

func NewNameserverDialContent(ib session.Inbound) session.Content {
	content := session.NewContent()
	content.SetID(session.NewID())
	content.SetInbound(ib)

	return content
}

func ParseNameserverUrl(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}
