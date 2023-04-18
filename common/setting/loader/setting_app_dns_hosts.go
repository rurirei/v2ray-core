package loader

import (
	dns_app "v2ray.com/core/app/dns"
	"v2ray.com/core/common/net"
)

type HostSetting struct {
	Domain         string
	Hosts6, Hosts4 []string
}

func BuildHostsDNS(setting ...HostSetting) dns_app.Provider {
	toIPs := func(ips []string) []net.IP {
		ips2 := make([]net.IP, 0)
		for _, ip := range ips {
			ips2 = append(ips2, net.ParseIP(ip))
		}
		return ips2
	}

	hosts := make([]dns_app.HostsDNSetting, 0)

	for _, v := range setting {
		hosts = append(hosts, dns_app.HostsDNSetting{
			Domain: v.Domain,
			Hosts6: toIPs(v.Hosts6),
			Hosts4: toIPs(v.Hosts4),
		})
	}

	return dns_app.NewHostsDNS(hosts...)
}
