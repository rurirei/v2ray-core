package tor_test

import (
	"v2ray.com/core/common/errors"

	_ "v2ray.com/core/proxy/tor"
)

type errorPathHolder struct {
}

func newError(msg string, args ...interface{}) errors.Error {
	return errors.New(msg, args...).WithPath(errorPathHolder{})
}
