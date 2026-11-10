package plan

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
		initFlag atomic.Bool
		once     sync.Once
		useConf cfg.ScriptConfig
		sqlFlag bool

		readConnPool         []conntype.ConnPool
		readConnPoolIndexMap map[string]int

		syncPool conntype.ConnPool
		vmPool pool.PoolExtension[generic.NoImplCloser[vm.Vm], [][]any]
	}
}

func (p *PlanV1) Close() error {
	return nil
}

func (p *PlanV1) Init(s state.SharedStatePlanCtl, conf cfg.ScriptConfig) error {
	if !p.initVal.initFlag.Load() {
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

			if conf.Plans.Read[idx].ReadPlan.Bind != nil && conf.Plans.Read[idx].ReadPlan.Bind.SQL != nil {
				p.initVal.sqlFlag = true
			}

			p.initVal.readConnPool = append(p.initVal.readConnPool, readP)
			p.initVal.readConnPoolIndexMap[ident] = len(p.initVal.readConnPool) - 1
		}
		syncP, syncErr := s.GetConnPool(config.SyncIdent)
		if syncErr != nil {
			err = errlist.ErrG.NewError(prefix.SentinelCatchError, "not find pool %s", syncErr)
			return
		}

		p.initVal.syncPool = syncP
		p.initVal.useConf = conf
		p.initVal.initFlag.Store(true)
	})

	return err
}

func (p *PlanV1) initSelf(planState *PlanState) error {
	ctx := planState.ctx
	if planState.isInitSelf.Load() {
		return nil
	}

	for _, initQuery := range p.initVal.useConf.Init.Self {
		data, _,_, triggerErr := p.initVal.syncPool.RunQuery(ctx, &conntype.Args{
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
		data, _,_, triggerErr := initDb.RunQuery(ctx, &conntype.Args{
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

func (p *PlanV1) runBind(oldData [][]any, planState *PlanState, qPlan *cfg.ScriptQueryPlan) ([][]any, error) {
	if qPlan.Bind.SQL != nil {
		return planState.sqlDB.Select(planState.ctx, qPlan.Bind.SQL.Query, qPlan.Bind.SQL.IsTransaction)
	} else 	if qPlan.Bind.Vm != nil {
		vmData, vmErr := p.initVal.vmPool.Use(func(data generic.NoImplCloser[vm.Vm]) ([][]any, error) {
			return data.Val.Do(planState.vmGlobalMap, qPlan.Bind.Vm.Script, planState.paramMap)
		})
		return vmData, vmErr
	} else if qPlan.Bind.Prev {
		return oldData, nil
	}

	return nil, nil
}

func (p *PlanV1) readRun(planState *PlanState) error {
	var data [][]any = nil
	var names []string = nil
	var colType []types.ConnPoolColType = nil
	var err error = nil

	for readIdx, readPlan := range p.initVal.useConf.Plans.Read {

		readP := p.initVal.readConnPool[readIdx]

		if readPlan.ReadPlan.Bind == nil {
			data, names, colType, err = readP.RunQuery(planState.ctx, &types.Args{
				Query:         readPlan.ReadPlan.Query,
				Args:          nil,
				IsTransaction: readPlan.ReadPlan.Tran,
			})
		} else {
			var useData [][]any
			var useErr error

			useData, useErr = p.runBind(data, planState, &readPlan.ReadPlan)

			if useErr != nil {
				return useErr
			}

			data, names, colType, err = readP.RunQuery(planState.ctx, &types.Args{
				Query:         readPlan.ReadPlan.Query,
				Args:          useData,
				IsTransaction: readPlan.ReadPlan.Tran,
			})
		}

		if err != nil {
			return err
		}

		if p.initVal.sqlFlag {
			if err = planState.sqlDB.CreateCacheTable(readIdx, names, colType); err != nil {
				return err
			}

			if err = planState.sqlDB.InsertPlanData(readIdx, data); err != nil {
				return err
			}
		}

		planState.paramMap.Store(readIdx, data)
	}

	return err
}

func (p *PlanV1) syncRun(planState *PlanState) error {
	data, err := p.runBind(nil, planState, &p.initVal.useConf.Plans.Sync)
	if err != nil {
		return err
	}

	return p.initVal.syncPool.RunExecute(planState.ctx, &conntype.Args{
		Query: p.initVal.useConf.Plans.Sync.Query,
		Args:  data,
		IsTransaction: p.initVal.useConf.Plans.Sync.Tran,
	})
}

func (p *PlanV1) Run(planState *PlanState, isWait bool) error {
	if isWait {
		interval := time.Duration(p.initVal.useConf.Interval.Sec) * time.Second
		utils.IntervalSleep(interval, time.Millisecond*10)
	}

	defer planState.paramMap.Clear()

	if err := p.initSelf(planState); err != nil {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "")
	}

	if err := p.initOther(planState); err != nil {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "")
	}

	if err := p.readRun(planState); err != nil {return errlist.ErrG.NewError(prefix.SentinelCatchError, "")}

	if err := p.syncRun(planState); err != nil {return errlist.ErrG.NewError(prefix.SentinelCatchError, "")}

	return nil

}

var _ Plan = (*PlanV1)(nil)
