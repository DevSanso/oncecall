package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	selftype "oncecall/cmd/oncecall_query/executor/types"
	"oncecall/cmd/oncecall_query/executor/plan"
	"oncecall/cmd/oncecall_query/global"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"time"
)
type executeorV1IntervalSec int

type executorV1PlanState struct {
	existsPlanSet map[selftype.IdentKey]struct{}
}
type ExecutorV1 struct {
	fetcher selftype.DbFetcher

	newRunQ         []struct {
		ident selftype.IdentKey
		conf cfg.ScriptConfig
	}
	stopRunQ        []selftype.IdentKey
	threadStopFnMap  map[selftype.IdentKey]context.CancelFunc
	executorCancelFn context.CancelFunc

	sharedState selftype.SharedState

	cache struct {
		dbConf []cfg.ConnConfig
		dbScriptLink []selftype.IdentKey
		scriptConf []cfg.ScriptConfig
	}

	runPlanMap map[executeorV1IntervalSec] map[selftype.IdentKey]selftype.Plan
}

func (e *ExecutorV1) Update(timeoutMs int64) error {


	return nil
}

func (e *ExecutorV1) Fetch(timeoutMs int64) error {
	ctx, cancelFn := context.WithTimeout(e.sharedState.GetContext(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancelFn()

	if db, dbErr := e.fetcher.DbList(ctx, global.ManageP); dbErr != nil {
		return errlist.ErrG.NewError(dbErr, "update db list failed")
	} else {
		e.cache.dbConf = db
	}

	if dbScriptLink, dbScriptErr := e.fetcher.DbAndScriptLink(ctx, global.ManageP); dbScriptErr != nil {
		return errlist.ErrG.NewError(dbScriptErr, "update db script list failed")
	} else {
		e.cache.dbScriptLink = make([]selftype.IdentKey, 0, len(dbScriptLink))
		for idx := range dbScriptLink {
			e.cache.dbScriptLink = append(e.cache.dbScriptLink, selftype.NewIdentKey(dbScriptLink[idx].DbUUID, dbScriptLink[idx].ScriptName))
		}
	}

	if script, scriptErr := e.fetcher.Script(ctx, global.ManageP); scriptErr != nil {
		return errlist.ErrG.NewError(scriptErr, "update script list failed")
	} else {
		e.cache.scriptConf = script
	}

	return nil
}
func (e *ExecutorV1) disPatchRmStopTh() error {
	for idx := range e.stopRunQ {
		stop, exists := e.threadStopFnMap[e.stopRunQ[idx]]
		if !exists {continue}
		stop()
		delete(e.threadStopFnMap, e.stopRunQ[idx])
	}
	return nil
}

func (e *ExecutorV1) disPatchStartNewTh() error {
	for idx := range e.newRunQ {
		if isRun, exists := e.sharedState.GetThreadState(e.newRunQ[idx].ident); exists && isRun {
			continue
		}

		newPlan := plan.NewScriptPlan(e.newRunQ[idx].ident, e.newRunQ[idx].conf, e.sharedState)

		if _, exists := e.runPlanMap[e.newRunQ[idx].conf.Interval.Sec][]

	}

	return nil
}

func (e *ExecutorV1) DisPatch() error {
	if err := e.disPatchRmStopTh(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}


}

func (e *ExecutorV1) Close() error {
	e.executorCancelFn()

	stopMap := make(map[generic.Pair[string, string]]context.CancelFunc, 10)
	e.threadStopFnMap.RawRead(stopMap)

	for _, thStopFn := range stopMap {
		thStopFn()
	}

	return nil
}




