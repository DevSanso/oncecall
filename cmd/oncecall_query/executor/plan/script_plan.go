package plan

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/cmd/oncecall_query/executor/types"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	conntype "oncecall/pool/types"
	"oncecall/utils"
	"strconv"
	"time"
)

type paramReadParamType int

const (
	paramPlanNil paramReadParamType = iota
	paramPlanFromFix
	paramPlanFromPrev
	paramPlanFromVm
)

type scriptPlan struct {
	plan   cfg.ScriptConfig
	shared types.SharedStatePlanCtl
}

func NewScriptPlan(scriptConfig cfg.ScriptConfig, shared types.SharedStatePlanCtl) Plan {
	return &scriptPlan{
		shared: shared,
		plan:   scriptConfig,
	}
}

func (s *scriptPlan) init(ctx context.Context, ident types.IdentKey, isInit bool) error {
	p := &s.plan

	if p.Init == nil && isInit {
		return nil
	}

	self, err := s.shared.GetConnPool(ident.DbID())
	if err != nil {
		return errlist.ErrG.NewError(err, "cant load self pool")
	}

	for _, initQuery := range p.Init.Self {
		data, _, triggerErr := self.RunQuery(ctx, &conntype.Args{
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

		initErr := self.RunExecute(ctx, &conntype.Args{
			Query:         initQuery.Query,
			Args:          nil,
			IsTransaction: false,
		})

		if initErr != nil {
			return errlist.ErrG.NewError(initErr, "self init failed, init query failed")
		}
	}

	for k, val := range p.Init.Other {
		initDb, err := s.shared.GetConnPool(k)
		if err != nil {
			return errlist.ErrG.NewError(err, "init get failed db[%s] target[%v]", k, ident)
		}

		data, _, triggerErr := initDb.RunQuery(ctx, &conntype.Args{
			Query:         val.TriggerQuery,
			Args:          nil,
			IsTransaction: false,
		})

		if triggerErr != nil {
			return errlist.ErrG.NewError(triggerErr, "init failed, trigger query failed db[%s] target[%v]", k, ident)
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
				return errlist.ErrG.NewError(initErr, "init failed, init query failed db[%s] target[%v] idx[%d]", k, ident, idx)
			}
		}
	}
	return nil
}

func (s *scriptPlan) makeReadPlanParam(currentPlanIdx int, state *PlanState, prevData [][]any, currentSeq int64) (dataRet [][]any, err error) {
	var t paramReadParamType
	current := s.plan.Plans.Read[currentPlanIdx].ReadPlan

	if current.Bind == nil {
		t = paramPlanNil
	} else if current.Bind.Fix != nil {
		t = paramPlanFromFix
	} else if current.Bind.PrevVm != nil {
		t = paramPlanFromVm
	} else if current.Bind.Prev == true {
		t = paramPlanFromPrev
	} else {
		err = errlist.ErrG.NewError(prefix.NotExistsError, "not setting param data")
		return
	}

	if t == paramPlanFromFix {
		var maxK = 0
		fix := current.Bind.Fix

		for k := range fix {
			conv, convErr := strconv.Atoi(k)
			if convErr != nil {
				return nil, errlist.ErrG.NewError(convErr, "only number string [%s]", k)
			}
			maxK = max(maxK, conv)
		}

		var data = make([]any, maxK)

		for k, val := range fix {
			conv, convErr := strconv.Atoi(k)
			if convErr != nil {
				return nil, errlist.ErrG.NewError(convErr, "only number string [%s]", k)
			}
			fixIdx := conv - 1

			if fixIdx < 0 {
				return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "")
			}

			if val.Static != nil {
				data[fixIdx] = *val.Static
			} else if val.Dynamic != nil {
				var err error
				data[fixIdx], err = state.ref.paramBuffer.get(val.Dynamic.PlanIdx, val.Dynamic.Col, val.Dynamic.RowIdx, currentSeq)
				if err != nil {
					return nil, errlist.ErrG.NewError(err, "")
				}
			} else {
				data[fixIdx] = nil
			}
		}

		dataRet = [][]any{data}
		err = nil
	} else if t == paramPlanFromVm {
		vmConf := current.Bind.PrevVm
		langVm, getVmErr := s.shared.GetVm(vmConf.Lang)
		if getVmErr != nil {
			return nil, errlist.ErrG.NewError(getVmErr, "")
		}
		vmData, vmErr := langVm.Do(&state.ref.scriptCacheMap, vmConf.Script, prevData)

		if vmErr != nil {
			return nil, errlist.ErrG.NewError(vmErr, "langVm Failed %s", vmConf.Script)
		}
		dataRet = vmData
		err = nil
	} else if t == paramPlanFromPrev {
		dataRet = prevData
		err = nil
	} else {
		dataRet = nil
		err = nil
	}
	return
}

