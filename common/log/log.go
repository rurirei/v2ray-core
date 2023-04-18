package log

var (
	defaultLevel  = Debug
	defaultPrefix = NewPrefix("v2ray")

	localLogger = NewSimpleLogger(defaultLevel, defaultPrefix)

	Close = localLogger.Close

	Debugf = localLogger.Debugf
	Infof  = localLogger.Infof
	Warnf  = localLogger.Warningf
	Errorf = localLogger.Errorf
)

func RegisterAlternativeLogger(logger Logger) {
	localLogger = logger
}
