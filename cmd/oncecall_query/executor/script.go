package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/conn"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"os"
	"time"
)

type planParamBuffer struct {
	m      []map[string][]struct{data any; seq int64}
	rowCap int
	rowIdleSec int
}

func newPlanParamBuffer(count int, mapInitSize int, rowInitCap int, rowIdleSec int) planParamBuffer {
	obj := make([]map[string][]struct{data any; seq int64}, count)
	for idx := range obj {
		obj[idx] = make(map[string][]struct{data any; seq int64}, mapInitSize)
	}

	return planParamBuffer{
		m:      obj,
		rowCap: rowInitCap,
		rowIdleSec: rowIdleSec,
	}
}

func (ppb *planParamBuffer) Get(idx int, col string, row int, seq int64) any {
	if len(ppb.m) <= idx {
		return nil
	}

	planMap := ppb.m[idx]
	if len(planMap[col]) <= row {
		return nil
	}

	if planMap[col][row].seq > seq {
		return nil
	}

	return planMap[col][row].data
}

func (ppb *planParamBuffer) Set(idx int, col string, row int, data any, seq int64) error {
	if len(ppb.m) <= idx {
		return errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]

	if len(planMap[col]) <= row {
		temp := make([]struct{data any; seq int64}, len(planMap[col]) + ppb.rowCap)
		copy(temp, planMap[col])
		planMap[col] = temp
	} else {
		idle := time.Duration(ppb.rowIdleSec)
		if idle <= 0 { idle = 60 }

		if seq - planMap[col][len(planMap[col])-1].seq > int64(time.Second * idle) {
			temp := make([]struct{data any; seq int64}, row + ppb.rowCap)
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

type scriptThread struct {
	state *execSharedState
	key   generic.Pair[string, string]
}

func newScriptThread(key generic.Pair[string, string], state *execSharedState) *scriptThread {

}

func (s *scriptThread) init(ctx context.Context, cfg *cfg.ScriptConfig) error {
	for k, val := range cfg.Init {
		initDb, exists := s.state.pMap.Load(k)
		if !exists {
			return errlist.ErrG.NewError(nil, "init failed, not exists db[%s] target[%v]", k, s.key)
		}

		data, _, triggerErr := initDb.RunQuery(ctx, &conn.Args{
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
			initErr := initDb.RunExecute(ctx, &conn.Args{
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

func (s *scriptThread) makeParamFromPlanBindCfg(p *cfg.ScriptQueryPlan, buf *planParamBuffer, seq int64) ([][]any, error) {
	var maxK int = 0
	var maxPlanIdx int = 0

	for k, val := range p.Bind {
		maxK = max(maxK, k)

		if val.Dynamic != nil {
			maxPlanIdx = max(maxPlanIdx, val.Dynamic.PlanIdx)
		}
	}


	for k, val := range p.Bind {

	}


}

func (*scriptThread) allocCurrentData(idx int, rows [][]any, cols []string, buf *planParamBuffer, seq int64) error {
	if len(rows) <= 0 {
		return nil
	}

	if len(rows[0]) != len(cols) {
		return errlist.ErrG.NewError(nil, "not matching %d != %d", len(rows[0]), len(cols))
	}

	for rowIdx, row := range rows {
		for dataIdx, data := range row {
			if err := buf.Set(idx, cols[dataIdx], rowIdx, data, seq); err != nil {
				return errlist.ErrG.NewError(err, "plan buffer set failed")
			}
		}
	}

	return nil
}

func (s *scriptThread) connRun(ctx context.Context, p conn.ConnPoolInterface, scriptPlan *cfg.ScriptQueryPlan, param [][]any) ([][]any, []string, error) {
	var data [][]any
	var dCols []string

	queryRet, cols, queryErr := p.RunQuery(ctx, &conn.Args{
		Query:         scriptPlan.Query,
		Args:          param,
		IsTransaction: scriptPlan.Tran,
	})

	if queryErr != nil {
		return nil, nil, errlist.ErrG.NewError(queryErr, "")
	}

	data = queryRet
	dCols = cols

	return data, dCols, nil
}

func (s *scriptThread) Run(ctx context.Context) error {
	db, dbOk := s.state.pMap.Load(s.key.First)
	scriptPair, scriptOk := s.state.scriptMap.Load(s.key.Second)
	
	if !( dbOk && scriptOk) {
		return errlist.ErrG.NewError(nil, "can't load conn or script [%t:%t]", dbOk, scriptOk)
	}

	script := scriptPair.Second

	if err := s.init(ctx, script); err != nil {
		return errlist.ErrG.NewError(err, "init failed")
	}

	var planParamBuf planParamBuffer
	{
		var colsSize = 10
		var rowCap = 100

		if opt := script.Option; opt != nil {
			colsSize = opt.PlanColsBufAlloc
			rowCap = opt.PlanRowBufCap
		}

		planParamBuf = newPlanParamBuffer(len(script.Plans.Read), colsSize, rowCap)
	}
	vmCacheMap := generic.NewGenericSyncMap[string, any]()

	RunLoop:
	for {
		select {
		case <-ctx.Done():
			break RunLoop
		default:
		}
		seq := time.Now().UnixMicro()

		for planIdx := range script.Plans.Read {
			currentPlan := &script.Plans.Read[planIdx]

			data, paramBufErr := s.makeParamFromPlanBindCfg(&currentPlan.SubPlan, &planParamBuf, seq)
			if paramBufErr != nil {
				return errlist.ErrG.NewError(paramBufErr, "paramBuf make failed [idx:%d]", planIdx)
			}

			if vmCfg := currentPlan.SubPlan.Vm; vmCfg != nil {
				beforeData := data
				langVm := s.state.vmP.Get()
				var vmErr error
				data, vmErr = langVm.Do(vmCacheMap, vmCfg.Script, beforeData)

				if vmErr != nil {
					return errlist.ErrG.NewError(vmErr, "langVm Failed %s", vmCfg.Script)
				}
			}

			currentP, isGetP := s.state.pMap.Load(currentPlan.ReadIdent)
			if isGetP {
				return errlist.ErrG.NewError(nil, "get pool failed [plan:%v, idx:%d, p:%s]", s.key, planIdx, currentPlan.ReadIdent)
			}

			currentData, currentCols, runErr := s.connRun(ctx, currentP, &currentPlan.SubPlan, data)
			if runErr != nil {
				return errlist.ErrG.NewError(runErr, "get pool run failed [plan:%v, idx:%d, p:%s]", s.key, planIdx, currentPlan.ReadIdent)
			}

			if allocErr := s.allocCurrentData(planIdx, currentData, currentCols, &planParamBuf, seq); allocErr != nil {
				return errlist.ErrG.NewError(allocErr, "alloc failed")
			}
		}

		data, paramBufErr := s.makeParamFromPlanBindCfg(&script.Plans.Sync, &planParamBuf, seq)
		if paramBufErr != nil {
			return errlist.ErrG.NewError(paramBufErr, "paramBuf make failed sync")
		}

		if vmCfg := script.Plans.Sync.Vm; vmCfg != nil {
			beforeData := data
			langVm := s.state.vmP.Get()
			var vmErr error
			data, vmErr = langVm.Do(vmCacheMap, vmCfg.Script, beforeData)

			if vmErr != nil {
				return errlist.ErrG.NewError(vmErr, "sync langVm Failed %s", vmCfg.Script)
			}
		}

		if syncErr := db.RunExecute(ctx, &conn.Args{
			Query: script.Plans.Sync.Query,
			Args:  data,
			IsTransaction: script.Plans.Sync.Tran,
		}); syncErr != nil {
			return errlist.ErrG.NewError(syncErr, "execute sync failed")
		}
	}

	return nil
}
