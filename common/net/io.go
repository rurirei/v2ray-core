package net

import (
	"v2ray.com/core/common/bytespool"
)

type WriteToFunc = func([]byte, Address) (int, error)

type WriterTo interface {
	WriteTo([]byte, Address) (int, error)
}

type writerTo struct {
	writer WriteToFunc
}

func NewWriterToFunc(writer WriteToFunc) WriterTo {
	return writerTo{
		writer: writer,
	}
}

func (w writerTo) WriteTo(p []byte, address Address) (int, error) {
	return w.writer(p, address)
}

type ReadFromFunc = func([]byte) (int, Address, error)

type ReaderFrom interface {
	ReadFrom([]byte) (int, Address, error)
}

type readerFrom struct {
	reader ReadFromFunc
}

func NewReaderFromFunc(reader ReadFromFunc) ReaderFrom {
	return readerFrom{
		reader: reader,
	}
}

func (r readerFrom) ReadFrom(p []byte) (int, Address, error) {
	return r.reader(p)
}

func CopyPacketAddrConn(dst WriterTo, src ReaderFrom) (int64, error) {
	buf := bytespool.Alloc(bytespool.Size)
	defer bytespool.Free(buf)

	n := int64(0)

	for {
		nr, address, err := src.ReadFrom(buf)
		if err != nil {
			return n, err
		}

		nw, err := dst.WriteTo(buf[:nr], address)
		if err != nil {
			return n, err
		}

		n += int64(nw)
	}
}

func CopyNonPacketAddrConn(dst WriterTo, src ReaderFrom, address Address) (int64, error) {
	buf := bytespool.Alloc(bytespool.Size)
	defer bytespool.Free(buf)

	n := int64(0)

	for {
		nr, _, err := src.ReadFrom(buf)
		if err != nil {
			return n, err
		}

		nw, err := dst.WriteTo(buf[:nr], address)
		if err != nil {
			return n, err
		}

		n += int64(nw)
	}
}
