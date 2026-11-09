package new

import (
	"oncecall/cmd/oncecall_sync/cfg"
	"oncecall/cmd/oncecall_sync/global/config"
	"oncecall/cmd/oncecall_sync/state"
	"oncecall/connpool/types"
	conntype "oncecall/connpool/types"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/pool"
	"oncecall/utils"
	"oncecall/utils/generic"
	"oncecall/vm"
	"sync"
	"sync/atomic"
	"time"
)

type PlanV1 struct {
	name string

	initVal struct {
		flag    atomic.Bool
		once    sync.Once
		useConf cfg.ScriptConfig

		readConnPool         []conntype.ConnPool
		readConnPoolIndexMap map[string]int

		syncPool conntype.ConnPool

		colBufCapSize int
		rowBufCapSize int
		bufIdleTime   int

		vmPool pool.PoolExtension[generic.NoImplCloser[vm.Vm], [][]any]
	}
}

func (p *PlanV1) Close() error {
	return nil
}

func (p *PlanV1) Init(s state.SharedStatePlanCtl, conf cfg.ScriptConfig) error {
	if !p.initVal.flag.Load() {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "already init")
	}
	var err error = nil

	p.initVal.once.Do(func() {
		for idx := range conf.Plans.Read {
			ident := conf.Plans.Read[idx].ReadIdent
			readP, pErr := s.GetConnPool(ident)
			if pErr != nil {
				err = errlist.ErrG.NewError(prefix.SentinelCatchError, "not find pool %s", ident)
				return
			}

			p.initVal.readConnPool = append(p.initVal.readConnPool, readP)
			p.initVal.readConnPoolIndexMap[ident] = len(p.initVal.readConnPool) - 1
		}
		syncP, syncErr := s.GetConnPool(config.SyncIdent)
		if syncErr != nil {
			err = errlist.ErrG.NewError(prefix.SentinelCatchError, "not find pool %s", syncErr)
			return
		}

		if conf.Option != nil {
			p.initVal.bufIdleTime = conf.Option.PlanRowBufIdleTime
			p.initVal.colBufCapSize = conf.Option.PlanColsBufAlloc
			p.initVal.rowBufCapSize = conf.Option.PlanRowBufCap
		}

		p.initVal.syncPool = syncP
		p.initVal.useConf = conf
		p.initVal.flag.Store(true)
	})

	return err
}

func (p *PlanV1) initSelf(planState *PlanState) error {
	ctx := planState.ctx
	if planState.isInitSelf.Load() {
		return nil
	}

	for _, initQuery := range p.initVal.useConf.Init.Self {
		data, _, triggerErr := p.initVal.syncPool.RunQuery(ctx, &conntype.Args{
			Query:         initQuery.TriggerQuery,
			Args:          nil,
			IsTransaction: false,
		})

		if triggerErr != nil {
			return errlist.ErrG.NewError(triggerErr, "self init failed, trigger query failed")
		}

		if len(data) <= 0 {
			continue
		}

		initErr := p.initVal.syncPool.RunExecute(ctx, &conntype.Args{
			Query:         initQuery.Query,
			Args:          nil,
			IsTransaction: false,
		})

		if initErr != nil {
			return errlist.ErrG.NewError(initErr, "self init failed, init query failed")
		}
	}
	return nil
}

func (p *PlanV1) initOther(planState *PlanState) error {
	ctx := planState.ctx
	if planState.isInitOther.Load() {
		return nil
	}

	for k, val := range p.initVal.useConf.Init.Other {
		pIdx, ok := p.initVal.readConnPoolIndexMap[k]
		if !ok {
			return errlist.ErrG.NewError(prefix.SentinelCatchError, "not exists %s", k)
		}
		initDb := p.initVal.readConnPool[pIdx]
		data, _, triggerErr := initDb.RunQuery(ctx, &conntype.Args{
			Query:         val.TriggerQuery,
			Args:          nil,
			IsTransaction: false,
		})

		if triggerErr != nil {
			return errlist.ErrG.NewError(triggerErr, "init failed, trigger query failed db[%s]", k)
		}

		if len(data) <= 0 {
			continue
		}

		for idx := range val.Query {
			initErr := initDb.RunExecute(ctx, &conntype.Args{
				Query:         val.Query[idx],
				Args:          nil,
				IsTransaction: false,
			})

			if initErr != nil {
				return errlist.ErrG.NewError(initErr, "init failed, init query failed db[%s] idx[%d]", k, idx)
			}
		}
	}

	return nil
}

func (p *PlanV1) readRunPrevBind(oldData []any, planState *PlanState, readPlan *cfg.ScriptQueryPlan) ([]any, error) {
	if readPlan.Bind.Vm != nil {
		p.initVal.vmPool.Use(func(data generic.NoImplCloser[vm.Vm]) ([][]any, error) {
			return data.Val.Do(planState.vmGlobalMap, readPlan.Bind.Vm.Script, planState.paramMap)
		})
	} else if readPlan.Bind.Prev {
		return oldData, nil
	}

	return nil, nil
}
func (p *PlanV1) readRun(planState *PlanState, seq int64) error {
	var data [][]any = nil
	var err error = nil

	for readIdx, readPlan := range p.initVal.useConf.Plans.Read {

		readP := p.initVal.readConnPool[readIdx]

		if readPlan.ReadPlan.Bind == nil {
			data, _, err = readP.RunQuery(planState.ctx, &types.Args{
				Query:         readPlan.ReadPlan.Query,
				Args:          nil,
				IsTransaction: readPlan.ReadPlan.Tran,
			})

			planState.paramMap.Store(readIdx, data)
		} else {

		}
	}

	return err
}

func (p *PlanV1) Run(planState *PlanState, isWait bool) error {
	if isWait {
		interval := time.Duration(p.initVal.useConf.Interval.Sec) * time.Second
		utils.IntervalSleep(interval, time.Millisecond*10)
	}

	if err := p.initSelf(planState); err != nil {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "")
	}

	if err := p.initOther(planState); err != nil {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "")
	}

	return nil

}

var _ Plan = (*PlanV1)(nil)
