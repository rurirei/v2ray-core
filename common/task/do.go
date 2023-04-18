package task

import (
	"sync"
)

type ErrorFunc = func() error

func Parallel(fns ...ErrorFunc) []error {
	var (
		errs = make([]error, 0, len(fns))
		wg   sync.WaitGroup
	)

	for _, fn := range fns {
		wg.Add(1)

		go func(fn ErrorFunc) {
			defer wg.Done()

			if err := fn(); err != nil {
				errs = append(errs, err)
			}
		}(fn)
	}

	wg.Wait()

	return errs
}

// OnAfter defers onAfter after fn returns
func OnAfter(fn, onAfter ErrorFunc) error {
	defer func() {
		_ = onAfter()
	}()

	return fn()
}

// OnSuccess defers onSuccess if fn returns no error
func OnSuccess(fn, onSuccess ErrorFunc) error {
	err := fn()
	if err == nil {
		defer func() {
			_ = onSuccess()
		}()
	}
	return err
}

// OnFail defers onFail if fn returns an error
func OnFail(fn, onFail ErrorFunc) error {
	err := fn()
	if err != nil {
		defer func() {
			_ = onFail()
		}()
	}
	return err
}
