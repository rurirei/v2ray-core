package task

import (
	"time"
)

type TimerFunc = func()

type Timer interface {
	After(TimerFunc, time.Duration)
	Period(TimerFunc, time.Duration)
	Close() error
}

type timer struct {
	done chan struct{}
}

func NewTimer() Timer {
	return &timer{
		done: make(chan struct{}),
	}
}

func (t *timer) After(fn TimerFunc, after time.Duration) {
	t0 := time.NewTimer(after)
	defer t0.Stop()

	select {
	case <-t0.C:
		fn()
	case <-t.done:
	}
}

func (t *timer) Period(fn TimerFunc, period time.Duration) {
	t0 := time.NewTimer(period)
	defer t0.Stop()

	for {
		select {
		case <-t0.C:
			fn()
		case <-t.done:
			break
		}
	}
}

func (t *timer) Close() error {
	close(t.done)

	return nil
}
