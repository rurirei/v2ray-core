package io

import (
	"io"

	"v2ray.com/core/common/bytespool"
)

func Discard(src Reader) (int64, error) {
	return Copy(NewWriterFunc(func(p []byte) (int, error) {
		return len(p), nil
	}), src)
}

func Copy(dst Writer, src Reader) (int64, error) {
	buf := bytespool.Alloc(bytespool.Size)
	defer bytespool.Free(buf)

	return io.CopyBuffer(dst, src, buf)
}
