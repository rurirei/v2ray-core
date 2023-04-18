package transport

import (
	"sync"
	"time"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/signal"
)

var (
	errSlowDown = newError("pipe slow down")
)

type PipeReadWriteCloser interface {
	PipeReader
	PipeWriteCloser
}

type PipeWriteCloser interface {
	buffer.Writer
	io.Closer
}

type PipeReader interface {
	buffer.Reader
	buffer.TimeoutReader
}

type pipe2 struct {
	data      chan buffer.MultiBuffer
	writeDone signal.Done
}

func NewPipe2() PipeReadWriteCloser {
	return &pipe2{
		data:      make(chan buffer.MultiBuffer),
		writeDone: signal.NewDone(),
	}
}

func (p *pipe2) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	select {
	case p.data <- mb:
		return nil
	case <-p.writeDone.Wait():
		return io.ErrClosedPipe
	}
}

func (p *pipe2) ReadMultiBufferTimeout(timeout time.Duration) (buffer.MultiBuffer, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case data := <-p.data:
		return data, nil
	case <-p.writeDone.Wait():
		close(p.data)
		return nil, io.EOF
	case <-timer.C:
		return nil, buffer.ErrReadTimeout
	}
}

func (p *pipe2) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	select {
	case data := <-p.data:
		return data, nil
	case <-p.writeDone.Wait():
		close(p.data)
		return nil, io.EOF
	}
}

func (p *pipe2) Close() error {
	_ = p.writeDone.Close()

	return nil
}

type multiData struct {
	from, to *int
	mbs      map[int]buffer.MultiBuffer
}

func (d multiData) Write(d2 buffer.MultiBuffer) {
	d.mbs[*d.to] = d2
	*d.to++
}

func (d multiData) Read() buffer.MultiBuffer {
	d2 := d.mbs[*d.from]

	d.mbs[*d.from] = nil
	delete(d.mbs, *d.from)
	*d.from++

	return d2
}

func (d multiData) IsEmpty() bool {
	return len(d.mbs) == 0
}

func (d multiData) Release() {
	for i, d2 := range d.mbs {
		d2 = buffer.ReleaseMulti(d2)
		d.mbs[i] = d2
		delete(d.mbs, i)
	}

	d.mbs = nil
	d.from = nil
	d.to = nil
}

type pipe struct {
	sync.Mutex

	data       multiData
	readSignal signal.Notifier
	done       signal.Done
}

func NewPipe() PipeReadWriteCloser {
	return &pipe{
		data: multiData{
			from: new(int),
			to:   new(int),
			mbs:  make(map[int]buffer.MultiBuffer),
		},
		readSignal: signal.NewNotifier(),
		done:       signal.NewDone(),
	}
}

func (p *pipe) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	write := func() error {
		p.Lock()
		defer p.Unlock()

		if p.done.Done() {
			return io.ErrClosedPipe
		}

		p.data.Write(mb)

		p.readSignal.Signal()
		return nil
	}

	return write()
}

func (p *pipe) ReadMultiBufferTimeout(timeout time.Duration) (buffer.MultiBuffer, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if mb, err := p.readMultiBuffer(); err != errSlowDown {
			return mb, err
		}

		select {
		case <-p.readSignal.Wait():
		case <-p.done.Wait():
		case <-timer.C:
			return nil, buffer.ErrReadTimeout
		}
	}
}

func (p *pipe) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	for {
		if mb, err := p.readMultiBuffer(); err != errSlowDown {
			return mb, err
		}

		select {
		case <-p.readSignal.Wait():
		case <-p.done.Wait():
		}
	}
}

func (p *pipe) readMultiBuffer() (buffer.MultiBuffer, error) {
	p.Lock()
	defer p.Unlock()

	if p.done.Done() && p.data.IsEmpty() {
		p.data.Release()

		return nil, io.EOF
	}

	if p.data.IsEmpty() {
		return nil, errSlowDown
	}

	data := p.data.Read()

	return data, nil
}

func (p *pipe) Close() error {
	p.Lock()
	defer p.Unlock()

	// p.data.Release()

	_ = p.readSignal.Close()
	_ = p.done.Close()

	return nil
}
