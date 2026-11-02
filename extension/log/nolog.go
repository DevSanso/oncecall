package log

type NoLogLoggerExtension struct{}

func (n *NoLogLoggerExtension) Debug(msg string, any ...any) {

}

func (n *NoLogLoggerExtension) Info(msg string, any ...any) {

}

func (n *NoLogLoggerExtension) Warn(msg string, any ...any) {

}

func (n *NoLogLoggerExtension) Error(msg string, any ...any) {

}

func (n *NoLogLoggerExtension) Close() error {
	return nil
}
