package types

import "oncecall/cmd/oncecall_query/cfg"

type SharedStateSetter interface {
	SetRunningThreadState()
	SetStopThreadState()

	PushConnPool()
	PopConnPool()
}

type SharedStateGetter interface {
	GetThreadState() (isRun bool, exists bool)
	GetConnPool()
	GetVm()
	GetScriptConfig() *cfg.ScriptConfig
}
type SharedState interface {
	SetRunThread()
	GetIsRunningThread() bool


}