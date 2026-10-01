package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"oncecall/vm"
	"oncecall/vm/lua"
	"sync"
)

type execPrivateState struct {
	manage types.ConnPoolInterface

	logicMutex    sync.Mutex
	DbListRaw     [][]any
	DbOptionRaw   [][]any
	ScriptRaw     [][]any
	ScriptLinkRaw [][]any

	//Ident, script
	newRunQ         []generic.Pair[string, string]
	StopRunQ        []generic.Pair[string, string]
	threadStopFnMap *generic.SyncMap[generic.Pair[string, string], context.CancelFunc]
}

type execSharedState struct {
	executorContext context.Context
	pMap            *generic.SyncMap[string, types.ConnPoolInterface]
	scriptMap       *generic.SyncMap[string, generic.Pair[[16]byte, *cfg.ScriptConfig]]
	vmP             *generic.SyncPool[vm.Vm]

	isRunningThreadMap *generic.SyncMap[generic.Pair[string, string], bool]
}

func newExecSharedState() *execSharedState {
	return &execSharedState{
		pMap: generic.NewGenericSyncMap[string, types.ConnPoolInterface](),
		vmP: generic.NewGenericSyncPool[vm.Vm](func() vm.Vm {
			v := lua.NewLuaVM()
			return v
		}),

		isRunningThreadMap: generic.NewGenericSyncMap[generic.Pair[string, string], bool](),
	}
}
