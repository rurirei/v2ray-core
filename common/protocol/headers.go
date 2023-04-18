package protocol

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/shadowsocks"
	"v2ray.com/core/common/protocol/socks"
	"v2ray.com/core/common/protocol/trojan"
	"v2ray.com/core/common/protocol/vmess"
)

// RequestCommand is a custom command in a proxy request.
type RequestCommand struct {
	Shadowsocks shadowsocks.RequestCommand
	Socks       socks.RequestCommand
	Trojan      trojan.RequestCommand
	Vmess       vmess.RequestCommand
}

func (c RequestCommand) TransferType(i int) TransferType {
	switch i {
	case 1:
		return TransferTypeStream
	case 2:
		return TransferTypePacket
	default:
		panic(common.ErrUnknownNetwork)
	}
}

type RequestOption struct {
	Vmess vmess.RequestOption
}

type RequestVersion struct {
	Shadowsocks shadowsocks.Version
	Socks       socks.Version
	Vmess       vmess.Version
}

type RequestUser struct {
	Level uint32
	Email string

	Shadowsocks shadowsocks.User
	Socks       socks.User
	Vmess       vmess.User
}

type RequestAddress struct {
	net.Address
}

func (a RequestAddress) AsAddress(network net.Network) net.Address {
	return net.AddressFromHostPort(network, a.Address)
}

type RequestHeader struct {
	Command RequestCommand
	Option  RequestOption
	Version RequestVersion
	User    RequestUser
	Address RequestAddress
}

type ResponseCommand struct {
	Vmess vmess.ResponseCommand
}

type ResponseOption struct {
	Vmess vmess.ResponseOption
}

type ResponseHeader struct {
	Command ResponseCommand
	Option  ResponseOption
}

func isDomainTooLong(domain string) bool {
	return len(domain) > 256
}