func (s *scriptPlan) makeSyncPlanParam(state *PlanState, prevData [][]any, currentSeq int64) (dataRet [][]any, err error) {
	if s.plan.Plans.Sync.Bind == nil {
		return nil, nil
	}

	var t paramReadParamType = -1

	if s.plan.Plans.Sync.Bind.PrevVm != nil {
		t = paramPlanFromVm
	} else if s.plan.Plans.Sync.Bind.Fix != nil {
		t = paramPlanFromFix
	} else if s.plan.Plans.Sync.Bind.Prev == true {
		t = paramPlanFromPrev
	} else {
		err = errlist.ErrG.NewError(prefix.NotExistsError, "not setting param data")
		return
	}

	if t == paramPlanFromFix {
		fix := s.plan.Plans.Sync.Bind.Fix

		colSize, rowSize, getSizeErr := state.ref.paramBuffer.maxSize(currentSeq)
		if getSizeErr != nil {
			return nil, errlist.ErrG.NewError(getSizeErr, "failed get plan bind size [k:%v]", state.syncIdent)
		}

		param := make([][]any, rowSize)
		for idx := range rowSize {
			param[idx] = make([]any, colSize)
		}

		for k, val := range fix.Pos {
			conv, convErr := strconv.Atoi(k)
			if convErr != nil {
				return nil, errlist.ErrG.NewError(convErr, "only number string [%s]", k)
			}
			fixIdx := conv - 1
			if fixIdx < 0 {
				return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "")
			}

			if fixIdx >= colSize {
				return nil, errlist.ErrG.NewError(prefix.OutLenError, "")
			}

			if val.Static != nil {
				for idx := range rowSize {
					param[idx][fixIdx] = *val.Static
				}
			} else if val.Dynamic != nil {
				for idx := range rowSize {
					if idx < val.Dynamic.StartSyncOffset {
						continue
					}
					data, getErr := state.ref.paramBuffer.get(val.Dynamic.PlanIdx, val.Dynamic.Col, idx, currentSeq)
					if getErr != nil {
						return nil, errlist.ErrG.NewError(getErr, "get dynamic plan failed, idx[%d]", idx)
					}
					param[idx][fixIdx] = data
				}

			} else {
				err = errlist.ErrG.NewError(prefix.SentinelCatchError, "static and dynamic are nil [idx:%d]", k)
				return
			}
		}
		dataRet = param
	} else if t == paramPlanFromVm {
		vmConf := s.plan.Plans.Sync.Bind.PrevVm
		langVm, getVmErr := s.shared.GetVm(vmConf.Lang)
		if getVmErr != nil {
			return nil, errlist.ErrG.NewError(getVmErr, "")
		}
		vmData, vmErr := langVm.Do(&state.ref.scriptCacheMap, vmConf.Script, prevData)

		if vmErr != nil {
			return nil, errlist.ErrG.NewError(vmErr, "langVm Failed %s", vmConf.Script)
		}
		dataRet = vmData
		err = nil
	} else if t == paramPlanFromPrev {
		dataRet = prevData
		err = nil
	}

	return
}

func (s *scriptPlan) runRead(ctx context.Context, state *PlanState, currentSeq int64) (lastData [][]any, err error) {
	var prevData [][]any

	for planIdx := range s.plan.Plans.Read {
		select {
		case <-ctx.Done():
			break
		default:
		}

		paramData, paramErr := s.makeReadPlanParam(planIdx, state, prevData, currentSeq)
		if paramErr != nil {
			return nil, errlist.ErrG.NewError(paramErr, "")
		}

		rows, cols, isNext, getErr := s.getReadPlanDataFromConn(ctx, planIdx, paramData)
		if getErr != nil {
			return nil, errlist.ErrG.NewError(getErr, "")
		}

		if allocErr := s.allocCurrentDataToState(state, planIdx, rows, cols, currentSeq); allocErr != nil {
			return nil, errlist.ErrG.NewError(allocErr, "")
		}

		if !isNext {
			break
		}
	}

	if len(prevData) == 0 {
		return nil, nil
	}

	return prevData, nil
}

