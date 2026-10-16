package plan

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/cmd/oncecall_query/executor/types"
	"strconv"
	"sync/atomic"

	"oncecall/errlist"
	"oncecall/errlist/prefix"
	conn_type "oncecall/pool/types"
	"oncecall/utils"
	"oncecall/utils/generic"
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
	key            types.IdentKey
	plan           cfg.ScriptConfig
	scriptCacheMap *generic.SyncMap[string, any]

	shared types.SharedState

	isInit      atomic.Bool
	paramBuffer planParamBuffer
	seq         int64
}

func NewScriptPlan(key types.IdentKey, scriptConfig cfg.ScriptConfig, shared types.SharedState) types.Plan {
	var planParamBuf planParamBuffer
	{
		var colsSize = 10
		var rowCap = 100
		var idleRowMem = 60

		if opt := scriptConfig.Option; opt != nil {
			colsSize = opt.PlanColsBufAlloc
			rowCap = opt.PlanRowBufCap
			idleRowMem = opt.PlanRowBufIdleTime
		}

		planParamBuf = newPlanParamBuffer(len(scriptConfig.Plans.Read), colsSize, rowCap, idleRowMem)
	}

	return &scriptPlan{
		shared:         shared,
		key:            key,
		plan:           scriptConfig,
		scriptCacheMap: generic.NewGenericSyncMap[string, any](),
		paramBuffer:    planParamBuf,
	}
}

