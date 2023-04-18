package udp

import (
	"sync"

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
	WritePacket(Packet) error

	io.Closer
}

type PipeReader interface {
	io.Reader
}

type pipe2 struct {
	data      chan Packet
	writeDone signal.Done
}

func NewPipe2() PipeReadWriteCloser {
	return &pipe2{
		data:      make(chan Packet),
		writeDone: signal.NewDone(),
	}
}

func (p *pipe2) WritePacket(pkt Packet) error {
	select {
	case p.data <- pkt:
		return nil
	case <-p.writeDone.Wait():
		return io.ErrClosedPipe
	}
}

// ReadFrom todo check len(p2) to read in
func (p *pipe2) Read(p2 []byte) (int, error) {
	pkt, err := p.ReadPacket()
	if err != nil {
		return 0, err
	}
	defer pkt.Payload.Release()

	return pkt.Payload.Read(p2)
}

func (p *pipe2) ReadPacket() (Packet, error) {
	select {
	case data := <-p.data:
		return data, nil
	case <-p.writeDone.Wait():
		close(p.data)
		return Packet{}, io.EOF
	}
}

func (p *pipe2) Close() error {
	_ = p.writeDone.Close()

	return nil
}

type multiData struct {
	from, to *int
	ps       map[int]Packet
}

func (d multiData) Write(d2 Packet) {
	d.ps[*d.to] = d2
	*d.to++
}

func (d multiData) Read(readLen int) Packet {
	d2 := d.ps[*d.from]

	if n := readLen; n < d2.Payload.Len() {
		d3 := d2
		d3.Payload.Advance(n)
		d.ps[*d.from] = d3
	} else {
		d.ps[*d.from] = Packet{}
		delete(d.ps, *d.from)
		*d.from++
	}

	return d2
}

func (d multiData) IsEmpty() bool {
	return len(d.ps) == 0
}

func (d multiData) Release() {
	for i, d2 := range d.ps {
		d2.Payload.Release()
		d2 = Packet{}
		d.ps[i] = d2
		delete(d.ps, i)
	}

	d.ps = nil
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
			ps:   make(map[int]Packet),
		},
		readSignal: signal.NewNotifier(),
		done:       signal.NewDone(),
	}
}

func (p *pipe) WritePacket(p2 Packet) error {
	write := func() error {
		p.Lock()
		defer p.Unlock()

		if p.done.Done() {
			return io.ErrClosedPipe
		}

		p.data.Write(p2)

		p.readSignal.Signal()
		return nil
	}

	return write()
}

func (p *pipe) Read(p2 []byte) (int, error) {
	pkt, err := p.ReadPacket(len(p2))
	if err != nil {
		return 0, err
	}
	defer pkt.Payload.Release()

	return pkt.Payload.Read(p2)
}

func (p *pipe) ReadPacket(readLen int) (Packet, error) {
	read := func() (Packet, error) {
		p.Lock()
		defer p.Unlock()

		if p.done.Done() && p.data.IsEmpty() {
			p.data.Release()

			return Packet{}, io.EOF
		}

		if p.data.IsEmpty() {
			return Packet{}, errSlowDown
		}

		data := p.data.Read(readLen)

		return data, nil
	}

	for {
		if data, err := read(); err != errSlowDown {
			return data, err
		}

		select {
		case <-p.readSignal.Wait():
		case <-p.done.Wait():
		}
	}
}

func (p *pipe) Close() error {
	p.Lock()
	defer p.Unlock()

	// p.data.Release()

	_ = p.readSignal.Close()
	_ = p.done.Close()

	return nil
}
