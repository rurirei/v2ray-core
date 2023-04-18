package mux

import (
	"sync/atomic"
	"time"

	"v2ray.com/core/common/cache"
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport"
)

var (
	// SessionOption todo timeout and size
	SessionOption = struct {
		Timeout time.Duration
		Size    int
	}{
		Timeout: 180 * time.Second,
		Size:    16,
	}
)

type sessionsManager struct {
	pool cache.Pool

	idGen atomic.Uint32
}

func newSessionsManager() *sessionsManager {
	return &sessionsManager{
		pool: cache.NewPool(),
	}
}

func (m *sessionsManager) Require(target net.Address) (*sessionManager, bool) {
	if manager0, ok := m.pool.Get(target.NetworkAndDomainPreferredAddress()); ok {
		return manager0.(*sessionManager), true
	}
	return nil, false
}

func (m *sessionsManager) New(target net.Address, link transport.Link) *sessionManager {
	manager := newSessionManager(link)

	m.pool.Set(target.NetworkAndDomainPreferredAddress(), manager)

	return manager
}

func (m *sessionsManager) IDGen() sessionID {
	return uint16(m.idGen.Add(1))
}

func (m *sessionsManager) Delete(target net.Address) {
	if _, ok := m.pool.Get(target.NetworkAndDomainPreferredAddress()); ok {
		m.pool.Delete(target.NetworkAndDomainPreferredAddress())
	}
}

type sessionManager struct {
	pool cache.Pool

	link transport.Link
}

func newSessionManager(link transport.Link) *sessionManager {
	return &sessionManager{
		pool: cache.NewPool(),
		link: link,
	}
}

func (m *sessionManager) Get(id sessionID) (sessionBody, bool) {
	if body, ok := m.pool.Get(id); ok {
		return body.(sessionBody), true
	}
	return sessionBody{}, false
}

func (m *sessionManager) Set(id sessionID, body sessionBody) {
	m.pool.Set(id, body)
}

func (m *sessionManager) Delete(id sessionID) {
	if _, ok := m.pool.Get(id); ok {
		m.pool.Delete(id)
	}
}

// sessionBody represents a client connection in a Mux connection.
type sessionBody struct {
	link transport.Link

	target net.Address
	id     sessionID
}
