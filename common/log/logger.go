package log

type Logger interface {
	Close() error

	Debugf(msg string, args ...interface{})
	Infof(msg string, args ...interface{})
	Warningf(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
}
