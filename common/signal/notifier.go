package signal

type Notifier interface {
	Signal()
	Wait() <-chan struct{}
	Close() error
}

// notifier is a utility for notifying changes. The change producer may notify changes multiple time, and the consumer may get notified asynchronously.
type notifier struct {
	ch chan struct{}
}

// NewNotifier creates a new notifier.
func NewNotifier() Notifier {
	return &notifier{
		ch: make(chan struct{}),
	}
}

// Signal signals a change, usually by producer. This method never blocks.
func (n *notifier) Signal() {
	select {
	case n.ch <- struct{}{}:
	default:
	}
}

// Wait returns a channel for waiting for changes. The returned channel never gets closed.
func (n *notifier) Wait() <-chan struct{} {
	return n.ch
}

func (n *notifier) Close() error {
	close(n.ch)

	return nil
}
