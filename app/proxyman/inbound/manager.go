package inbound

import (
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/cache"
)

type Manager interface {
	Get(interface{}) (proxyman.Inbound, bool)
	Add(interface{}, proxyman.Inbound)
	Delete(interface{})
}

type manager struct {
	pool cache.Pool
}

func NewManager() Manager {
	return &manager{
		pool: cache.NewPool(),
	}
}

func (m *manager) Get(key interface{}) (proxyman.Inbound, bool) {
	if handler, ok := m.pool.Get(key); ok {
		return handler.(proxyman.Inbound), true
	}
	return nil, false
}

func (m *manager) Add(key interface{}, handler proxyman.Inbound) {
	m.pool.Set(key, handler)
}

func (m *manager) Delete(key interface{}) {
	if _, ok := m.pool.Get(key); ok {
		m.pool.Delete(key)
	}
}
