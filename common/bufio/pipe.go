package bufio

import (
	"sync"

	"v2ray.com/core/common/io"
)

type pipe struct {
	sync.Mutex

	writer *Writer
	reader *Reader
}

func NewPipe(readWriter io.ReadWriter) io.ReadWriteCloser {
	return &pipe{
		writer: NewWriter(readWriter),
		reader: NewReader(readWriter),
	}
}

func (p *pipe) Write(p2 []byte) (int, error) {
	p.Lock()
	defer p.Unlock()

	return p.writer.Write(p2)
}

func (p *pipe) Read(p2 []byte) (int, error) {
	p.Lock()
	defer p.Unlock()

	return p.reader.Read(p2)
}

func (p *pipe) Close() error {
	p.Lock()
	defer p.Unlock()

	_ = p.writer.Flush()
	_, _ = p.reader.Discard(p.reader.Buffered())

	p.writer = nil
	p.reader = nil

	return nil
}
