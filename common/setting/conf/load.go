package conf

import (
	"net/netip"
	"strings"

	dns_app "v2ray.com/core/app/dns"
	"v2ray.com/core/app/proxyman/outbound"
	router_app "v2ray.com/core/app/router"
	"v2ray.com/core/common/geofile"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/mux"
	router_common "v2ray.com/core/common/router"
	"v2ray.com/core/common/setting/loader"
	"v2ray.com/core/proxy/block"
	dns_proxy "v2ray.com/core/proxy/dns"
	"v2ray.com/core/proxy/dokodemo"
	"v2ray.com/core/proxy/freedom"
	"v2ray.com/core/proxy/http"
	"v2ray.com/core/proxy/shadowsocks"
	"v2ray.com/core/proxy/socks"
	"v2ray.com/core/proxy/tor"
	"v2ray.com/core/proxy/trojan"
	"v2ray.com/core/proxy/tun"
	"v2ray.com/core/proxy/vmess"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/tcp"
	"v2ray.com/core/transport/internet/tls"
	tun_transport "v2ray.com/core/transport/internet/tun"
	"v2ray.com/core/transport/internet/udp"
	"v2ray.com/core/transport/internet/websocket"
)

const (
	strCIDRip  = "cidrip:"
	strGeoip   = "geoip:"
	strGeosite = "geosite:"

	preservedHosts = "preservedHosts"
	preservedDNS   = "preservedDNS"
)

var (
	AnyConditionDNS = func(s []string) router_common.Condition {
		return router_common.Condition{
			router_common.ConditionBody{
				Name:   router_app.LookupDomain,
				Length: router_common.Full,
				String: s,
			},
		}
	}

	AnyConditionOutbound = router_common.Condition{
		router_common.ConditionBody{
			Name:   router_app.SrcNetwork,
			Length: router_common.Full,
			String: []string{net.Network_TCP, net.Network_UDP},
		},
	}
)

func Loads() error {
	conf, err := unmarshal()
	if err != nil {
		return err
	}

	if err := conf.LoadRouter(); err != nil {
		return err
	}

	if err := conf.LoadDNS(); err != nil {
		return err
	}

	if err := conf.LoadOutbound(); err != nil {
		return err
	}

	if err := conf.LoadDispatcher(); err != nil {
		return err
	}

	if err := conf.LoadInbound(); err != nil {
		return err
	}

	return nil
}

