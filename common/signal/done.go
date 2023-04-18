package signal

type Done interface {
	Done() bool
	Wait() <-chan struct{}
	Close() error
}

// done is a utility for notifications of something being done.
type done struct {
	ch chan struct{}
}

// NewDone returns a new Done.
func NewDone() Done {
	return &done{
		ch: make(chan struct{}),
	}
}

// Done returns true if Close() is called.
func (d *done) Done() bool {
	select {
	case <-d.ch:
		return true
	default:
		return false
	}
}

// Wait returns a channel for waiting for done.
func (d *done) Wait() <-chan struct{} {
	return d.ch
}

// Close marks this Done 'done'.
// This method could be called only once.
func (d *done) Close() error {
	close(d.ch)

	return nil
}
