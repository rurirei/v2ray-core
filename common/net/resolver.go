package net

// LookupIPFunc implements net.Resolver.LookupIP
type LookupIPFunc = func(string, string) ([]IP, error)

type ListenPacketFunc = func(Address) (PacketConn, error)

type DialFuncFunc = func(Address, *GoDialer) DialFunc

// https://pkg.go.dev/net#Resolver.LookupIP
// network must be one of "ip", "ip4" or "ip6"
const (
	Network_Resolver_IP  = Network_IP
	Network_Resolver_IP6 = Network_IP6
	Network_Resolver_IP4 = Network_IP4
)

var (
	LookupIPOption = struct {
		Network Network
		TTL     int64
	}{
		Network: Network_IP,
		TTL:     1,
	}
)

var LocalLookupIPFunc = func(_, host string) ([]IP, error) {
	return LookupIP(host)
}

var LocalListenPacketFunc = func(address Address) (PacketConn, error) {
	return ListenPacket(address.Network.This(), address.IPAddress())
}

var LocalDialFunc = func(_ Address, dialer *GoDialer) DialFunc {
	return dialer.Dial
}