func (c config) LoadInbound() error {
	for _, v := range c.Inbounds.Dokodemo {
		for _, network := range v.Network {
			address, err := net.ParseAddress(network, v.Listen)
			if err != nil {
				return err
			}

			handler, err := loader.NewInboundHandler(loader.InboundHandlerSetting{
				Tag:          v.Tag,
				Address:      address,
				Server:       dokodemo.NewServer(),
				ListenerFunc: tcp.Listen,
				HubFunc:      udp.Listen,
			})
			if err != nil {
				return err
			}

			loader.RegisterInboundHandler(handler)
		}
	}

	for _, v := range c.Inbounds.Http {
		for _, network := range v.Network {
			address, err := net.ParseAddress(network, v.Listen)
			if err != nil {
				return err
			}

			handler, err := loader.NewInboundHandler(loader.InboundHandlerSetting{
				Tag:          v.Tag,
				Address:      address,
				Server:       http.NewServer(),
				ListenerFunc: tcp.Listen,
				HubFunc:      udp.Listen,
			})
			if err != nil {
				return err
			}

			loader.RegisterInboundHandler(handler)
		}
	}

	for _, v := range c.Inbounds.Shadowsocks {
		for _, network := range v.Network {
			address, err := net.ParseAddress(network, v.Listen)
			if err != nil {
				return err
			}

			user, err := loader.BuildShadowsocksUser(loader.ShadowsocksUserSetting{
				Security: v.User.Security,
				Password: v.User.Password,
			})
			if err != nil {
				return err
			}

			handler, err := loader.NewInboundHandler(loader.InboundHandlerSetting{
				Tag:     v.Tag,
				Address: address,
				Server: shadowsocks.NewServer(shadowsocks.ServerSetting{
					User: user,
				}),
				ListenerFunc: func() internet.ListenerFunc {
					if len(v.Websocket.Path) > 0 {
						if len(v.Websocket.Tls.ServerName) > 0 {
							return tls.Listen(tls.ListenSetting{
								Config: loader.BuildTLSetting(loader.TLSetting{
									ServerName:  v.Websocket.Tls.ServerName,
									Certificate: v.Websocket.Tls.Certificate,
									Key:         v.Websocket.Tls.Key,
								}),
							}, websocket.Listen(websocket.ListenSetting{
								Path: v.Websocket.Path,
							}, tcp.Listen))
						}
						return websocket.Listen(websocket.ListenSetting{
							Path: v.Websocket.Path,
						}, tcp.Listen)
					}
					if len(v.Tcp.Tls.ServerName) > 0 {
						return tls.Listen(tls.ListenSetting{
							Config: loader.BuildTLSetting(loader.TLSetting{
								ServerName:  v.Tcp.Tls.ServerName,
								Certificate: v.Tcp.Tls.Certificate,
								Key:         v.Tcp.Tls.Key,
							}),
						}, tcp.Listen)
					}
					return tcp.Listen
				}(),
				HubFunc: udp.Listen,
			})
			if err != nil {
				return err
			}

			loader.RegisterInboundHandler(handler)
		}
	}

	for _, v := range c.Inbounds.Socks {
		for _, network := range v.Network {
			address, err := net.ParseAddress(network, v.Listen)
			if err != nil {
				return err
			}

			handler, err := loader.NewInboundHandler(loader.InboundHandlerSetting{
				Tag:     v.Tag,
				Address: address,
				Server: socks.NewServer(socks.ServerSetting{
					ResponseAddress: func() net.Address {
						if address2, err := net.ParseHost(v.Resp); err == nil {
							return address2
						}
						return net.Address{}
					}(),
				}),
				ListenerFunc: tcp.Listen,
				HubFunc:      udp.Listen,
			})
			if err != nil {
				return err
			}

			loader.RegisterInboundHandler(handler)
		}
	}

	for _, v := range c.Inbounds.Tun {
		for _, network := range v.Network {
			address := net.LocalhostTCPAddress
			address.Network = net.Network(network)

			handler, err := loader.NewInboundHandler(loader.InboundHandlerSetting{
				Tag:          v.Tag,
				Address:      address,
				Server:       tun.NewServer(),
				ListenerFunc: tun_transport.ListenTCP,
				HubFunc:      tun_transport.ListenUDP,
			})
			if err != nil {
				return err
			}

			loader.RegisterInboundHandler(handler)
		}
	}

	for _, v := range c.Inbounds.Vmess {
		for _, network := range v.Network {
			address, err := net.ParseAddress(network, v.Listen)
			if err != nil {
				return err
			}

			user, err := loader.BuildVmessUser(loader.VmessUserSetting{
				Security: loader.Vmess_Security_NONE,
				UUID:     v.User.UUID,
			})
			if err != nil {
				return err
			}

			handler, err := loader.NewInboundHandler(loader.InboundHandlerSetting{
				Tag:     v.Tag,
				Address: address,
				Server: vmess.NewServer(vmess.ServerSetting{
					User: user,
				}),
				ListenerFunc: func() internet.ListenerFunc {
					if len(v.Websocket.Path) > 0 {
						if len(v.Websocket.Tls.ServerName) > 0 {
							return tls.Listen(tls.ListenSetting{
								Config: loader.BuildTLSetting(loader.TLSetting{
									ServerName:  v.Websocket.Tls.ServerName,
									Certificate: v.Websocket.Tls.Certificate,
									Key:         v.Websocket.Tls.Key,
								}),
							}, websocket.Listen(websocket.ListenSetting{
								Path: v.Websocket.Path,
							}, tcp.Listen))
						}
						return websocket.Listen(websocket.ListenSetting{
							Path: v.Websocket.Path,
						}, tcp.Listen)
					}
					if len(v.Tcp.Tls.ServerName) > 0 {
						return tls.Listen(tls.ListenSetting{
							Config: loader.BuildTLSetting(loader.TLSetting{
								ServerName:  v.Tcp.Tls.ServerName,
								Certificate: v.Tcp.Tls.Certificate,
								Key:         v.Tcp.Tls.Key,
							}),
						}, tcp.Listen)
					}
					return tcp.Listen
				}(),
				HubFunc: udp.Listen,
			})
			if err != nil {
				return err
			}

			loader.RegisterInboundHandler(handler)
		}
	}

	return nil
}

func (c config) LoadDispatcher() error {
	loader.RegisterDispatcher()
	loader.RegisterMuxDispatcher()

	return nil
}