func (s *scriptPlan) init(ctx context.Context) error {
	p := &s.plan
	if p.Init == nil && !s.isInit.Load() {
		return nil
	}

	self, err := s.shared.GetConnPool(s.key.First)
	if err != nil {
		return errlist.ErrG.NewError(err, "cant load self pool")
	}

	for _, initQuery := range p.Init.Self {
		data, _, triggerErr := self.RunQuery(ctx, &conn_type.Args{
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

		initErr := self.RunExecute(ctx, &conn_type.Args{
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
			return errlist.ErrG.NewError(err, "init get failed db[%s] target[%v]", k, s.key)
		}

		data, _, triggerErr := initDb.RunQuery(ctx, &conn_type.Args{
			Query:         val.TriggerQuery,
			Args:          nil,
			IsTransaction: false,
		})

		if triggerErr != nil {
			return errlist.ErrG.NewError(triggerErr, "init failed, trigger query failed db[%s] target[%v]", k, s.key)
		}

		if len(data) <= 0 {
			continue
		}

		for idx := range val.Query {
			initErr := initDb.RunExecute(ctx, &conn_type.Args{
				Query:         val.Query[idx],
				Args:          nil,
				IsTransaction: false,
			})

			if initErr != nil {
				return errlist.ErrG.NewError(initErr, "init failed, init query failed db[%s] target[%v] idx[%d]", k, s.key, idx)
			}
		}
	}

	s.isInit.Store(true)
	return nil
}

func (s *scriptPlan) makeReadPlanParam(currentPlanIdx int, buf *planParamBuffer, prevData [][]any, currentSeq int64) (dataRet [][]any, err error) {
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
				data[fixIdx], err = buf.get(val.Dynamic.PlanIdx, val.Dynamic.Col, val.Dynamic.RowIdx, currentSeq)
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
		vmData, vmErr := langVm.Do(s.scriptCacheMap, vmConf.Script, prevData)

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

func (s *scriptPlan) makeSyncPlanParam(buf *planParamBuffer, prevData [][]any, currentSeq int64) (dataRet [][]any, err error) {
	if s.plan.Plans.Sync.Bind == nil {
		return nil, nil
	}

	var t paramReadParamType

	if s.plan.Plans.Sync.Bind.PrevVm != nil {
		t = paramPlanFromVm
	} else if s.plan.Plans.Sync.Bind.Fix != nil {
		t = paramPlanFromFix
	} else if s.plan.Plans.Sync.Bind.Prev == true {
		t= paramPlanFromPrev
	} else {
		err = errlist.ErrG.NewError(prefix.NotExistsError, "not setting param data")
		return
	}

	if t == paramPlanFromFix {
		fix := s.plan.Plans.Sync.Bind.Fix

		colSize, rowSize, getSizeErr := buf.maxSize(currentSeq)
		if getSizeErr != nil {
			return nil, errlist.ErrG.NewError(getSizeErr, "failed get plan bind size [k:%v]", s.key)
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
					data, getErr := buf.get(val.Dynamic.PlanIdx, val.Dynamic.Col, idx, currentSeq)
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
		vmData, vmErr := langVm.Do(s.scriptCacheMap, vmConf.Script, prevData)

		if vmErr != nil {
			return nil, errlist.ErrG.NewError(vmErr, "langVm Failed %s", vmConf.Script)
		}
		dataRet = vmData
		err = nil
	} else if t == paramPlanFromPrev{
		dataRet = prevData
		err = nil
	}

	return
}

func (s *scriptPlan) runRead(ctx context.Context, currentSeq int64) (lastData [][]any, err error) {
	var prevData [][]any

	for planIdx := range s.plan.Plans.Read {
		select {
		case <-ctx.Done():
			break
		default:
		}

		paramData, paramErr := s.makeReadPlanParam(planIdx, &s.paramBuffer, prevData, currentSeq)
		if paramErr != nil {
			return nil, errlist.ErrG.NewError(paramErr, "")
		}

		rows, cols, isNext, getErr := s.getReadPlanDataFromConn(ctx, planIdx, paramData)
		if getErr != nil {
			return nil, errlist.ErrG.NewError(getErr, "")
		}

		if allocErr := s.allocCurrentData(planIdx, rows, cols, currentSeq); allocErr != nil {
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

func (s *scriptPlan) runWrite(ctx context.Context, data [][]any, currentSeq int64) error {
	param, paramErr := s.makeSyncPlanParam(&s.paramBuffer, data, currentSeq)
	if paramErr != nil {
		return errlist.ErrG.NewError(paramErr, "")
	}

	writeDbIdent := s.key.First
	writeDB, writeDbErr := s.shared.GetConnPool(writeDbIdent)
	if writeDbErr != nil {
		return errlist.ErrG.NewError(writeDbErr, "")
	}

	execErr := writeDB.RunExecute(ctx, &conn_type.Args{
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

	queryRet, cols, queryErr := p.RunQuery(ctx, &conn_type.Args{
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
		next, _, nextQueryErr := p.RunQuery(ctx, &conn_type.Args{
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

func (s *scriptPlan) allocCurrentData(idx int, rows [][]any, cols []string, seq int64) error {
	if len(rows) <= 0 {
		return nil
	}

	if len(rows[0]) != len(cols) {
		return errlist.ErrG.NewError(prefix.NotMatchingError, "not matching %d != %d", len(rows[0]), len(cols))
	}

	for rowIdx, row := range rows {
		for dataIdx, data := range row {
			if err := s.paramBuffer.set(idx, cols[dataIdx], rowIdx, data, seq); err != nil {
				return errlist.ErrG.NewError(err, "plan buffer set failed")
			}
		}
	}

	return nil
}

func (s *scriptPlan) Run(ctx context.Context, isWait bool) error {
	if isRun, exists := s.shared.GetThreadState(s.key); isRun || exists {
		return errlist.ErrG.NewError(prefix.CreateError, "already run")
	}

	s.shared.SetRunningThreadState(s.key)
	defer s.shared.SetStopThreadState(s.key)

	if isWait {
		interval := time.Duration(s.plan.Interval.Sec) * time.Second
		utils.IntervalSleep(interval, time.Millisecond*10)
	}

	if err := s.init(s.shared.GetContext()); err != nil {
		return errlist.ErrG.NewError(err, "init failed")
	}

	currentSeq := time.Now().UnixMicro()

	mergeContext, cancelFn := utils.AnyContext(s.shared.GetContext(), ctx)
	defer cancelFn()

	readLastData, readErr := s.runRead(mergeContext, currentSeq)
	if readErr != nil {
		return errlist.ErrG.NewError(readErr, "read failed")
	}

	if writeErr := s.runWrite(mergeContext, readLastData, currentSeq); writeErr != nil {
		return errlist.ErrG.NewError(writeErr, "")
	}

	s.seq = currentSeq


	return nil
}

/**
import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"strconv"
	"time"
)

type scriptThread struct {
	state      *execSharedState
	key        generic.Pair[string, string]
	vmCacheMap *generic.SyncMap[string, any]
}

func newScriptThread(key generic.Pair[string, string], state *execSharedState) *scriptThread {

	return &scriptThread{
		state:      state,
		key:        key,
		vmCacheMap: generic.NewGenericSyncMap[string, any](),
	}
}

func (s *scriptThread) init(self types.ConnPoolCloser, ctx context.Context, cfg *cfg.ScriptConfig) error {
	if cfg.Init == nil {
		return nil
	}

	for _, initQuery := range cfg.Init.Self {
		data, _, triggerErr := self.RunQuery(ctx, &types.Args{
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

		initErr := self.RunExecute(ctx, &types.Args{
			Query:         initQuery.Query,
			Args:          nil,
			IsTransaction: false,
		})

		if initErr != nil {
			return errlist.ErrG.NewError(initErr, "self init failed, init query failed")
		}
	}

	for k, val := range cfg.Init.Other {
		initDb, exists := s.state.pMap.Load(k)
		if !exists {
			return errlist.ErrG.NewError(prefix.NotExistsError, "init failed, not exists db[%s] target[%v]", k, s.key)
		}

		data, _, triggerErr := initDb.RunQuery(ctx, &types.Args{
			Query:         val.TriggerQuery,
			Args:          nil,
			IsTransaction: false,
		})

		if triggerErr != nil {
			return errlist.ErrG.NewError(triggerErr, "init failed, trigger query failed db[%s] target[%v]", k, s.key)
		}

		if len(data) <= 0 {
			continue
		}

		for idx := range val.Query {
			initErr := initDb.RunExecute(ctx, &types.Args{
				Query:         val.Query[idx],
				Args:          nil,
				IsTransaction: false,
			})

			if initErr != nil {
				return errlist.ErrG.NewError(initErr, "init failed, init query failed db[%s] target[%v] idx[%d]", k, s.key, idx)
			}
		}
	}

	return nil
}

func (s *scriptThread) makeParamFromSyncPlanBindFix(p *cfg.ScriptQuerySyncPlan, buf *planParamBuffer, seq int64) ([][]any, error) {
	fix := p.Bind.Fix

	colSize, rowSize, getSizeErr := buf.maxSize(seq)
	if getSizeErr != nil {
		return nil, errlist.ErrG.NewError(getSizeErr, "failed get plan bind size [k:%v]", s.key)
	}

	param := make([][]any, rowSize)
	for idx := 0; idx < rowSize; idx += 1 {
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
			for idx := 0; idx < rowSize; idx += 1 {
				param[idx][fixIdx] = *val.Static
			}
		} else if val.Dynamic != nil {
			for idx := 0; idx < rowSize; idx += 1 {
				if idx < val.Dynamic.StartSyncOffset {
					continue
				}
				data, getErr := buf.get(val.Dynamic.PlanIdx, val.Dynamic.Col, idx, seq)
				if getErr != nil {
					return nil, errlist.ErrG.NewError(getErr, "get dynamic plan failed, idx[%d]", idx)
				}
				param[idx][fixIdx] = data
			}

		} else {
			return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "static and dynamic are nil [idx:%d]", k)
		}
	}

	return param, nil
}

func (s *scriptThread) makeParamFromSyncPlanBindVm(p *cfg.ScriptQuerySyncPlan, data [][]any) ([][]any, error) {
	vmConf := p.Bind.Vm
	langVm := s.state.vmP.Get()

	vmData, vmErr := langVm.Do(s.vmCacheMap, vmConf.Script, data)

	if vmErr != nil {
		return nil, errlist.ErrG.NewError(vmErr, "langVm Failed %s", vmConf.Script)
	}

	return vmData, nil
}

func (s *scriptThread) makeParamFromReadPlanBindFix(p *cfg.ScriptQueryReadPlan, buf *planParamBuffer, seq int64) ([][]any, error) {
	var maxK = 0
	fix := p.Bind.Fix

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
			data[fixIdx], err = buf.get(val.Dynamic.PlanIdx, val.Dynamic.Col, val.Dynamic.RowIdx, seq)
			if err != nil {
				return nil, errlist.ErrG.NewError(err, "")
			}
		} else {
			data[fixIdx] = nil
		}
	}

	return [][]any{data}, nil
}

func (s *scriptThread) makeParamFromReadPlanBindVm(p *cfg.ScriptQueryReadPlan, data [][]any) ([][]any, error) {
	vmConf := p.Bind.Vm

	langVm := s.state.vmP.Get()
	vmData, vmErr := langVm.Do(s.vmCacheMap, vmConf.Script, data)

	if vmErr != nil {
		return nil, errlist.ErrG.NewError(vmErr, "langVm Failed %s", vmConf.Script)
	}

	return vmData, nil
}

func (*scriptThread) allocCurrentData(idx int, rows [][]any, cols []string, buf *planParamBuffer, seq int64) error {
	if len(rows) <= 0 {
		return nil
	}

	if len(rows[0]) != len(cols) {
		return errlist.ErrG.NewError(prefix.NotMatchingError, "not matching %d != %d", len(rows[0]), len(cols))
	}

	for rowIdx, row := range rows {
		for dataIdx, data := range row {
			if err := buf.set(idx, cols[dataIdx], rowIdx, data, seq); err != nil {
				return errlist.ErrG.NewError(err, "plan buffer set failed")
			}
		}
	}

	return nil
}

func (s *scriptThread) getDataFromConn(ctx context.Context, p types.ConnPoolCloser, scriptPlan *cfg.ScriptQueryReadPlan, param [][]any) ([][]any, []string, bool, error) {
	var data [][]any
	var dCols []string
	var nextRun = false

	queryRet, cols, queryErr := p.RunQuery(ctx, &types.Args{
		Query:         scriptPlan.Query,
		Args:          param,
		IsTransaction: scriptPlan.Tran,
	})

	if scriptPlan.IsNext == nil {
		nextRun = true
	} else {
		next, _, nextQueryErr := p.RunQuery(ctx, &types.Args{
			Query:         scriptPlan.Query,
			Args:          param,
			IsTransaction: scriptPlan.Tran,
		})

		if nextQueryErr != nil {
			return nil, nil, false, errlist.ErrG.NewError(queryErr, "")
		}

		nextRun = false
		if len(next) > 0 {
			nextRun = true
		}
	}

	if queryErr != nil {
		return nil, nil, false, errlist.ErrG.NewError(queryErr, "")
	}

	data = queryRet
	dCols = cols

	return data, dCols, nextRun, nil
}

func (s *scriptThread) Run(ctx context.Context) error {
	db, dbOk := s.state.pMap.Load(s.key.First)
	scriptPair, scriptOk := s.state.scriptMap.Load(s.key.Second)

	if !(dbOk && scriptOk) {
		return errlist.ErrG.NewError(nil, "can't load conn or script [%t:%t]", dbOk, scriptOk)
	}

	script := scriptPair.Second

	if err := s.init(db, ctx, script); err != nil {
		return errlist.ErrG.NewError(err, "init failed")
	}

	var planParamBuf planParamBuffer
	{
		var colsSize = 10
		var rowCap = 100
		var idleRowMem = 60

		if opt := script.Option; opt != nil {
			colsSize = opt.PlanColsBufAlloc
			rowCap = opt.PlanRowBufCap
			idleRowMem = opt.PlanRowBufIdleTime
		}

		planParamBuf = newPlanParamBuffer(len(script.Plans.Read), colsSize, rowCap, idleRowMem)
	}

RunLoop:
	for {
		var data [][]any = nil
		var dataErr error = nil

		select {
		case <-ctx.Done():
			break RunLoop
		default:
		}
		seq := time.Now().UnixMicro()

		for planIdx := range script.Plans.Read {
			select {
			case <-ctx.Done():
				break RunLoop
			default:
			}

			currentPlan := &script.Plans.Read[planIdx]

			if script.Plans.Read[planIdx].ReadPlan.Bind.Fix != nil {
				data, dataErr = s.makeParamFromReadPlanBindFix(
					&script.Plans.Read[planIdx].ReadPlan, &planParamBuf, seq)
			} else if script.Plans.Read[planIdx].ReadPlan.Bind.Fix != nil {
				data, dataErr = s.makeParamFromReadPlanBindVm(&script.Plans.Read[planIdx].ReadPlan, data)
			} else {
				data = nil
				dataErr = nil
			}

			if dataErr != nil {
				return errlist.ErrG.NewError(dataErr, "")
			}

			currentP, isGetP := s.state.pMap.Load(currentPlan.ReadIdent)
			if isGetP {
				return errlist.ErrG.NewError(nil, "get pool failed [plan:%v, idx:%d, p:%s]", s.key, planIdx, currentPlan.ReadIdent)
			}

			currentData, currentCols, nextRun, runErr := s.getDataFromConn(ctx, currentP, &currentPlan.ReadPlan, data)
			if runErr != nil {
				return errlist.ErrG.NewError(runErr, "get pool run failed [plan:%v, idx:%d, p:%s]", s.key, planIdx, currentPlan.ReadIdent)
			}

			if allocErr := s.allocCurrentData(planIdx, currentData, currentCols, &planParamBuf, seq); allocErr != nil {
				return errlist.ErrG.NewError(allocErr, "alloc failed")
			}

			if !nextRun {
				break
			}
		}

		if script.Plans.Sync.Bind.Fix != nil {
			data, dataErr = s.makeParamFromSyncPlanBindFix(
				&script.Plans.Sync, &planParamBuf, seq)
		} else if script.Plans.Sync.Bind.Vm != nil {
			data, dataErr = s.makeParamFromSyncPlanBindVm(&script.Plans.Sync, data)
		} else {
			data = nil
			dataErr = nil
		}

		if dataErr != nil {
			return errlist.ErrG.NewError(dataErr, "")
		}

		if syncErr := db.RunExecute(ctx, &types.Args{
			Query:         script.Plans.Sync.Query,
			Args:          data,
			IsTransaction: script.Plans.Sync.Tran,
		}); syncErr != nil {
			return errlist.ErrG.NewError(syncErr, "execute sync failed")
		}
	}

	return nil
}

*/
