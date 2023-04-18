package loader

import (
	dns_app "v2ray.com/core/app/dns"
)

func RegisterNameserver(providers []dns_app.ConditionProvider) {
	localInstance.Nameserver = dns_app.NewConditionServer(localInstance.NameserverMatcher, providers...)
}
