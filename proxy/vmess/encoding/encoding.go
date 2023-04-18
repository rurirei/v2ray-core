package encoding

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(byte(protocol.HostLengthIPv4), net.HostLengthIPv4),
	protocol.AddressFamilyByte(byte(protocol.HostLengthIPv6), net.HostLengthIPv6),
	protocol.AddressFamilyByte(byte(protocol.HostLengthDomain), net.HostLengthDomain),
	protocol.PortThenAddress(),
)

//go:generate go run v2ray.com/core/common/errors/errorgen
