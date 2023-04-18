package buffer

var (
	// ErrReadTimeout is an error that happens with IO timeout.
	ErrReadTimeout = newError("Buffer Read timeout")
)

type dataHandler func(MultiBuffer)

type copyHandler struct {
	onData []dataHandler
}

// SizeCounter is for counting bytes copied by Copy().
type SizeCounter struct {
	Size int64
}

// CopyOption is an option for copying data.
type CopyOption func(*copyHandler)

// CountSize is a CopyOption that sums the total size of data copied into the given SizeCounter.
func CountSize(sc *SizeCounter) CopyOption {
	return func(handler *copyHandler) {
		handler.onData = append(handler.onData, func(b MultiBuffer) {
			sc.Size += int64(b.Len())
		})
	}
}

type readError struct {
	error
}

func (e readError) Error() string {
	return e.error.Error()
}

func (e readError) Inner() error {
	return e.error
}

// IsReadError returns true if the error in Copy() comes from reading.
func IsReadError(err error) bool {
	_, ok := err.(readError)
	return ok
}

func CauseReadError(err error) error {
	rerr, _ := err.(readError)
	return rerr.Inner()
}

type writeError struct {
	error
}

func (e writeError) Error() string {
	return e.error.Error()
}

func (e writeError) Inner() error {
	return e.error
}

// IsWriteError returns true if the error in Copy() comes from writing.
func IsWriteError(err error) bool {
	_, ok := err.(writeError)
	return ok
}

func CauseWriteError(err error) error {
	werr, _ := err.(writeError)
	return werr.Inner()
}

// Copy dumps all payload from reader to writer or stops when an error occurs. It returns nil when EOF.
func Copy(writer Writer, reader Reader, options ...CopyOption) error {
	copyFunc := func(writer Writer, reader Reader, handler *copyHandler) error {
		for {
			mb, rerr := reader.ReadMultiBuffer()
			if rerr != nil {
				return readError{rerr}
			}

			if mb != nil && !mb.IsEmpty() {
				for _, onData := range handler.onData {
					onData(mb)
				}

				if werr := writer.WriteMultiBuffer(mb); werr != nil {
					return writeError{werr}
				}
			}
		}
	}

	var handler copyHandler
	for _, option := range options {
		option(&handler)
	}

	return copyFunc(writer, reader, &handler)
}
