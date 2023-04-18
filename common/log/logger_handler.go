package log

import (
	"io"
	"log"
	"os"
)

const (
	prefix_empty = ""
)

type LoggerHandler interface {
	LoggerWriter
	LoggerCloser
}

type LoggerWriter interface {
	Printf(s string, v ...interface{})
}

type LoggerCloser interface {
	Close() error
}

type loggerHandler struct {
	*log.Logger
}

func NewSimpleLoggerHandler(logger *log.Logger) LoggerHandler {
	return &loggerHandler{
		Logger: logger,
	}
}

func (l *loggerHandler) Close() error {
	if closer, ok := l.Writer().(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

func StdGoLoggerWithTimestamp(flag int) *log.Logger {
	return StdGoLogger(flag | log.Ldate | log.Ltime)
}

func StdGoLogger(flag int) *log.Logger {
	return OutGoLogger(os.Stdout, prefix_empty, flag)
}

func OutGoLogger(out io.WriteCloser, prefix string, flag int) *log.Logger {
	return log.New(out, prefix, flag)
}
