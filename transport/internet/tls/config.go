package tls

import (
	"crypto/tls"
)

var (
	ClientOption = struct {
		NextProtos    []string
		AllowInsecure bool
	}{
		NextProtos:    []string{"h2", "http/1.1"},
		AllowInsecure: false,
	}
)

type Config struct {
	// Override server name.
	ServerName string
	// Certificate to be served on server.
	Certificate Certificate
}

func (c Config) BuildTLSClient() *tls.Config {
	return &tls.Config{
		ServerName:         c.ServerName,
		InsecureSkipVerify: ClientOption.AllowInsecure,
		NextProtos:         ClientOption.NextProtos,
	}
}

func (c Config) BuildTLSServer() (*tls.Config, error) {
	certificate, err := c.BuildCertificate()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}, nil
}

func (c Config) BuildCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair(c.Certificate.Certificate, c.Certificate.Key)
}

type Certificate struct {
	// TLS certificate in x509 format.
	Certificate []byte
	// TLS key in x509 format.
	Key []byte
}