func (c config) LoadOutbound() error {
	for _, v := range c.Outbounds.Block {
		handler := outbound.NewOutbound(outbound.Setting{
			Tag:         v.Tag,
			Client:      block.NewClient(),
			TCPDialFunc: tcp.Dial,
			UDPDialFunc: udp.Dial,
		})

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Dns {
		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: dns_proxy.NewClient(dns_proxy.ClientSetting{
				LookupIPConditionFunc: loader.RequireInstance().Nameserver.LookupIPCondition,
			}),
			TCPDialFunc: tcp.Dial,
			UDPDialFunc: udp.Dial,
		})

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Freedom {
		handler := outbound.NewOutbound(outbound.Setting{
			Tag:         v.Tag,
			Client:      freedom.NewClient(),
			TCPDialFunc: tcp.Dial,
			UDPDialFunc: udp.Dial,
		})

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Http {
		address, err := net.ParseAddress(net.Network_TCP, v.Target)
		if err != nil {
			return err
		}

		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: http.NewClient(http.ClientSetting{
				Address: address,
			}),
			TCPDialFunc: tcp.Dial,
			UDPDialFunc: udp.Dial,
		})

		if v.Mux {
			handler = mux.NewClient(handler)
		}

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Shadowsocks {
		address, err := net.ParseAddress(net.Network_TCP, v.Target)
		if err != nil {
			return err
		}

		user, err := loader.BuildShadowsocksUser(loader.ShadowsocksUserSetting{
			Security: v.User.Security,
			Password: v.User.Password,
		})
		if err != nil {
			return err
		}

		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: shadowsocks.NewClient(shadowsocks.ClientSetting{
				Address: address,
				User:    user,
			}),
			TCPDialFunc: func() internet.DialTCPFunc {
				if len(v.Websocket.Path) > 0 {
					if len(v.Websocket.Tls.ServerName) > 0 {
						return tls.Dial(tls.DialSetting{
							Config: loader.BuildTLSetting(loader.TLSetting{
								ServerName: v.Websocket.Tls.ServerName,
							}),
						}, websocket.Dial(websocket.DialSetting{
							Path: v.Websocket.Path,
							TLSConfig: loader.BuildTLSetting(loader.TLSetting{
								ServerName: v.Websocket.Tls.ServerName,
							}),
						}, tcp.Dial))
					}
					return websocket.Dial(websocket.DialSetting{
						Path: v.Websocket.Path,
					}, tcp.Dial)
				}
				if len(v.Tcp.Tls.ServerName) > 0 {
					return tls.Dial(tls.DialSetting{
						Config: loader.BuildTLSetting(loader.TLSetting{
							ServerName: v.Tcp.Tls.ServerName,
						}),
					}, tcp.Dial)
				}
				return tcp.Dial
			}(),
			TCPForwardDialFunc: func() outbound.ForwardDialTCPFunc {
				if len(v.Forward.Tag) > 0 {
					return loader.NewForwardDialTCPFunc(loader.OutboundForwardDialTCPFuncSetting{
						Handlers: loader.RequireInstance().OutboundManager,
						Tag:      v.Forward.Tag,
						TLSConfig: func() tls.Config {
							if len(v.Forward.Tls.ServerName) > 0 {
								return loader.BuildTLSetting(loader.TLSetting{
									ServerName: v.Forward.Tls.ServerName,
								})
							}
							return tls.Config{}
						}(),
					})
				}
				return nil
			}(),
			UDPDialFunc: udp.Dial,
		})

		if v.Mux {
			handler = mux.NewClient(handler)
		}

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Socks {
		address, err := net.ParseAddress(net.Network_TCP, v.Target)
		if err != nil {
			return err
		}

		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: socks.NewClient(socks.ClientSetting{
				Address: address,
			}),
			TCPDialFunc: tcp.Dial,
			UDPDialFunc: udp.Dial,
		})

		if v.Mux {
			handler = mux.NewClient(handler)
		}

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Tor {
		dialer, err := loader.BuildTorClient()
		if err != nil {
			return err
		}

		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: tor.NewClient(tor.ClientSetting{
				Dialer: dialer,
			}),
			TCPDialFunc: tcp.Dial,
			TCPForwardDialFunc: func() outbound.ForwardDialTCPFunc {
				if len(v.Forward.Tag) > 0 {
					return loader.NewForwardDialTCPFunc(loader.OutboundForwardDialTCPFuncSetting{
						Handlers: loader.RequireInstance().OutboundManager,
						Tag:      v.Forward.Tag,
						TLSConfig: func() tls.Config {
							if len(v.Forward.Tls.ServerName) > 0 {
								return loader.BuildTLSetting(loader.TLSetting{
									ServerName: v.Forward.Tls.ServerName,
								})
							}
							return tls.Config{}
						}(),
					})
				}
				return nil
			}(),
			UDPDialFunc: udp.Dial,
		})

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Trojan {
		address, err := net.ParseAddress(net.Network_TCP, v.Target)
		if err != nil {
			return err
		}

		user := loader.BuildTrojanUser(loader.TrojanUserSetting{
			Password: v.User.Password,
		})

		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: trojan.NewClient(trojan.ClientSetting{
				Address: address,
				User:    user,
			}),
			TCPDialFunc: func() internet.DialTCPFunc {
				if len(v.Websocket.Path) > 0 {
					if len(v.Websocket.Tls.ServerName) > 0 {
						return tls.Dial(tls.DialSetting{
							Config: loader.BuildTLSetting(loader.TLSetting{
								ServerName: v.Websocket.Tls.ServerName,
							}),
						}, websocket.Dial(websocket.DialSetting{
							Path: v.Websocket.Path,
							TLSConfig: loader.BuildTLSetting(loader.TLSetting{
								ServerName: v.Websocket.Tls.ServerName,
							}),
						}, tcp.Dial))
					}
					return websocket.Dial(websocket.DialSetting{
						Path: v.Websocket.Path,
					}, tcp.Dial)
				}
				if len(v.Tcp.Tls.ServerName) > 0 {
					return tls.Dial(tls.DialSetting{
						Config: loader.BuildTLSetting(loader.TLSetting{
							ServerName: v.Tcp.Tls.ServerName,
						}),
					}, tcp.Dial)
				}
				return tcp.Dial
			}(),
			TCPForwardDialFunc: func() outbound.ForwardDialTCPFunc {
				if len(v.Forward.Tag) > 0 {
					return loader.NewForwardDialTCPFunc(loader.OutboundForwardDialTCPFuncSetting{
						Handlers: loader.RequireInstance().OutboundManager,
						Tag:      v.Forward.Tag,
						TLSConfig: func() tls.Config {
							if len(v.Forward.Tls.ServerName) > 0 {
								return loader.BuildTLSetting(loader.TLSetting{
									ServerName: v.Forward.Tls.ServerName,
								})
							}
							return tls.Config{}
						}(),
					})
				}
				return nil
			}(),
			UDPDialFunc: udp.Dial,
		})

		if v.Mux {
			handler = mux.NewClient(handler)
		}

		loader.RegisterOutboundHandler(handler)
	}

	for _, v := range c.Outbounds.Vmess {
		address, err := net.ParseAddress(net.Network_TCP, v.Target)
		if err != nil {
			return err
		}

		user, err := loader.BuildVmessUser(loader.VmessUserSetting{
			Security: v.User.Security,
			UUID:     v.User.UUID,
		})
		if err != nil {
			return err
		}

		handler := outbound.NewOutbound(outbound.Setting{
			Tag: v.Tag,
			Client: vmess.NewClient(vmess.ClientSetting{
				Address: address,
				User:    user,
			}),
			TCPDialFunc: func() internet.DialTCPFunc {
				if len(v.Websocket.Path) > 0 {
					if len(v.Websocket.Tls.ServerName) > 0 {
						return tls.Dial(tls.DialSetting{
							Config: loader.BuildTLSetting(loader.TLSetting{
								ServerName: v.Websocket.Tls.ServerName,
							}),
						}, websocket.Dial(websocket.DialSetting{
							Path: v.Websocket.Path,
							TLSConfig: loader.BuildTLSetting(loader.TLSetting{
								ServerName: v.Websocket.Tls.ServerName,
							}),
						}, tcp.Dial))
					}
					return websocket.Dial(websocket.DialSetting{
						Path: v.Websocket.Path,
					}, tcp.Dial)
				}
				if len(v.Tcp.Tls.ServerName) > 0 {
					return tls.Dial(tls.DialSetting{
						Config: loader.BuildTLSetting(loader.TLSetting{
							ServerName: v.Tcp.Tls.ServerName,
						}),
					}, tcp.Dial)
				}
				return tcp.Dial
			}(),
			TCPForwardDialFunc: func() outbound.ForwardDialTCPFunc {
				if len(v.Forward.Tag) > 0 {
					return loader.NewForwardDialTCPFunc(loader.OutboundForwardDialTCPFuncSetting{
						Handlers: loader.RequireInstance().OutboundManager,
						Tag:      v.Forward.Tag,
						TLSConfig: func() tls.Config {
							if len(v.Forward.Tls.ServerName) > 0 {
								return loader.BuildTLSetting(loader.TLSetting{
									ServerName: v.Forward.Tls.ServerName,
								})
							}
							return tls.Config{}
						}(),
					})
				}
				return nil
			}(),
			UDPDialFunc: udp.Dial,
		})

		if v.Mux {
			handler = mux.NewClient(handler)
		}

		loader.RegisterOutboundHandler(handler)
	}

	return nil
}

