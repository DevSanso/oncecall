package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"os"
	"strconv"
	"time"
)

type planParamBuffer struct {
	m []map[string][]struct {
		data any
		seq  int64
	}
	rowCap     int
	rowIdleSec int
}

func newPlanParamBuffer(count int, mapInitSize int, rowInitCap int, rowIdleSec int) planParamBuffer {
	obj := make([]map[string][]struct {
		data any
		seq  int64
	}, count)
	for idx := range obj {
		obj[idx] = make(map[string][]struct {
			data any
			seq  int64
		}, mapInitSize)
	}

	return planParamBuffer{
		m:          obj,
		rowCap:     rowInitCap,
		rowIdleSec: rowIdleSec,
	}
}

func (ppb *planParamBuffer) get(idx int, col string, row int, seq int64) (any, error) {
	if len(ppb.m) <= idx {
		return nil, nil
	}

	planMap := ppb.m[idx]
	if len(planMap[col]) <= row {
		return nil, errlist.ErrG.NewError(prefix.OutLenError, "plan len :%d, search idx : %d", len(planMap[col]), row)
	}

	if planMap[col][row].seq > seq {
		return nil, nil
	}

	return planMap[col][row].data, nil
}

func (ppb *planParamBuffer) set(idx int, col string, row int, data any, seq int64) error {
	if len(ppb.m) <= idx {
		return errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]

	if len(planMap[col]) <= row {
		temp := make([]struct {
			data any
			seq  int64
		}, len(planMap[col])+ppb.rowCap)
		copy(temp, planMap[col])
		planMap[col] = temp
	} else {
		idle := time.Duration(ppb.rowIdleSec)
		if idle <= 0 {
			idle = 60
		}

		if seq-planMap[col][len(planMap[col])-1].seq > int64(time.Second*idle) {
			temp := make([]struct {
				data any
				seq  int64
			}, row+ppb.rowCap)
			copy(temp, planMap[col])
			planMap[col] = temp
		}
	}

	planMap[col][row] = struct {
		data any
		seq  int64
	}{data: data, seq: seq}

	return nil
}

func (ppb *planParamBuffer) maxColSize(idx int, seq int64) (int, error) {
	if len(ppb.m) <= idx {
		return -1, errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]
	return len(planMap), nil
}

func (ppb *planParamBuffer) maxRowSize(idx int, seq int64) (int, error) {
	if len(ppb.m) <= idx {
		return -1, errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]

	cnt := 0
	for _, val := range planMap {
		cnt = max(cnt, len(val))
	}

	return cnt, nil
}

func (ppb *planParamBuffer) maxSize(seq int64) (cols int, row int, err error) {
	maxC := 0
	maxR := 0

	for idx := range ppb.m {
		c, cerr := ppb.maxColSize(idx, seq)
		if cerr != nil {
			return -1, -1, errlist.ErrG.NewError(cerr, "")
		}
		r, rerr := ppb.maxRowSize(idx, seq)
		if rerr != nil {
			return -1, -1, errlist.ErrG.NewError(rerr, "")
		}

		maxC = max(maxC, c)
		maxR = max(maxR, r)
	}

	return maxC, maxR, nil
}

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

func (s *scriptThread) init(self types.ConnPoolInterface, ctx context.Context, cfg *cfg.ScriptConfig) error {
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

func (s *scriptThread) getDataFromConn(ctx context.Context, p types.ConnPoolInterface, scriptPlan *cfg.ScriptQueryReadPlan, param [][]any) ([][]any, []string, bool, error) {
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
