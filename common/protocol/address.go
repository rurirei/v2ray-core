package protocol

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/serial"
)

type AddressOption func(*option)

func PortThenAddress() AddressOption {
	return func(p *option) {
		p.portFirst = true
	}
}

func AddressFamilyByte(b byte, f net.HostLength) AddressOption {
	if b >= 16 {
		panic("address family byte too big")
	}
	return func(p *option) {
		p.hostTypeMap[b] = f
		p.hostByteMap[f] = b
	}
}

type AddressTypeParser func(byte) byte

func WithAddressTypeParser(atp AddressTypeParser) AddressOption {
	return func(p *option) {
		p.typeParser = atp
	}
}

type AddressSerializer interface {
	ReadAddress(*buffer.Buffer, io.Reader) (net.Address, error)

	WriteAddress(io.Writer, net.Address) error
}

const afInvalid = 255

type option struct {
	hostTypeMap [16]net.HostLength
	hostByteMap [16]byte
	portFirst   bool
	typeParser  AddressTypeParser
}

// NewAddressParser creates a new AddressParser
func NewAddressParser(options ...AddressOption) AddressSerializer {
	var o option
	for i := range o.hostByteMap {
		o.hostByteMap[i] = afInvalid
	}
	for i := range o.hostTypeMap {
		o.hostTypeMap[i] = net.HostLength(afInvalid)
	}
	for _, opt := range options {
		opt(&o)
	}

	ap := &hostParser{
		hostByteMap: o.hostByteMap,
		hostTypeMap: o.hostTypeMap,
	}

	if o.typeParser != nil {
		ap.typeParser = o.typeParser
	}

	if o.portFirst {
		return portFirstAddressParser{ap: ap}
	}

	return portLastAddressParser{ap: ap}
}

type portFirstAddressParser struct {
	ap *hostParser
}

func (p portFirstAddressParser) ReadAddress(buf *buffer.Buffer, input io.Reader) (net.Address, error) {
	if buf == nil {
		buf = buffer.New()
		defer buf.Release()
	}

	port, err := readPort(buf, input)
	if err != nil {
		return net.Address{}, err
	}

	host, err := p.ap.readHost(buf, input)
	if err != nil {
		return net.Address{}, err
	}

	host.Port = port.Port
	return host, nil
}

func (p portFirstAddressParser) WriteAddress(writer io.Writer, address net.Address) error {
	if err := writePort(writer, address); err != nil {
		return err
	}

	if err := p.ap.writeHost(writer, address); err != nil {
		return err
	}

	return nil
}

type portLastAddressParser struct {
	ap *hostParser
}

func (p portLastAddressParser) ReadAddress(buf *buffer.Buffer, input io.Reader) (net.Address, error) {
	if buf == nil {
		buf = buffer.New()
		defer buf.Release()
	}

	host, err := p.ap.readHost(buf, input)
	if err != nil {
		return net.Address{}, err
	}

	port, err := readPort(buf, input)
	if err != nil {
		return net.Address{}, err
	}

	host.Port = port.Port
	return host, nil
}

func (p portLastAddressParser) WriteAddress(writer io.Writer, address net.Address) error {
	if err := p.ap.writeHost(writer, address); err != nil {
		return err
	}

	if err := writePort(writer, address); err != nil {
		return err
	}

	return nil
}

func readPort(b *buffer.Buffer, reader io.Reader) (net.Address, error) {
	if _, err := b.ReadFullFrom(reader, 2); err != nil {
		return net.Address{}, err
	}

	return net.Address{
		Port: net.PortFromBytes(b.BytesFrom(-2)),
	}, nil
}

func writePort(writer io.Writer, port net.Address) error {
	_, err := serial.WriteUint16(writer, port.Port.This())
	return err
}

func maybeIPPrefix(b byte) bool {
	return b == '[' || (b >= '0' && b <= '9')
}

func isValidDomain(d string) bool {
	for _, c := range d {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '.' || c == '_') {
			return false
		}
	}
	return true
}

type hostParser struct {
	hostTypeMap [16]net.HostLength
	hostByteMap [16]byte
	typeParser  AddressTypeParser
}

func (p *hostParser) readHost(b *buffer.Buffer, reader io.Reader) (net.Address, error) {
	if _, err := b.ReadFullFrom(reader, 1); err != nil {
		return net.Address{}, err
	}

	hostType := b.Byte(b.Len() - 1)
	if p.typeParser != nil {
		hostType = p.typeParser(hostType)
	}

	if hostType >= 16 {
		return net.Address{}, newError("unknown address type: ", hostType)
	}

	hostFamily := p.hostTypeMap[hostType]
	if hostFamily == net.HostLength(afInvalid) {
		return net.Address{}, newError("unknown address type: ", hostType)
	}

	switch hostFamily {
	case net.HostLengthIPv4:
		if _, err := b.ReadFullFrom(reader, 4); err != nil {
			return net.Address{}, err
		}
		return net.ParseHost(net.ByteToIP(b.BytesFrom(-4)).String())
	case net.HostLengthIPv6:
		if _, err := b.ReadFullFrom(reader, 16); err != nil {
			return net.Address{}, err
		}
		return net.ParseHost(net.ByteToIP(b.BytesFrom(-16)).String())
	case net.HostLengthDomain:
		if _, err := b.ReadFullFrom(reader, 1); err != nil {
			return net.Address{}, err
		}
		domainLength := int32(b.Byte(b.Len() - 1))
		if _, err := b.ReadFullFrom(reader, int(domainLength)); err != nil {
			return net.Address{}, err
		}
		domain := string(b.BytesFrom(int(-domainLength)))
		host, err := net.ParseHost(domain)
		if err != nil {
			return net.Address{}, err
		}
		if maybeIPPrefix(domain[0]) {
			if host.IsIPHost() {
				return host, nil
			}
		}
		if !isValidDomain(domain) {
			return net.Address{}, newError("invalid domain name: ", domain)
		}
		return host, nil
	default:
		return net.Address{}, common.ErrUnknownNetwork
	}
}

func (p *hostParser) writeHost(writer io.Writer, host net.Address) error {
	tb := p.hostByteMap[host.HostLength()]
	if tb == afInvalid {
		return newError("unknown host family", host.HostLength())
	}

	switch host.HostLength() {
	case net.HostLengthIPv4, net.HostLengthIPv6:
		if _, err := writer.Write([]byte{tb}); err != nil {
			return err
		}
		if _, err := writer.Write(host.IP); err != nil {
			return err
		}
	case net.HostLengthDomain:
		if isDomainTooLong(host.Domain.This()) {
			return newError("Super long domain is not supported: %s", host.Domain.This())
		}

		if _, err := writer.Write([]byte{tb, byte(len(host.Domain))}); err != nil {
			return err
		}
		if _, err := writer.Write([]byte(host.Domain)); err != nil {
			return err
		}
	default:
		return common.ErrUnknownNetwork
	}

	return nil
}
