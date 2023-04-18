package router

import (
	"net/netip"

	"v2ray.com/core/common/net"
)

type Condition []ConditionBody

func (c Condition) MatchString(name Name, s string) bool {
	for _, body := range c {
		if body.Name == name && !body.MatchString(s) {
			return false
		}
	}
	return true
}

func (c Condition) MatchIP(name Name, s net.IP) bool {
	for _, body := range c {
		if body.Name == name && !body.MatchIP(s) {
			return false
		}
	}
	return true
}

type Name byte

type Length byte

const (
	Full Length = iota
	Sub
	Regex
)

type ConditionBody struct {
	Name   Name
	Length Length
	String []string
	CIDR   []netip.Prefix
}

func (c ConditionBody) MatchString(s string) bool {
	switch c.Length {
	case Full:
		return MatchFullString(c.String, s)
	case Sub:
		return MatchSubString(c.String, s)
	case Regex:
		return MatchRegexString(c.String, s)
	default:
		return false
	}
}

func (c ConditionBody) MatchIP(s net.IP) bool {
	switch c.Length {
	case Full:
		return MatchIP(c.CIDR, func() netip.Addr {
			ip, _ := netip.AddrFromSlice(s)
			return ip
		}())
	default:
		return false
	}
}
