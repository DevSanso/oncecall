package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	selftype "oncecall/cmd/oncecall_query/executor/types"
	"oncecall/cmd/oncecall_query/global"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"time"
)

type ExecutorV1 struct {
	fetcher selftype.DbFetcher

	newRunQ         []generic.Pair[string, string]
	stopRunQ        []generic.Pair[string, string]
	threadStopFnMap *generic.SyncMap[generic.Pair[string, string], context.CancelFunc]
	executorCancelFn context.CancelFunc

	sharedState *execSharedState

	cache struct {
		dbConf []cfg.ConnConfig
		dbScriptLink []generic.Pair[string, string]
		scriptConf []cfg.ScriptConfig
	}
}

func (e *ExecutorV1) Update(timeoutMs int64) error {


	return nil
}

func (e *ExecutorV1) Fetch(timeoutMs int64) error {
	ctx, cancelFn := context.WithTimeout(e.sharedState.executorContext, time.Duration(timeoutMs)*time.Millisecond)
	defer cancelFn()

	if db, dbErr := e.fetcher.DbList(ctx, global.ManageP); dbErr != nil {
		return errlist.ErrG.NewError(dbErr, "update db list failed")
	} else {
		e.cache.dbConf = db
	}

	if dbScriptLink, dbScriptErr := e.fetcher.DbAndScriptLink(ctx, global.ManageP); dbScriptErr != nil {
		return errlist.ErrG.NewError(dbScriptErr, "update db script list failed")
	} else {
		e.cache.dbScriptLink = make([]generic.Pair[string, string], 0, len(dbScriptLink))
		for idx := range dbScriptLink {
			e.cache.dbScriptLink = append(e.cache.dbScriptLink, generic.Pair[string, string]{
				First: dbScriptLink[idx].DbUUID, Second: dbScriptLink[idx].ScriptName})
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
		stop, exists := e.threadStopFnMap.Load(e.stopRunQ[idx])
		if !exists {continue}
		stop()
	}

	return nil
}

func (e *ExecutorV1) disPatchStartNewTh() error {
	for idx := range e.newRunQ {
		if isRun, exists := e.sharedState.isRunningThreadMap.Load(e.newRunQ[idx]); exists && isRun {
			continue
		}


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




