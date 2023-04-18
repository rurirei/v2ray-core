package router

import (
	"v2ray.com/core/common/router"
	"v2ray.com/core/common/session"
)

const (
	LookupDomain router.Name = iota

	LookupInboundTag
)

type DefaultLookup session.Lookup

func (d DefaultLookup) Match(condition router.Condition) bool {
	if !condition.MatchString(LookupDomain, d.Domain) {
		return false
	}
	if !condition.MatchString(LookupInboundTag, d.InboundTag) {
		return false
	}
	return true
}

func BuildDefaultLookup(lookup session.Lookup) DefaultLookup {
	return DefaultLookup(lookup)
}