func (c config) LoadDNS() error {
	providers := make([]dns_app.ConditionProvider, 0)

	{
		hosts := func() dns_app.Provider {
			setting := make([]loader.HostSetting, 0)

			for _, v := range c.Dns.Hosts {
				setting = append(setting, loader.HostSetting{
					Domain: v.Domain,
					Hosts6: v.Ips6,
					Hosts4: v.Ips4,
				})
			}

			return loader.BuildHostsDNS(setting...)
		}()

		providers = append(providers, dns_app.ConditionProvider{
			Provider: hosts,
			Tag:      preservedHosts,
		})
	}

	{
		for _, v := range c.Dns.Fake {
			if tag := v.Tag; len(tag) > 0 {
				fake, err := loader.BuildFakeDNS(loader.FakeDNSetting{
					Cidr6: v.Cidr6,
					Cidr4: v.Cidr4,
				})
				if err != nil {
					return err
				}

				providers = append(providers, dns_app.ConditionProvider{
					Provider: fake,
					Tag:      tag,
				})
			}
		}
	}

	{
		for _, v := range c.Dns.Doh {
			doh, err := loader.BuildDoHDNS(loader.DoHSetting{
				Url: v.Url,
				Tag: v.Tag,
			})
			if err != nil {
				return err
			}

			providers = append(providers, dns_app.ConditionProvider{
				Provider: doh,
				Tag:      v.Tag,
			})
		}
	}

	loader.RegisterNameserver(providers)

	loader.RegisterPreservedNameserver(preservedDNS)

	return nil
}

