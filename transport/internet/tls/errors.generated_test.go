package tls_test

import (
	"v2ray.com/core/common/errors"

	_ "v2ray.com/core/transport/internet/tls"
)

type errorPathHolder struct {
}

func newError(msg string, args ...interface{}) errors.Error {
	return errors.New(msg, args...).WithPath(errorPathHolder{})
}
