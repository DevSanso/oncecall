package global

import (
	"oncecall/extension/log"
	"oncecall/extension/metric"
)

var (
	Log struct {
		ConnLogExtension   log.LoggerExtension[any]
		ScriptLogExtension log.LoggerExtension[any]
		ManageLogExtension log.LoggerExtension[any]
		QueryLogExtension  log.LoggerExtension[any]
		GenLogExtension    log.LoggerExtension[any]
	}

	Metric struct {
		Metric metric.MetricExtension
	}
)
