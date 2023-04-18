package socks

// Version AuthType is the outbound server version of Socks proxy.
type Version = int32

const (
	Version_SOCKS5  Version = 0
	Version_SOCKS4  Version = 1
	Version_SOCKS4A Version = 2
)
