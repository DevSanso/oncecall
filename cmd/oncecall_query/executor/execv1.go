package executor

import (
	"context"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/cmd/oncecall_query/executor/plan"
	selftype "oncecall/cmd/oncecall_query/executor/types"
	"oncecall/cmd/oncecall_query/global"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"sync"
	"time"
)

type executeorV1IntervalSec int

type executorV1DbAndPlanManage struct {
	planStateMap          map[executeorV1IntervalSec][]plan.PlanState
	planMap               map[string]plan.Plan
	ConnUseIntervalSecMap map[string][]int

	mutex sync.RWMutex
}

func (e *executorV1DbAndPlanManage) PushPlan() {}

type ExecutorV1 struct {
	fetcher          selftype.DbFetcher
	executorCancelFn context.CancelFunc
	sharedState      selftype.SharedState

	cache struct {
		fetch struct {
			dbConf       []cfg.ConnConfig
			dbScriptLink []selftype.IdentKey
			scriptConf   []cfg.ScriptConfig
		}
		update struct {
			newRunQ []struct {
				ident selftype.IdentKey
				conf  cfg.ScriptConfig
			}
			stopRunQ []selftype.IdentKey
		}

		dispatch struct {
			lazyDropPlan map[selftype.IdentKey]struct{}
		}
	}

	//key : dbuuid
	planDesc map[string]map[struct {
		ScriptName string
		Interval   executeorV1IntervalSec
	}]plan.PlanState
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
		e.cache.fetch.dbConf = db
	}

	if dbScriptLink, dbScriptErr := e.fetcher.DbAndScriptLink(ctx, global.ManageP); dbScriptErr != nil {
		return errlist.ErrG.NewError(dbScriptErr, "update db script list failed")
	} else {
		e.cache.fetch.dbScriptLink = make([]selftype.IdentKey, 0, len(dbScriptLink))
		for idx := range dbScriptLink {
			e.cache.fetch.dbScriptLink = append(e.cache.fetch.dbScriptLink, selftype.NewIdentKey(dbScriptLink[idx].DbUUID, dbScriptLink[idx].ScriptName))
		}
	}

	if script, scriptErr := e.fetcher.Script(ctx, global.ManageP); scriptErr != nil {
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
