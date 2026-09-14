package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/conn"
	"oncecall/utils/generic"
	"oncecall/vm"
	"oncecall/vm/lua"
	"sync"
)

type execPrivateState struct {
	manage conn.ConnPoolInterface

	logicMutex    sync.Mutex
	DbListRaw     [][]any
	DbOptionRaw   [][]any
	ScriptRaw     [][]any
	ScriptLinkRaw [][]any

	//dbname, script
	newRunQ         []generic.Pair[string, string]
	threadStopFnMap *generic.GenericSyncMap[generic.Pair[string, string], context.CancelFunc]
}

type execSharedState struct {
	ctx       context.Context
	pMap      *generic.GenericSyncMap[string, conn.ConnPoolInterface]
	scriptMap *generic.GenericSyncMap[string, generic.Pair[[16]byte, *cfg.ScriptConfig]]
	vmP       *generic.GenericSyncPool[vm.Vm]

	isRunningThreadMap *generic.GenericSyncMap[generic.Pair[string, string], bool]
}

func newExecSharedState() *execSharedState {
	return &execSharedState{
		pMap: generic.NewGenericSyncMap[string, conn.ConnPoolInterface](),
		vmP: generic.NewGenericSyncPool[vm.Vm](func() vm.Vm {
			v := lua.NewLuaVM()
			return v
		}),

		isRunningThreadMap: generic.NewGenericSyncMap[generic.Pair[string, string], bool](),
	}
}
