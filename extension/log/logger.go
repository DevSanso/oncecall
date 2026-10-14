package log

import (
	"io"
)

type LoggerDebugExtension[PARAM any] interface {
	Debug(msg string, param ...PARAM)
}

type LoggerLogExtension[PARAM any] interface {
	LoggerDebugExtension[PARAM]
	Info(msg string, param ...PARAM)
	Warn(msg string, param ...PARAM)
	Error(msg string, param ...PARAM)
}

type LoggerExtension[PARAM any] interface {
	LoggerLogExtension[PARAM]
	io.Closer
}
