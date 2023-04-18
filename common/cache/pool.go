package cache

import (
	"sync"
	"time"

	"v2ray.com/core/common/task"
)

type poolEntry struct {
	key, value interface{}
	expire     task.Timer
}

type pool struct {
	sync.RWMutex

	pool map[interface{}]poolEntry
}

func NewPool() Pool {
	return &pool{
		pool: make(map[interface{}]poolEntry),
	}
}

func (p *pool) Get(key interface{}) (interface{}, bool) {
	p.RLock()
	defer p.RUnlock()

	return p.get(key)
}

func (p *pool) Set(key, value interface{}) {
	p.Lock()
	defer p.Unlock()

	p.setExpire(key, value, 0)
}

func (p *pool) SetExpire(key, value interface{}, expire int64) {
	p.Lock()
	defer p.Unlock()

	p.setExpire(key, value, expire)
}

func (p *pool) Delete(key interface{}) {
	p.Lock()
	defer p.Unlock()

	p.delete(key)
}

func (p *pool) Range(fn RangeFunc) {
	p.Lock()
	defer p.Unlock()

	p.ranging(fn)
}

func (p *pool) Close() error {
	p.Lock()
	defer p.Unlock()

	return p.closing()
}

func (p *pool) get(key interface{}) (interface{}, bool) {
	if v, ok := p.pool[key]; ok {
		return v.value, true
	}
	return nil, false
}

func (p *pool) setExpire(key, value interface{}, expire int64) {
	entry := func() poolEntry {
		expired := func() {
			if _, ok := p.get(key); ok {
				p.delete(key)
			}
		}

		expiring := func(entry poolEntry) {
			if expire > 0 {
				go entry.expire.After(func() {
					p.Delete(key)
				}, time.Duration(expire)*time.Second)
			}
		}

		entry := poolEntry{
			key:    key,
			value:  value,
			expire: task.NewTimer(),
		}

		expired()

		expiring(entry)

		return entry
	}()

	p.pool[key] = entry
}

func (p *pool) delete(key interface{}) {
	func() {
		v := p.pool[key]
		_ = v.expire.Close()
	}()

	delete(p.pool, key)
}

func (p *pool) ranging(fn RangeFunc) {
	for k, v := range p.pool {
		if !fn(k, v.value) {
			return
		}
	}
}

func (p *pool) closing() error {
	p.ranging(func(k, _ interface{}) bool {
		p.delete(k)
		return true
	})

	p.pool = nil

	return nil
}
