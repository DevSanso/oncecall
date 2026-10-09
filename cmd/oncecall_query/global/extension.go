package global

import (
	"oncecall/extension/log"
	"oncecall/extension/metric"
)

var (
	Log struct {
		ConnLogExtension    log.LoggerExtension[any]
		ScriptLogExtension    log.LoggerExtension[any]
		ExecuteLogExtension log.LoggerExtension[any]
		RawLogExtension log.LoggerExtension[any]
	}

	Metric struct {
		Metric metric.MetricExtension
	}
)
