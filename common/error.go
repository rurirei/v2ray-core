package common

var (
	ErrUnknownNetwork = newError("unknown network")
	ErrUnknownError   = newError("unknown error")
	ErrNotImplemented = newError("not implemented")
)
