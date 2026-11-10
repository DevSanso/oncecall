package plan

import (
	"io"
	"oncecall/cmd/oncecall_sync/cfg"
	"oncecall/cmd/oncecall_sync/state"
)

type Plan interface {
	Init(state.SharedStatePlanCtl, cfg.ScriptConfig) error
	Run(planState *PlanState, isWait bool) error
	io.Closer
}
