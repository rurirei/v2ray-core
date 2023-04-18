package session

import (
	"v2ray.com/core/common/cache"
)

type sessionKey byte

const (
	idSessionKey sessionKey = iota
	inboundSessionKey
	muxSessionKey
)

type Content interface {
	SetID(ID)
	GetID() (ID, bool)
	SetInbound(Inbound)
	GetInbound() (Inbound, bool)
	SetMux(Mux)
	GetMux() (Mux, bool)

	Close() error
}

type content struct {
	cache.Pool
}

func NewContent() Content {
	return &content{
		Pool: cache.NewPool(),
	}
}

func (c *content) SetID(id ID) {
	c.Set(idSessionKey, id)
}

func (c *content) GetID() (ID, bool) {
	if id, ok := c.Get(idSessionKey); ok {
		return id.(ID), true
	}
	return 0, false
}

func (c *content) SetInbound(inbound Inbound) {
	c.Set(inboundSessionKey, inbound)
}

func (c *content) GetInbound() (Inbound, bool) {
	if inbound, ok := c.Get(inboundSessionKey); ok {
		return inbound.(Inbound), true
	}
	return Inbound{}, false
}

func (c *content) SetMux(mux Mux) {
	c.Set(muxSessionKey, mux)
}

func (c *content) GetMux() (Mux, bool) {
	if mux, ok := c.Get(muxSessionKey); ok {
		return mux.(Mux), true
	}
	return Mux{}, false
}
