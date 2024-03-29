package tls

import (
	"encoding/binary"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/dns"
	"v2ray.com/core/common/protocol/sniffer"
)

var (
	errNotTLS         = newError("not TLS header")
	errNotClientHello = newError("not client hello")

	// errNoClue is for the situation that existing information is not enough to make a decision. For example, Router may return this error when there is no suitable route.
	errNoClue = newError("not enough information for making a decision")
)

type tlsSniffer struct {
}

func NewSniffer() sniffer.Sniffer {
	return &tlsSniffer{}
}

func (s *tlsSniffer) Protocol() sniffer.Protocol {
	return sniffer.TLS
}

func (s *tlsSniffer) Sniff(b *buffer.Buffer, _ net.IP) (sniffer.SniffResult, error) {
	data, err := beginWithTLS(b.Bytes())
	if err != nil {
		return sniffer.SniffResult{}, err
	}

	domain, err := ReadClientHello(data)
	if err != nil {
		return sniffer.SniffResult{}, err
	}

	return sniffer.SniffResult{
		Protocol: s.Protocol(),
		Domain:   domain,
	}, nil
}

// ReadClientHello returns server name (if any) from TLS client hello message.
// https://github.com/golang/go/blob/master/src/crypto/tls/handshake_messages.go#L300
func ReadClientHello(data []byte) (string, error) {
	if len(data) < 42 {
		return "", errNoClue
	}

	sessionIDLen := int(data[38])
	if sessionIDLen > 32 || len(data) < 39+sessionIDLen {
		return "", errNoClue
	}
	data = data[39+sessionIDLen:]
	if len(data) < 2 {
		return "", errNoClue
	}

	// cipherSuiteLen is the number of bytes of cipher suite numbers. Since
	// they are uint16s, the number must be even.
	cipherSuiteLen := int(data[0])<<8 | int(data[1])
	if cipherSuiteLen%2 == 1 || len(data) < 2+cipherSuiteLen {
		return "", errNotClientHello
	}
	data = data[2+cipherSuiteLen:]
	if len(data) < 1 {
		return "", errNoClue
	}

	compressionMethodsLen := int(data[0])
	if len(data) < 1+compressionMethodsLen {
		return "", errNoClue
	}
	data = data[1+compressionMethodsLen:]
	if len(data) == 0 {
		return "", errNotClientHello
	}
	if len(data) < 2 {
		return "", errNotClientHello
	}

	extensionsLength := int(data[0])<<8 | int(data[1])
	data = data[2:]
	if extensionsLength != len(data) {
		return "", errNotClientHello
	}

	for len(data) != 0 {
		if len(data) < 4 {
			return "", errNotClientHello
		}

		extension := uint16(data[0])<<8 | uint16(data[1])
		length := int(data[2])<<8 | int(data[3])
		data = data[4:]
		if len(data) < length {
			return "", errNotClientHello
		}

		if extension == 0x00 { /* extensionServerName */
			d := data[:length]
			if len(d) < 2 {
				return "", errNotClientHello
			}

			namesLen := int(d[0])<<8 | int(d[1])
			d = d[2:]
			if len(d) != namesLen {
				return "", errNotClientHello
			}

			for len(d) > 0 {
				if len(d) < 3 {
					return "", errNotClientHello
				}

				nameType := d[0]
				nameLen := int(d[1])<<8 | int(d[2])
				d = d[3:]
				if len(d) < nameLen {
					return "", errNotClientHello
				}

				if nameType == 0 {
					serverName := string(d[:nameLen])
					// An SNI value may not include a
					// trailing dot. See
					// https://tools.ietf.org/html/rfc6066#section-3.
					if dns.IsFqdn(serverName) {
						return "", errNotClientHello
					}
					return serverName, nil
				}

				d = d[nameLen:]
			}
		}
		data = data[length:]
	}

	return "", errNotTLS
}

func beginWithTLS(b []byte) ([]byte, error) {
	if len(b) < 5 {
		return nil, errNoClue
	}

	if b[0] != 0x16 /* TLS Handshake */ {
		return nil, errNotTLS
	}

	if !IsValidTLSVersion(b[1], b[2]) {
		return nil, errNotTLS
	}

	headerLen := int(binary.BigEndian.Uint16(b[3:5]))
	if 5+headerLen > len(b) {
		return nil, errNoClue
	}

	return b[5 : 5+headerLen], nil
}

func IsValidTLSVersion(major, minor byte) bool {
	return major == 3
}
