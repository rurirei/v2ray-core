package dns

import (
	"bytes"
	"context"
	"net/http"
	"net/url"

	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
)

type DoHDNSetting struct {
	Url      *url.URL
	DialFunc net.DialFunc
}

// dohDNS implemented DNS over HTTPS (RFC8484) Wire Format,
// which is compatible with traditional dns over udp(RFC1035),
// thus most of the DOH implementation is copied from udpns.go
type dohDNS struct {
	url    *url.URL
	client *http.Client

	base *baseDNS
}

func NewDoHDNS(setting DoHDNSetting) Provider {
	return &dohDNS{
		url: setting.Url,
		client: &http.Client{
			Transport: &http.Transport{
				ForceAttemptHTTP2: false,
				DialContext: func(_ context.Context, network, address string) (net.Conn, error) {
					return setting.DialFunc(network, address)
				},
			},
		},
		base: newBaseDNS(),
	}
}

func (p *dohDNS) LookupIP(network, host string) ([]net.IP, error) {
	return p.base.LookupIP(network, host, p.dohHTTPSContext)
}

func (p *dohDNS) dohHTTPSContext(b []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, p.url.String(), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/dns-message")
	req.Header.Add("Content-Type", "application/dns-message")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(resp.Body)
	default:
		_, err = io.Discard(resp.Body) // flush resp.Body so that the conn is reusable
		return nil, newError("DOH server returned %d", resp.StatusCode).WithError(err)
	}
}

func (p *dohDNS) Name() string {
	return "DoH DNS"
}
