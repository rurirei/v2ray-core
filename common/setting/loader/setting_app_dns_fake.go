package loader

import (
	"net/netip"

	dns_app "v2ray.com/core/app/dns"
	sniffer_app "v2ray.com/core/app/sniffer"
	"v2ray.com/core/common/protocol/fakedns"
)

type FakeDNSetting struct {
	Cidr6, Cidr4 string
}

func BuildFakeDNS(setting FakeDNSetting) (dns_app.Provider, error) {
	iprange6, err := netip.ParsePrefix(setting.Cidr6)
	if err != nil {
		return nil, err
	}

	iprange4, err := netip.ParsePrefix(setting.Cidr4)
	if err != nil {
		return nil, err
	}

	fake := dns_app.NewFakeDNS(dns_app.FakeDNSetting{
		IPRange6: iprange6,
		IPRange4: iprange4,
	})

	sniffer_app.RegisterFakeSniffer(fakedns.NewSniffer(dns_app.NewFakePool(fake)))

	return fake, nil
}
