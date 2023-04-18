package dns

import (
	router_app "v2ray.com/core/app/router"
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/session"
)

type ConditionServer interface {
	LookupIPv6Condition(string, string, session.Lookup) ([]net.IP, error)
	LookupIPv4Condition(string, string, session.Lookup) ([]net.IP, error)
	LookupIPCondition(string, string, session.Lookup) ([]net.IP, error)
	Names() []string
}

type Server interface {
	LookupIPv6(string, string) ([]net.IP, error)
	LookupIPv4(string, string) ([]net.IP, error)
	LookupIP(string, string) ([]net.IP, error)
	Names() []string
}

type server struct {
	matcher   router_app.Matcher
	providers map[string]Provider
}

func NewConditionServer(matcher router_app.Matcher, providers ...ConditionProvider) ConditionServer {
	providers0 := make(map[string]Provider, len(providers))

	for _, provider := range providers {
		providers0[provider.Tag] = provider.Provider
	}

	return &server{
		matcher:   matcher,
		providers: providers0,
	}
}

func NewServer(providers ...Provider) Server {
	providers0 := make(map[string]Provider, len(providers))

	for i, provider := range providers {
		providers0[string(byte(i))] = provider
	}

	return &server{
		providers: providers0,
	}
}

func (s *server) LookupIPv6Condition(network, host string, lookup session.Lookup) ([]net.IP, error) {
	ips6, _, err := s.lookupIP(network, host, s.matchConditionProviders(lookup))
	if err != nil {
		return nil, err
	}
	return ips6, nil
}

func (s *server) LookupIPv4Condition(network, host string, lookup session.Lookup) ([]net.IP, error) {
	_, ips4, err := s.lookupIP(network, host, s.matchConditionProviders(lookup))
	if err != nil {
		return nil, err
	}
	return ips4, nil
}

func (s *server) LookupIPCondition(network, host string, lookup session.Lookup) ([]net.IP, error) {
	ips6, ips4, err := s.lookupIP(network, host, s.matchConditionProviders(lookup))
	if err != nil {
		return nil, err
	}
	return append(ips6, ips4...), nil
}

func (s *server) matchConditionProviders(lookup session.Lookup) []Provider {
	providers2 := make([]Provider, 0, 1)

	if tag, ok := s.matcher.MatchLookup(lookup); ok {
		providers2 = append(providers2, s.providers[tag])
	}

	return providers2
}

func (s *server) LookupIPv6(network, host string) ([]net.IP, error) {
	ips6, _, err := s.lookupIP(network, host, s.getAllProviders())
	if err != nil {
		return nil, err
	}
	return ips6, nil
}

func (s *server) LookupIPv4(network, host string) ([]net.IP, error) {
	_, ips4, err := s.lookupIP(network, host, s.getAllProviders())
	if err != nil {
		return nil, err
	}
	return ips4, nil
}

func (s *server) LookupIP(network, host string) ([]net.IP, error) {
	ips6, ips4, err := s.lookupIP(network, host, s.getAllProviders())
	if err != nil {
		return nil, err
	}
	return append(ips6, ips4...), nil
}

func (s *server) getAllProviders() []Provider {
	providers2 := make([]Provider, 0, len(s.providers))

	for _, provider := range s.providers {
		providers2 = append(providers2, provider)
	}

	return providers2
}

func (s *server) lookupIP(network, host string, providers []Provider) ([]net.IP, []net.IP, error) {
	errs := make([]error, 0, len(providers))

	for _, provider := range providers {
		ips, err := provider.LookupIP(network, host)
		if err == nil {
			var ips6, ips4 []net.IP

			for _, ip := range ips {
				switch len(ip) {
				case net.IPv6len:
					ips6 = append(ips6, ip)
				case net.IPv4len:
					ips4 = append(ips4, ip)
				default:
					return nil, nil, common.ErrUnknownNetwork
				}
			}

			newError("nameserver ip lookup result %v of [%s] via [%s]", ips, host, provider.Name()).AtInfo().Logging()
			return ips6, ips4, nil
		}

		errs = append(errs, err)
	}

	return nil, nil, newError("all retries failed").WithError(errs)
}

func (s *server) Names() []string {
	var names []string
	for _, provider := range s.providers {
		names = append(names, provider.Name())
	}

	return names
}
