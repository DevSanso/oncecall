package executor

import (
	"context"
	"oncecall/errlist"

	gcfg "oncecall/cfg"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/cmd/oncecall_query/script"
	"oncecall/conn"
	"oncecall/define"
	"oncecall/manage"
	"oncecall/utils/generic"
	"time"

	"go.uber.org/zap"
)

type selfJobImpl struct {
	manageConf *cfg.ProcessConfig
}

type selfExecState = execState[*script.SelfScript]

func NewJob(manageConf *cfg.ProcessConfig) *selfJobImpl {
	return &selfJobImpl{manageConf: manageConf}
}

func (ji *selfJobImpl) initDb(ctx context.Context, p conn.ConnPoolInterface, triggerQuery string, initQuerys []string) error {
	triggerData, triggerErr := p.RunQuery(ctx, &conn.Args{
		Query:         triggerQuery,
		Args:          nil,
		IsTranscation: false,
	})
	if triggerErr != nil {
		return triggerErr
	}

	if triggerData == nil || len(triggerData) <= 0 {
		return nil
	}

	for _, q := range initQuerys {
		initErr := p.RunExecute(ctx, &conn.Args{
			Query:         q,
			Args:          nil,
			IsTranscation: false,
		})
		if initErr != nil {
			return initErr
		}
	}

	return nil
}

func (ji *selfJobImpl) Run(conf *cfg.ScriptSelfConfig, connKey []int, ctx context.Context, eState *selfExecState) error {
	isRunning := true

	if conf.Init != nil {
		var err error

		if conf.Init.Collect != nil {
			collectP, _ := eState.pMap.Load(generic.Pair[int, bool]{First: define.ConnMapCollectIdx, Second: false})
			err = ji.initDb(ctx, collectP, conf.Init.Collect.TriggerQuery, conf.Init.Collect.Query)
		}
		if conf.Init.Realtime != nil {
			realP, _ := eState.pMap.Load(generic.Pair[int, bool]{First: define.ConnMapCollectIdx, Second: false})
			err = ji.initDb(ctx, realP, conf.Init.Realtime.TriggerQuery, conf.Init.Realtime.Query)
		}

		if err != nil {
			return err
		}
	}

	flagM := generic.NewGenericSyncMap[int, bool]()

	for _, connK := range connKey {
		flagM.Store(connK, false)
	}

	for isRunning {
		select {
		case <-ctx.Done():
			isRunning = false
			continue
		default:
		}

		for _, connK := range connKey {
			if v, _ := flagM.Load(connK); v {
				continue
			} else {
				flagM.Store(connK, true)
			}

			script := eState.scriptPool.Get()
			go func(c *cfg.ScriptSelfConfig, m *generic.GenericSyncMap[int, bool], key int, state *selfExecState) {
				err := script.Run(c, key, state.jobCache, state.pMap, state.vmP)
				gap := time.Duration((1000 - time.Now().UnixMilli()%1000) + 10)
				time.Sleep(gap * time.Millisecond)
				m.Store(connK, false)
				if err != nil {
					zap.L().Error("script.fail", zap.Error(err))
				}
			}(conf, flagM, connK, eState)

			gap := time.Duration((1000 - time.Now().UnixMilli()%1000) + 50)
			time.Sleep(gap * time.Millisecond)
		}
	}

	return nil
}

type SelfExecutor struct {
	execSelfCtxCancelFn context.CancelFunc
	conf                *gcfg.Config
	manageConf          *cfg.ProcessConfig
	scripts             []cfg.ScriptSelfConfig

	state   *selfExecState
	jobPool *generic.GenericSyncPool[*selfJobImpl]
}

func NewSelfExecutor(conf *gcfg.Config, manageConf *cfg.ProcessConfig, scripts []cfg.ScriptSelfConfig) *SelfExecutor {
	obj := &SelfExecutor{
		conf:       conf,
		scripts:    scripts,
		manageConf: manageConf,
		state:      newExecState(script.NewSelfScript),
		jobPool: generic.NewGenericSyncPool(func() *selfJobImpl {
			return NewJob(manageConf)
		}),
	}
	return obj
}

