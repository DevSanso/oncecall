package executor

import (
	"context"
	"oncecall/cmd/oncecall_sync/cfg"
	selftype "oncecall/cmd/oncecall_sync/executor/types"
	"oncecall/cmd/oncecall_sync/global/manage"
	plan2 "oncecall/cmd/oncecall_sync/plan"
	"oncecall/cmd/oncecall_sync/state"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"sync"
	"time"
)

type executeorV1IntervalSec int

type executorV1DbAndPlanManage struct {
	planStateMap          map[executeorV1IntervalSec][]plan2.PlanState
	planMap               map[string]plan2.Plan
	ConnUseIntervalSecMap map[string][]int

	mutex sync.RWMutex
}

func (e *executorV1DbAndPlanManage) PushPlan() {}

type ExecutorV1 struct {
	fetcher          selftype.DbFetcher
	executorCancelFn context.CancelFunc
	sharedState      state.SharedState

	cache struct {
		fetch struct {
			dbConf       []cfg.ConnConfig
			dbScriptLink []state.IdentKey
			scriptConf   []cfg.ScriptConfig
		}
		update struct {
			newRunQ []struct {
				ident state.IdentKey
				conf  cfg.ScriptConfig
			}
			stopRunQ []state.IdentKey
		}

		dispatch struct {
			lazyDropPlan map[state.IdentKey]struct{}
		}
	}

	//key : dbuuid
	planDesc map[string]map[struct {
		ScriptName string
		Interval   executeorV1IntervalSec
	}]plan2.PlanState
}

func (e *ExecutorV1) Update(timeoutMs int64) error {

	return nil
}

func (e *ExecutorV1) Fetch(timeoutMs int64) error {
	ctx, cancelFn := context.WithTimeout(e.sharedState.GetContext(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancelFn()

	if db, dbErr := e.fetcher.DbList(ctx, manage.ManageP); dbErr != nil {
		return errlist.ErrG.NewError(dbErr, "update db list failed")
	} else {
		e.cache.fetch.dbConf = db
	}

	if dbScriptLink, dbScriptErr := e.fetcher.DbAndScriptLink(ctx, manage.ManageP); dbScriptErr != nil {
		return errlist.ErrG.NewError(dbScriptErr, "update db script list failed")
	} else {
		e.cache.fetch.dbScriptLink = make([]state.IdentKey, 0, len(dbScriptLink))
		for idx := range dbScriptLink {
			e.cache.fetch.dbScriptLink = append(e.cache.fetch.dbScriptLink, state.NewIdentKey(dbScriptLink[idx].DbUUID, dbScriptLink[idx].ScriptName))
		}
	}

	if script, scriptErr := e.fetcher.Script(ctx, manage.ManageP); scriptErr != nil {
		return errlist.ErrG.NewError(scriptErr, "update script list failed")
	} else {
		e.cache.fetch.scriptConf = script
	}

	return nil
}
func (e *ExecutorV1) disPatchRmStopTh() error {
	for _, data := range e.cache.update.stopRunQ {
		startup, running, _ := e.sharedState.GetThreadState(data)
		if startup || running {
			e.cache.dispatch.lazyDropPlan[data] = struct{}{}
			continue
		}
		desc, exists := e.planDesc[data.DbID()]
		if !exists {
			continue
		}

		for k, v := range desc {
			_ = v.Close()
			delete(desc, k)
		}
	}

	return nil
}

func (e *ExecutorV1) DisPatch() error {
	if err := e.disPatchRmStopTh(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	return nil
}

func (e *ExecutorV1) Close() error {
	e.executorCancelFn()

	stopMap := make(map[generic.Pair[string, string]]context.CancelFunc, 10)

	for _, thStopFn := range stopMap {
		thStopFn()
	}

	return nil
}
