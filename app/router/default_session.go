package router

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/router"
	"v2ray.com/core/common/session"
)

const (
	SrcNetwork router.Name = iota
	DstNetwork

	SrcIP
	DstIP

	DstDomain

	SrcPort
	DstPort

	InboundTag
)

type DefaultContent struct {
	SrcNetwork, DstNetwork net.Network
	SrcIP, DstIP           net.IP
	DstDomain              net.Domain
	SrcPort, DstPort       net.Port
	InboundTag             string
}

func (d DefaultContent) Match(condition router.Condition) bool {
	if !condition.MatchString(SrcNetwork, d.SrcNetwork.This()) {
		return false
	}
	if !condition.MatchString(DstNetwork, d.DstNetwork.This()) {
		return false
	}
	if !condition.MatchIP(SrcIP, d.SrcIP) && !condition.MatchString(SrcIP, d.SrcIP.String()) {
		return false
	}
	if !condition.MatchIP(DstIP, d.DstIP) && !condition.MatchString(DstIP, d.DstIP.String()) {
		return false
	}
	if !condition.MatchString(DstDomain, d.DstDomain.This()) {
		return false
	}
	if !condition.MatchString(SrcPort, d.SrcPort.String()) {
		return false
	}
	if !condition.MatchString(DstPort, d.DstPort.String()) {
		return false
	}
	if !condition.MatchString(InboundTag, d.InboundTag) {
		return false
	}
	return true
}

func BuildDefaultContent(content session.Content, address net.Address) DefaultContent {
	ib, _ := content.GetInbound()

	return DefaultContent{
		SrcNetwork: ib.Source.Network,
		DstNetwork: address.Network,
		SrcIP:      ib.Source.IP,
		DstIP:      address.IP,
		DstDomain:  address.Domain,
		SrcPort:    ib.Source.Port,
		DstPort:    address.Port,
		InboundTag: ib.Tag,
	}
}
