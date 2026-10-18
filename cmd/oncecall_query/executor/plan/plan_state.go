package plan

import (
	"context"
	"oncecall/cmd/oncecall_query/executor/types"
	"oncecall/utils/generic"
	"sync/atomic"
)

type PlanState struct {
	context    context.Context
	cancelFunc context.CancelFunc
	seq        int64
	syncIdent  types.IdentKey

	ref *struct {
		isInit         atomic.Bool
		paramBuffer    planParamBuffer
		scriptCacheMap generic.SyncMap[string, any]
	}
}

func (p *PlanState) Close() error {
	p.cancelFunc()
	return nil
}

func NewPlanState(baseCtx context.Context, syncIdent types.IdentKey, readPlanCount int, colCap int, rowCap int, idleTime int) PlanState {
	ctx, cancel := context.WithCancel(baseCtx)

	return PlanState{
		context:    ctx,
		cancelFunc: cancel,
		ref: &struct {
			isInit         atomic.Bool
			paramBuffer    planParamBuffer
			scriptCacheMap generic.SyncMap[string, any]
		}{
			isInit:         atomic.Bool{},
			paramBuffer:    newPlanParamBuffer(readPlanCount, colCap, rowCap, idleTime),
			scriptCacheMap: generic.NewGenericSyncMapWValue[string, any]()},
		seq:       0,
		syncIdent: syncIdent,
	}
}
