package bytespool

import (
	"sync"
)

type pool struct {
	size int
	pool *sync.Pool
}

func newPool(size int) *pool {
	return &pool{
		size: size,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, size, size)
			},
		},
	}
}

func (p *pool) Alloc() []byte {
	return p.pool.Get().([]byte)
}

func (p *pool) Free(b []byte) {
	p.pool.Put(b)
}
