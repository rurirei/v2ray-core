package task

func Retry(fn func() error, attempt int) []error {
	errs := make([]error, 0, attempt)

	for i := 0; i < attempt; i++ {
		err := fn()
		if err == nil {
			break
		}
		errs = append(errs, err)
	}

	return errs
}