func (s *scriptPlan) runWrite(ctx context.Context, state *PlanState, data [][]any, currentSeq int64) error {
	param, paramErr := s.makeSyncPlanParam(state, data, currentSeq)
	if paramErr != nil {
		return errlist.ErrG.NewError(paramErr, "")
	}

	writeDbIdent := state.syncIdent.DbID()
	writeDB, writeDbErr := s.shared.GetConnPool(writeDbIdent)
	if writeDbErr != nil {
		return errlist.ErrG.NewError(writeDbErr, "")
	}

	execErr := writeDB.RunExecute(ctx, &conntype.Args{
		Query:         s.plan.Plans.Sync.Query,
		Args:          param,
		IsTransaction: s.plan.Plans.Sync.Tran,
	})
	if execErr != nil {
		return errlist.ErrG.NewError(execErr, "")
	}
	return nil
}

func (s *scriptPlan) getReadPlanDataFromConn(ctx context.Context, planIdx int, param [][]any) ([][]any, []string, bool, error) {
	var data [][]any
	var dCols []string
	var nextRun = false

	readPlan := s.plan.Plans.Read[planIdx]
	p, err := s.shared.GetConnPool(readPlan.ReadIdent)
	if err != nil {
		return nil, nil, false, errlist.ErrG.NewError(err, "")
	}

	queryRet, cols, queryErr := p.RunQuery(ctx, &conntype.Args{
		Query:         readPlan.ReadPlan.Query,
		Args:          param,
		IsTransaction: readPlan.ReadPlan.Tran,
	})

	if queryErr != nil {
		return nil, nil, false, errlist.ErrG.NewError(queryErr, "")
	}

	if readPlan.ReadPlan.IsNext == nil {
		nextRun = true
	} else {
		next, _, nextQueryErr := p.RunQuery(ctx, &conntype.Args{
			Query:         readPlan.ReadPlan.IsNext.Query,
			Args:          nil,
			IsTransaction: readPlan.ReadPlan.Tran,
		})

		if nextQueryErr != nil {
			return nil, nil, false, errlist.ErrG.NewError(nextQueryErr, "")
		}

		nextRun = false
		if len(next) > 0 {
			nextRun = true
		}
	}

	data = queryRet
	dCols = cols

	return data, dCols, nextRun, nil
}

func (s *scriptPlan) allocCurrentDataToState(state *PlanState, idx int, rows [][]any, cols []string, seq int64) error {
	if len(rows) <= 0 {
		return nil
	}

	if len(rows[0]) != len(cols) {
		return errlist.ErrG.NewError(prefix.NotMatchingError, "not matching %d != %d", len(rows[0]), len(cols))
	}

	for rowIdx, row := range rows {
		for dataIdx, data := range row {
			if err := state.ref.paramBuffer.set(idx, cols[dataIdx], rowIdx, data, seq); err != nil {
				return errlist.ErrG.NewError(err, "plan buffer set failed")
			}
		}
	}

	return nil
}

func (s *scriptPlan) Run(planState *PlanState, isWait bool) error {
	key := planState.syncIdent

	s.shared.SetRunningThreadState(key)
	defer s.shared.SetStopThreadState(key)

	if err := s.init(s.shared.GetContext(), planState.syncIdent, planState.ref.isInit.Load()); err != nil {
		return errlist.ErrG.NewError(err, "init failed")
	}
	planState.ref.isInit.Store(true)

	if isWait {
		interval := time.Duration(s.plan.Interval.Sec) * time.Second
		utils.IntervalSleep(interval, time.Millisecond*10)
	}
	currentSeq := time.Now().UnixMicro()

	mergeContext, cancelFn := utils.AnyContext(s.shared.GetContext(), planState.context)
	defer cancelFn()

	readLastData, readErr := s.runRead(mergeContext, planState, currentSeq)
	if readErr != nil {
		return errlist.ErrG.NewError(readErr, "read failed")
	}

	if writeErr := s.runWrite(mergeContext, planState, readLastData, currentSeq); writeErr != nil {
		return errlist.ErrG.NewError(writeErr, "")
	}

	planState.seq = currentSeq
	return nil
}
