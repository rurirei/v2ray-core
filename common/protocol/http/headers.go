package http

import (
	"net/http"
	"strings"

	"v2ray.com/core/common/net"
)

const (
	Scheme_HTTP  = "http"
	Scheme_HTTPS = "https"

	Port_HTTP  = 80
	Port_HTTPS = 443
)

// ParseXRealIP parses X-Forwarded-For header in http headers, and return the IP list in it.
func ParseXRealIP(header http.Header) []string {
	return parseXForwardedHeader(header.Get("X-Real-Ip"))
}

// ParseXForwardedFor parses X-Forwarded-For header in http headers, and return the IP list in it.
func ParseXForwardedFor(header http.Header) []string {
	return parseXForwardedHeader(header.Get("X-Forwarded-For"))
}

func parseXForwardedHeader(xff string) []string {
	if len(xff) == 0 {
		return nil
	}
	list := strings.Split(xff, ",")
	addrs := make([]string, 0, len(list))
	for _, proxy := range list {
		addrs = append(addrs, proxy)
	}
	return addrs
}

// RemoveHopByHopHeaders remove hop by hop headers in http header list.
func RemoveHopByHopHeaders(header http.Header) {
	// Strip hop-by-hop header based on RFC:
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html#sec13.5.1
	// https://www.mnot.net/blog/2011/07/11/what_proxies_must_do

	header.Del("Client-Connection")
	header.Del("Client-Authenticate")
	header.Del("Client-Authorization")
	header.Del("TE")
	header.Del("Trailers")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")

	connections := header.Get("Connection")
	header.Del("Connection")
	if len(connections) == 0 {
		return
	}
	for _, h := range strings.Split(connections, ",") {
		header.Del(strings.TrimSpace(h))
	}
}

// ParseHostPort splits host and port from a raw string. Default port is used when raw string doesn't contain port.
func ParseHostPort(host, scheme string) string {
	defaultPort := func() net.Port {
		switch scheme {
		case Scheme_HTTPS:
			return net.Port(Port_HTTPS)
		case Scheme_HTTP:
			return net.Port(Port_HTTP)
		default:
			return net.Port(net.AnyPort)
		}
	}()

	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, defaultPort.String())
	}

	return host
}
