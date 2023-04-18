package log

type simpleLogger struct {
	logger LoggerHandler
	level  Level
	prefix Prefix
}

func NewSimpleLogger(level Level, prefix Prefix) Logger {
	return &simpleLogger{
		logger: NewSimpleLoggerHandler(StdGoLoggerWithTimestamp(0)),
		level:  level,
		prefix: prefix,
	}
}

func (l *simpleLogger) Close() error {
	return l.logger.Close()
}

func (l *simpleLogger) Debugf(msg string, args ...interface{}) {
	if l.level <= Debug {
		l.logger.Printf("[Debug] "+msg, args...)
	}
}

func (l *simpleLogger) Infof(msg string, args ...interface{}) {
	if l.level <= Info {
		l.logger.Printf("[Info] "+msg, args...)
	}
}

func (l *simpleLogger) Warningf(msg string, args ...interface{}) {
	if l.level <= Warning {
		l.logger.Printf("[Warning] "+msg, args...)
	}
}

func (l *simpleLogger) Errorf(msg string, args ...interface{}) {
	if l.level <= Error {
		l.logger.Printf("[Error] "+msg, args...)
	}
}
