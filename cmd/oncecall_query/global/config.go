package global

import "oncecall/cmd/oncecall_query/cfg"

var (
	Config struct {
		File      *cfg.ProcessConfig
		ProcParam struct {
			LoopSec         int
			UpdateTimeoutMs int
			FetchTimeoutMs  int
		}
	}
)
