package transport

type Link struct {
	Reader PipeReader
	Writer PipeWriteCloser
}

func NewLink() (Link, Link) {
	p1 := NewPipe()
	p2 := NewPipe()

	inboundLink := Link{
		Writer: p2,
		Reader: p1,
	}

	outboundLink := Link{
		Writer: p1,
		Reader: p2,
	}

	return inboundLink, outboundLink
}
