package router

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/router"
	"v2ray.com/core/common/session"
)

type Matcher interface {
	MatchContent(session.Content, net.Address) (string, bool)
	MatchLookup(session.Lookup) (string, bool)
}

type matcher struct {
	rule []router.Rule
}

func NewMatcher(rule ...router.Rule) Matcher {
	return &matcher{
		rule: rule,
	}
}

func (m *matcher) MatchContent(content session.Content, address net.Address) (string, bool) {
	for _, rule := range m.rule {
		if matchContentRule(rule.Condition, content, address) {
			return rule.OutboundTag, true
		}
	}
	return "", false
}

func (m *matcher) MatchLookup(lookup session.Lookup) (string, bool) {
	for _, rule := range m.rule {
		if matchLookupRule(rule.Condition, lookup) {
			return rule.OutboundTag, true
		}
	}
	return "", false
}

func matchContentRule(condition router.Condition, content session.Content, address net.Address) bool {
	dc := BuildDefaultContent(content, address)
	return dc.Match(condition)
}

func matchLookupRule(condition router.Condition, lookup session.Lookup) bool {
	dc := BuildDefaultLookup(lookup)
	return dc.Match(condition)
}
