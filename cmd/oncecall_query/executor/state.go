package executor

import (
	"context"
	"oncecall/conn"
	"oncecall/utils/generic"
	"oncecall/vm/lua"
)

type execState[S any] struct {
	ctx        context.Context
	jobCache   *generic.GenericSyncMap[string, any]
	pMap       *generic.GenericSyncMap[generic.Pair[int, bool], conn.ConnPoolInterface]
	vmP        *generic.GenericSyncPool[lua.LuaVm]
	scriptPool *generic.GenericSyncPool[S]

	isRunFlagMap *generic.GenericSyncMap[string, bool]
}

func newExecState[S any](scriptGenFn func() S) *execState[S] {
	return &execState[S]{
		jobCache:   generic.NewGenericSyncMap[string, any](),
		pMap:       generic.NewGenericSyncMap[generic.Pair[int, bool], conn.ConnPoolInterface](),
		scriptPool: generic.NewGenericSyncPool[S](scriptGenFn),
		vmP: generic.NewGenericSyncPool[lua.LuaVm](func() lua.LuaVm {
			v := lua.NewLuaVM()

			return v
		}),
		isRunFlagMap: generic.NewGenericSyncMap[string, bool](),
	}
}
