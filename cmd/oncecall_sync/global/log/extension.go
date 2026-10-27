package log

import (
	"oncecall/extension/log"
)

var (
	Log struct {
		Conn   log.LoggerExtension[any]
		Script log.LoggerExtension[any]
		Manage log.LoggerExtension[any]
		Query  log.LoggerExtension[any]
		Gen    log.LoggerExtension[any]
		Proc   log.LoggerExtension[any]
	}
)
