package loader

import (
	"net/netip"

	router_app "v2ray.com/core/app/router"
	router_common "v2ray.com/core/common/router"
)

func RegisterNameserverMatcher(matcher router_app.Matcher) {
	localInstance.NameserverMatcher = matcher
}

func RegisterOutboundMatcher(matcher router_app.Matcher) {
	localInstance.OutboundMatcher = matcher
}

type DefaultConditionSetting struct {
	Name   string
	Length string
	String []string
	CIDR   []netip.Prefix
}

func ParseDefaultCondition(parseDefaultConditionNameFunc ParseDefaultConditionNameFunc, setting DefaultConditionSetting) (router_common.ConditionBody, error) {
	name, err := parseDefaultConditionNameFunc(setting.Name)
	if err != nil {
		return router_common.ConditionBody{}, err
	}

	length, err := ParseConditionLength(setting.Length)
	if err != nil {
		return router_common.ConditionBody{}, err
	}

	return router_common.ConditionBody{
		Name:   name,
		Length: length,
		String: setting.String,
		CIDR:   setting.CIDR,
	}, nil
}

type ParseDefaultConditionNameFunc = func(string) (router_common.Name, error)

const (
	DefaultContentConditionName_SrcNetwork = "srcNetwork"
	DefaultContentConditionName_DstNetwork = "dstNetwork"
	DefaultContentConditionName_SrcIP      = "srcIP"
	DefaultContentConditionName_DstIP      = "dstIP"
	DefaultContentConditionName_DstDomain  = "dstDomain"
	DefaultContentConditionName_SrcPort    = "srcPort"
	DefaultContentConditionName_DstPort    = "dstPort"
	DefaultContentConditionName_InboundTag = "inboundTag"
)

const (
	DefaultLookupConditionName_Domains    = "domains"
	DefaultLookupConditionName_InboundTag = "inboundTag"
)

func ParseDefaultContentConditionName(s string) (router_common.Name, error) {
	switch s {
	case DefaultContentConditionName_SrcNetwork:
		return router_app.SrcNetwork, nil
	case DefaultContentConditionName_DstNetwork:
		return router_app.DstNetwork, nil
	case DefaultContentConditionName_SrcIP:
		return router_app.SrcIP, nil
	case DefaultContentConditionName_DstIP:
		return router_app.DstIP, nil
	case DefaultContentConditionName_DstDomain:
		return router_app.DstDomain, nil
	case DefaultContentConditionName_SrcPort:
		return router_app.SrcPort, nil
	case DefaultContentConditionName_DstPort:
		return router_app.DstPort, nil
	case DefaultContentConditionName_InboundTag:
		return router_app.InboundTag, nil
	default:
		return 0, newError("unknown name %s", s)
	}
}

func ParseDefaultLookupConditionName(s string) (router_common.Name, error) {
	switch s {
	case DefaultLookupConditionName_Domains:
		return router_app.LookupDomain, nil
	case DefaultLookupConditionName_InboundTag:
		return router_app.LookupInboundTag, nil
	default:
		return 0, newError("unknown name %s", s)
	}
}

const (
	ConditionLength_Full  = "full"
	ConditionLength_Sub   = "sub"
	ConditionLength_Regex = "regex"
)

func ParseConditionLength(s string) (router_common.Length, error) {
	switch s {
	case ConditionLength_Full:
		return router_common.Full, nil
	case ConditionLength_Sub:
		return router_common.Sub, nil
	case ConditionLength_Regex:
		return router_common.Regex, nil
	default:
		return 0, newError("unknown length %s", s)
	}
}
