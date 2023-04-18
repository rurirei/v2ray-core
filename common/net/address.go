package net

import (
	"bytes"
	"encoding/binary"
	"strconv"

	"v2ray.com/core/common"
)

var (
	LocalhostIPv4 = ByteToIP([]byte{127, 0, 0, 1})
	LocalhostIPv6 = ByteToIP([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})

	AnyIPv4 = ByteToIP([]byte{0, 0, 0, 0})
	AnyIPv6 = ByteToIP([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	AnyPort = uint16(0)
)

var (
	LocalhostTCPAddr = &TCPAddr{
		IP:   LocalhostIPv4,
		Port: int(AnyPort),
	}

	LocalhostUDPAddr = &UDPAddr{
		IP:   LocalhostIPv4,
		Port: int(AnyPort),
	}

	AnyTCPAddr = &TCPAddr{
		IP:   AnyIPv4,
		Port: int(AnyPort),
	}

	AnyUDPAddr = &UDPAddr{
		IP:   AnyIPv4,
		Port: int(AnyPort),
	}
)

var (
	LocalhostTCPAddress = Address{
		IP:      LocalhostIPv4,
		Domain:  EmptyDomain,
		Port:    Port(AnyPort),
		Network: Network_TCP,
	}

	LocalhostUDPAddress = Address{
		IP:      LocalhostIPv6,
		Domain:  EmptyDomain,
		Port:    Port(AnyPort),
		Network: Network_UDP,
	}

	AnyTCPAddress = Address{
		IP:      AnyIPv4,
		Domain:  EmptyDomain,
		Port:    Port(AnyPort),
		Network: Network_TCP,
	}

	AnyUDPAddress = Address{
		IP:      AnyIPv4,
		Domain:  EmptyDomain,
		Port:    Port(AnyPort),
		Network: Network_UDP,
	}
)

const (
	EmptyDomain = ""
	EmptyIP     = ""

	Network_TCP  = "tcp"
	Network_TCP6 = "tcp6"
	Network_TCP4 = "tcp4"

	Network_UDP  = "udp"
	Network_UDP6 = "udp6"
	Network_UDP4 = "udp4"

	Network_Unix = "unix"

	Network_IP  = "ip"
	Network_IP6 = "ip6"
	Network_IP4 = "ip4"
)

type HostLength byte

const (
	HostLengthIPv4 HostLength = iota
	HostLengthIPv6
	HostLengthDomain
)

// ByteToIP creates IP from []byte
func ByteToIP(ip []byte) IP {
	switch len(ip) {
	case IPv4len:
		return IP{ip[0], ip[1], ip[2], ip[3]}
	case IPv6len:
		var bytes0 = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		if bytes.Equal(ip[:10], bytes0) && ip[10] == 0xff && ip[11] == 0xff {
			return ByteToIP(ip[12:16])
		}
		return IP{ip[0], ip[1], ip[2], ip[3], ip[4], ip[5], ip[6], ip[7], ip[8], ip[9], ip[10], ip[11], ip[12], ip[13], ip[14], ip[15]}
	default:
		return IP(nil)
	}
}

type Domain string

func (domain Domain) This() string {
	return string(domain)
}

func (domain Domain) IsValid() bool {
	return len(domain) > 0
}

type Port uint16

func (port Port) This() uint16 {
	return uint16(port)
}

func (port Port) IsValid() bool {
	return port > 0
}

func (port Port) String() string {
	return strconv.Itoa(int(port))
}

// PortFromBytes converts a byte array to a Port, assuming bytes are in big endian order.
// @unsafe Caller must ensure that the byte array has at least 2 elements.
func PortFromBytes(port []byte) Port {
	return Port(binary.BigEndian.Uint16(port))
}

type Network string

func (network Network) This() string {
	return string(network)
}

func (network Network) IsValid() bool {
	switch network {
	case Network_TCP, Network_UDP:
		return true
	default:
		return false
	}
}

type Address struct {
	IP      IP      // IP{1.2.3.4}
	Domain  Domain  // "example.com"
	Port    Port    // uint16(443)
	Network Network // "tcp"
}

func (a Address) Equal(b Address) bool {
	return a.NetworkAndDomainPreferredAddress() == b.NetworkAndDomainPreferredAddress()
}

func (a Address) HostLength() HostLength {
	switch len(a.IP) {
	case IPv4len:
		return HostLengthIPv4
	case IPv6len:
		return HostLengthIPv6
	default:
		return HostLengthDomain
	}
}

func (a Address) IsIPv4Host() bool {
	return a.HostLength() == HostLengthIPv4
}

func (a Address) IsIPv6Host() bool {
	return a.HostLength() == HostLengthIPv6
}

func (a Address) IsIPHost() bool {
	return a.IsIPv4Host() || a.IsIPv6Host()
}

func (a Address) IsDomainHost() bool {
	return a.HostLength() == HostLengthDomain
}

func (a Address) IsFullHost() bool {
	return a.IsIPHost() && a.IsDomainHost()
}

func (a Address) IsOneHost() bool {
	return a.IsIPHost() || a.IsDomainHost()
}

func (a Address) IPHostString() string {
	return a.IP.String()
}

func (a Address) DomainHostString() string {
	return a.Domain.This()
}

func (a Address) IPPreferredHostString() string {
	switch {
	case a.IsIPHost():
		return a.IPHostString()
	case a.IsDomainHost():
		return a.DomainHostString()
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (a Address) DomainPreferredHostString() string {
	switch {
	case a.IsDomainHost():
		return a.DomainHostString()
	case a.IsIPHost():
		return a.IPHostString()
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (a Address) IPAddress() string {
	return JoinHostPort(a.IP.String(), a.Port.String())
}

func (a Address) DomainAddress() string {
	return JoinHostPort(a.Domain.This(), a.Port.String())
}

func (a Address) IPPreferredAddress() string {
	switch {
	case a.IsIPHost():
		return a.IPAddress()
	case a.IsDomainHost():
		return a.DomainAddress()
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (a Address) DomainPreferredAddress() string {
	switch {
	case a.IsDomainHost():
		return a.DomainAddress()
	case a.IsIPHost():
		return a.IPAddress()
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (a Address) NetworkAndIPAddress() string {
	return a.Network.This() + ":" + a.IPAddress()
}

func (a Address) NetworkAndDomainAddress() string {
	return a.Network.This() + ":" + a.DomainAddress()
}

func (a Address) NetworkAndIPPreferredAddress() string {
	switch {
	case a.IsIPHost():
		return a.NetworkAndIPAddress()
	case a.IsDomainHost():
		return a.NetworkAndDomainAddress()
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (a Address) NetworkAndDomainPreferredAddress() string {
	switch {
	case a.IsDomainHost():
		return a.NetworkAndDomainAddress()
	case a.IsIPHost():
		return a.NetworkAndIPAddress()
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func (a Address) AddrWithIPAddress() Addr {
	switch a.Network.This() {
	case Network_TCP:
		return &TCPAddr{
			IP:   a.IP,
			Port: int(a.Port.This()),
		}
	case Network_UDP:
		return &UDPAddr{
			IP:   a.IP,
			Port: int(a.Port.This()),
		}
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func AddressFromAddr(addr Addr) Address {
	switch address := addr.(type) {
	case *TCPAddr:
		return Address{
			IP:      address.IP,
			Port:    Port(address.Port),
			Network: Network_TCP,
		}
	case *UDPAddr:
		return Address{
			IP:      address.IP,
			Port:    Port(address.Port),
			Network: Network_UDP,
		}
	default:
		panic(common.ErrUnknownNetwork)
	}
}

func AddressFromHostPort(network Network, address Address) Address {
	return Address{
		IP:      address.IP,
		Domain:  address.Domain,
		Port:    address.Port,
		Network: network,
	}
}

func ParseHost(host string) (Address, error) {
	return ParseAddress(Network_UDP, JoinHostPort(host, Port(0).String()))
}

func ParseAddress(network, address string) (Address, error) {
	host, portStr, err := SplitHostPort(address)
	if err != nil {
		return Address{}, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Address{}, err
	}

	domain := host
	ip := ParseIP(host)
	if len(ip) > 0 {
		domain = EmptyDomain
	}

	return Address{
		IP:      ip,
		Domain:  Domain(domain),
		Port:    Port(uint16(port)),
		Network: Network(network),
	}, nil
}
