package loader

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
)

func RegisterPreservedNameserver(tag string) {
	net.LocalLookupIPFunc = func(network, host string) ([]net.IP, error) {
		// ip filter as unsupported
		if ip := net.ParseIP(host); len(ip) > 0 {
			return []net.IP{ip}, nil
		}

		return RequireInstance().Nameserver.LookupIPCondition(network, host, session.Lookup{
			Domain:     host,
			InboundTag: tag,
		})
	}
}
