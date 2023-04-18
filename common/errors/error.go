package errors

import (
	"fmt"
	"reflect"

	"v2ray.com/core/common/log"
)

type Error struct {
	level log.Level
	path  string
	error error
}

func New(msg string, args ...interface{}) Error {
	return Error{
		error: fmt.Errorf(msg, args...),
	}
}

func (e Error) Error() string {
	return e.error.Error()
}

func (e Error) WithError(err interface{}) Error {
	e.error = fmt.Errorf("%v > %v", e.error, err)
	return e
}

func (e Error) WithPath(path interface{}) Error {
	e.path = e.resolvePath(path)
	e.error = fmt.Errorf("%s: %v", e.path, e.error)
	return e
}

func (e Error) resolvePath(path interface{}) string {
	if path == nil {
		return "nil"
	}

	p := reflect.TypeOf(path).PkgPath()

	return p
}

func (e Error) AtDebug() Error {
	return e.atLevel(log.Debug)
}

func (e Error) AtInfo() Error {
	return e.atLevel(log.Info)
}

func (e Error) AtWarning() Error {
	return e.atLevel(log.Warning)
}

func (e Error) AtError() Error {
	return e.atLevel(log.Error)
}

func (e Error) AtNone() Error {
	return e.atLevel(log.None)
}

func (e Error) atLevel(level log.Level) Error {
	e.level = level
	return e
}

func (e Error) Logging() {
	switch e.level {
	case log.Debug:
		log.Debugf(e.error.Error())
	case log.Info:
		log.Infof(e.error.Error())
	case log.Warning:
		log.Warnf(e.error.Error())
	case log.Error:
		log.Errorf(e.error.Error())
	case log.None:
	default:
	}
}
