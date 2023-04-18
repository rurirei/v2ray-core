package bytespool

// The following parameters controls the size of buffer pools.
// There are numPools pools. Starting from 2k size, the size of each pool is sizeMulti of the previous one.
// Package buffer is guaranteed to not use buffers larger than the largest pool.
// Other packets may use larger buffers.
const (
	sizeBase = 2048
	sizeNum  = 4
)

const (
	Size = sizeBase
)

var (
	localPool = func() []*pool {
		p := make([]*pool, 0, sizeNum)

		for i := 1; i <= sizeNum; i++ {
			p = append(p, newPool(sizeBase*i))
		}

		return p
	}()
)

// Alloc returns a byte slice with at least the given size
// Default size of returned slice is 2048.
func Alloc(size int) []byte {
	for _, p := range localPool {
		if size <= p.size {
			return p.Alloc()
		}
	}
	return make([]byte, size, size)
}

// Free puts a byte slice into the internal pool.
func Free(b []byte) {
	for _, p := range localPool {
		if cap(b) >= p.size {
			b0 := b[:p.size]
			p.Free(b0)
			b = nil
		}
	}
}