func (c config) LoadRouter() error {
	rules1 := make([]router_common.Rule, 0)
	rules2 := make([]router_common.Rule, 0)

	{
		domains := func() []string {
			domains := make([]string, 0)

			for _, v := range c.Dns.Hosts {
				domains = append(domains, v.Domain)
			}

			return domains
		}()

		rules1 = append(rules1, router_common.Rule{
			Condition:   AnyConditionDNS(domains),
			OutboundTag: preservedHosts,
		})
	}

	for _, v := range c.Rules.Dns {
		{
			cc := make(router_common.Condition, 0)

			for _, cond := range v.Condition {
				sc, err := loader.ParseDefaultCondition(loader.ParseDefaultLookupConditionName, loader.DefaultConditionSetting{
					Name:   cond.Name,
					Length: cond.Length,
					String: cond.String,
				})
				if err != nil {
					return err
				}

				cc = append(cc, sc)
			}

			rules1 = append(rules1, router_common.Rule{
				Condition:   cc,
				OutboundTag: v.OutboundTag,
			})
		}
	}

	for _, v := range c.Rules.Outbound {
		{
			cc := make(router_common.Condition, 0)

			for _, cond := range v.Condition {
				str2 := make([]string, 0)
				cidr := make([]netip.Prefix, 0)

				for _, s := range cond.String {
					if ip := strings.TrimPrefix(s, strCIDRip); len(ip) < len(s) {
						ips, err := geofile.LoadIPCIDR([]string{ip})
						if err != nil {
							return err
						}
						cidr = append(cidr, ips...)
					} else if ip := strings.TrimPrefix(s, strGeoip); len(ip) < len(s) {
						ips0, err := geofile.LoadIPStr(ip)
						if err != nil {
							return err
						}
						ips, err := geofile.LoadIPCIDR(ips0)
						if err != nil {
							return err
						}
						cidr = append(cidr, ips...)
					} else if site := strings.TrimPrefix(s, strGeosite); len(site) < len(s) {
						sites, err := geofile.LoadSite(site)
						if err != nil {
							return err
						}
						str2 = append(str2, sites...)
					} else {
						str2 = append(str2, s)
					}
				}

				sc, err := loader.ParseDefaultCondition(loader.ParseDefaultContentConditionName, loader.DefaultConditionSetting{
					Name:   cond.Name,
					Length: cond.Length,
					String: str2,
					CIDR:   cidr,
				})
				if err != nil {
					return err
				}

				cc = append(cc, sc)
			}

			rules2 = append(rules2, router_common.Rule{
				Condition:   cc,
				OutboundTag: v.OutboundTag,
			})
		}
	}

	m1 := router_app.NewMatcher(rules1...)
	loader.RegisterNameserverMatcher(m1)

	m2 := router_app.NewMatcher(rules2...)
	loader.RegisterOutboundMatcher(m2)

	return nil
}
