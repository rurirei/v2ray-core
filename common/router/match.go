package router

import (
	"net/netip"
	"regexp"
	"strings"
)

func MatchIP(cidr []netip.Prefix, ip netip.Addr) bool {
	if ip.IsValid() {
		for _, c := range cidr {
			if c.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func MatchRegexString(str []string, s string) bool {
	if len(s) > 0 {
		for _, reg := range str {
			if ok, _ := regexp.MatchString(reg, s); ok {
				return true
			}
		}
	}
	return false
}

func MatchSubString(str []string, s string) bool {
	if len(s) > 0 {
		for _, sub := range str {
			if strings.Contains(s, sub) {
				return true
			}
		}
	}
	return false
}

func MatchFullString(str []string, s string) bool {
	if len(s) > 0 {
		for _, t := range str {
			if strings.EqualFold(s, t) {
				return true
			}
		}
	}
	return false
}
