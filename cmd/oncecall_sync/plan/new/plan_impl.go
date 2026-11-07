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
	"strconv"
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
func (p *PlanV1) readRunFixBind(oldData []any, planState *PlanState, readPlan *cfg.ScriptQueryReadPlan) ([]any, error) {
	if readPlan.Bind.Fix != nil && len(readPlan.Bind.Fix) > 0 {
		var bindData []any = make([]any, 10)
		var upsizeBindDataFn = func(resize int) {
			if len(bindData) >= resize {
				return
			}

			newData := make([]any, resize)
			copy(newData[:len(bindData)], bindData)
			bindData = newData
		}

		for colPos, val := range readPlan.Bind.Fix {
			colPosNum, convErr := strconv.Atoi(colPos)
			if convErr != nil {
				return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "")
			}
			if colPosNum <= 0 {
				return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "")
			}
			upsizeBindDataFn(colPosNum)

			if val.Static != nil {
				bindData[colPosNum-1] = *val.Static
			} else if val.Dynamic != nil {
				getData, getErr := planState.planBuf.Get(val.Dynamic.PlanIdx, val.Dynamic.Col, val.Dynamic.RowIdx, 0)
				if getErr != nil {
					return nil, errlist.ErrG.NewError(getErr, "")
				}
				bindData[colPosNum-1] = getData
			} else {
				bindData[colPosNum-1] = nil
			}
		}
		return bindData, nil
	}

	return oldData, nil
}
func (p *PlanV1) readRunPrevBind(oldData []any, planState *PlanState, readPlan *cfg.ScriptQueryReadPlan) ([]any, error) {
	if readPlan.Bind.PrevVm != nil {
		p.initVal.vmPool.Use(func(data generic.NoImplCloser[vm.Vm]) ([][]any, error) {
			data.Val.Do(planState.vmCacheMap, readPlan.Bind.PrevVm.Script, [][]any{oldData})
		})
	} else if readPlan.Bind.Prev {
		return oldData, nil
	}

	return nil, nil
}
func (p *PlanV1) readRun(planState *PlanState, seq int64) error {
	var data [][]any = nil
	var name []string = nil
	var err error = nil
	var currentData []any = nil

	for readIdx, readPlan := range p.initVal.useConf.Plans.Read {

		readP := p.initVal.readConnPool[readIdx]

		if readPlan.ReadPlan.Bind == nil {
			data, name, err = readP.RunQuery(planState.ctx, &types.Args{
				Query:         readPlan.ReadPlan.Query,
				Args:          nil,
				IsTransaction: readPlan.ReadPlan.Tran,
			})
			if err = planState.planBuf.SetTable(readIdx, name, data, seq); err != nil {
				return errlist.ErrG.NewError(prefix.SentinelCatchError, "")
			}
		} else {
			var bindData []any = nil
			var bindErr error = nil

			bindData, bindErr = p.readRunFixBind(currentData, planState, &readPlan.ReadPlan)
			if bindErr != nil {
				return errlist.ErrG.NewError(bindErr, "")
			}
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