func (e *SelfExecutor) setDbConn(manageP conn.ConnPoolInterface, ctx context.Context) ([]int, error) {
	var search *manage.DBSearch

	search = manage.NewDBSearch(manageP, e.manageConf.Cmd.Db.Query.DbList, e.manageConf.Cmd.Db.Query.DbOption)

	var ret = make([]int, 0, 10)

	if logmsDBInfo, selectErr := search.GetDb(ctx); selectErr != nil {
		return nil, selectErr
	} else {
		for idx := range logmsDBInfo {
			p, pErr := conn.GetConnPool(logmsDBInfo[idx].ConvertConnConfig())
			if pErr != nil {
				return nil, pErr
			}
			e.state.pMap.Store(generic.Pair[int, bool]{First: logmsDBInfo[idx].Identifier, Second: false}, p)
			ret = append(ret, logmsDBInfo[idx].Identifier)
		}
	}

	return ret, nil
}

func (e *SelfExecutor) Close() error {
	if e.execSelfCtxCancelFn == nil {
		return errlist.ErrG.NewError(nil, "not setting ctx cancel fn")
	}
	e.execSelfCtxCancelFn()

	e.state.pMap.Range(func(key generic.Pair[int, bool], value conn.ConnPoolInterface) bool {
		value.Close()
		return true
	})

	return nil
}

func (e *SelfExecutor) Run(baseCtx context.Context) error {
	var ctx context.Context

	ctx, cancel := context.WithCancel(baseCtx)
	e.state.ctx = ctx
	e.execSelfCtxCancelFn = cancel
	defer e.Close()

	manageP, manageErr := conn.GetConnPool(&e.conf.ManageDB)
	if manageErr != nil {
		return manageErr
	}
	e.state.pMap.Store(generic.Pair[int, bool]{First: define.ConnMapManageIdx, Second: false}, manageP)

	if e.conf.RealTimetDB != nil {
		realP, realErr := conn.GetConnPool(e.conf.RealTimetDB)
		if realErr != nil {
			return realErr
		}
		e.state.pMap.Store(generic.Pair[int, bool]{First: define.ConnMapRealTimeIdx, Second: false}, realP)
	}

	if e.conf.CollectDB != nil {
		collectP, collectErr := conn.GetConnPool(e.conf.CollectDB)
		if collectErr != nil {
			return collectErr
		}
		e.state.pMap.Store(generic.Pair[int, bool]{First: define.ConnMapCollectIdx, Second: false}, collectP)
	}

	var logmsNoArr []int = nil

	if list, setErr := e.setDbConn(manageP, ctx); setErr != nil {
		return setErr
	} else {
		logmsNoArr = list
	}

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("stop executor")
			break
		default:
		}

		for idx := range e.scripts {
			if e.scripts[idx].IsEanble == false {
				continue
			}

			if isRun, exists := e.state.isRunFlagMap.Load(e.scripts[idx].Name); exists && isRun {
				continue
			} else if !exists {
				e.state.isRunFlagMap.Store(e.scripts[idx].Name, true)
				zap.L().Debug("new job", zap.String("name", e.scripts[idx].Name))
			} else {
				e.state.isRunFlagMap.Store(e.scripts[idx].Name, true)
				zap.L().Debug("start set job", zap.String("name", e.scripts[idx].Name))
			}

			execJob := e.jobPool.Get()
			go func(c *cfg.ScriptSelfConfig, connKey []int, ctx context.Context, eState *selfExecState) {
				zap.L().Debug("start job", zap.String("name", e.scripts[idx].Name), zap.Int("idCnt", len(connKey)))

				execErr := execJob.Run(c, connKey, ctx, eState)
				if execErr != nil {
					zap.L().Error("execJob", zap.String("name", c.Name), zap.Error(execErr))
				}
				e.state.isRunFlagMap.Store(e.scripts[idx].Name, false)

				zap.L().Debug("stop job", zap.String("name", e.scripts[idx].Name))
			}(&e.scripts[idx], logmsNoArr, ctx, e.state)
		}

		time.Sleep(10 * time.Second)
	}
}
