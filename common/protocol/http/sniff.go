package http

import (
	"bytes"
	"strings"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/sniffer"
)

var (
	methods = []string{"get", "post", "head", "put", "delete", "options", "connect", "patch", "trace"}

	errNotHTTPMethod = newError("not an HTTP method")

	// errNoClue is for the situation that existing information is not enough to make a decision. For example, Router may return this error when there is no suitable route.
	errNoClue = newError("not enough information for making a decision")
)

type httpSniffer struct {
}

func NewSniffer() sniffer.Sniffer {
	return &httpSniffer{}
}

func (s *httpSniffer) Protocol() sniffer.Protocol {
	return sniffer.HTTP
}

func (s *httpSniffer) Sniff(b *buffer.Buffer, _ net.IP) (sniffer.SniffResult, error) {
	b0 := b.Bytes()

	if err := beginWithHTTPMethod(b0); err != nil {
		return sniffer.SniffResult{}, err
	}

	domain, err := readRawHost(b0)
	if err != nil {
		return sniffer.SniffResult{}, err
	}

	return sniffer.SniffResult{
		Protocol: s.Protocol(),
		Domain:   domain,
	}, nil
}

func readRawHost(b []byte) (string, error) {
	headers := bytes.Split(b, []byte{'\n'})

	for _, header := range headers {
		parts := bytes.SplitN(header, []byte{':'}, 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(string(parts[0]))
		if key == "host" {
			rawHost := strings.ToLower(string(bytes.TrimSpace(parts[1])))
			if host, _, err := net.SplitHostPort(rawHost); err == nil {
				return host, nil
			}
			return rawHost, nil
		}
	}

	return "", errNoClue
}

func beginWithHTTPMethod(b []byte) error {
	for _, m := range methods {
		if len(b) >= len(m) && strings.EqualFold(string(b[:len(m)]), m) {
			return nil
		}

		if len(b) < len(m) {
			return errNoClue
		}
	}

	return errNotHTTPMethod
}
