package executor

import (
	"bytes"
	"context"
	"crypto/md5"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/conn"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"slices"
	"time"

	"go.uber.org/zap"
)

type Executor struct {
	execSelfCtxCancelFn context.CancelFunc
	manageConf          *cfg.ProcessConfig

	private execPrivateState
	shared  *execSharedState
}

func (e *Executor) Fetch(timeoutMs int) error {
	e.private.logicMutex.Lock()
	defer e.private.logicMutex.Unlock()

	e.private.DbListRaw = nil
	e.private.DbOptionRaw = nil
	e.private.ScriptLinkRaw = nil
	e.private.ScriptRaw = nil

	timeCtx, cancelFn := context.WithTimeout(e.shared.ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancelFn()

	listData, _, listErr := e.private.manage.RunQuery(timeCtx, &conn.Args{
		Query:         e.manageConf.Cmd.Db.Query.DbList,
		Args:          nil,
		IsTransaction: false,
	})

	if listErr != nil {
		return errlist.ErrG.NewError(listErr, "get db list data failed : %s", e.manageConf.Cmd.Db.Query.DbList)
	}

	optionData, _, optErr := e.private.manage.RunQuery(timeCtx, &conn.Args{
		Query:         e.manageConf.Cmd.Db.Query.DbOption,
		Args:          nil,
		IsTransaction: false,
	})

	if optErr != nil {
		return errlist.ErrG.NewError(optErr, "get db opt data failed : %s", e.manageConf.Cmd.Db.Query.DbOption)
	}

	scriptData, _, scriptErr := e.private.manage.RunQuery(timeCtx, &conn.Args{
		Query:         e.manageConf.Cmd.Db.Query.Script,
		Args:          nil,
		IsTransaction: false,
	})

	if scriptErr != nil {
		return errlist.ErrG.NewError(scriptErr, "get script data failed : %s", e.manageConf.Cmd.Db.Query.DbOption)
	}

	scriptLinkData, _, scriptLinkErr := e.private.manage.RunQuery(timeCtx, &conn.Args{
		Query:         e.manageConf.Cmd.Db.Query.DbScriptLink,
		Args:          nil,
		IsTransaction: false,
	})

	if scriptLinkErr != nil {
		return errlist.ErrG.NewError(scriptLinkErr, "get db sciprt link data failed : %s", e.manageConf.Cmd.Db.Query.DbOption)
	}

	e.private.ScriptLinkRaw = scriptLinkData
	e.private.DbListRaw = listData
	e.private.DbOptionRaw = optionData
	e.private.ScriptRaw = scriptData

	return nil
}

func (e *Executor) delNotUseConnPool() error {
	identifiers := make([]string, len(e.private.DbListRaw))
	for idx := range e.private.DbListRaw {
		identifier, convIdent := e.private.DbListRaw[idx][0].(string)
		if !convIdent {
			return errlist.ErrG.NewError(nil, "convert failed ident %s", e.manageConf.Cmd.Db.Query.DbList)
		}
		identifiers[idx] = identifier
	}

	deleted := make([]string, 0, 3)
	e.shared.pMap.Range(func(key string, value conn.ConnPoolInterface) bool {
		if !slices.Contains(identifiers, key) {
			deleted = append(deleted, key)
		}

		return true
	})

	for _, delIdent := range deleted {
		p, exists := e.shared.pMap.Load(delIdent)
		if !exists {
			continue
		}
		_ = p.Close()
		e.shared.pMap.Delete(delIdent)
	}

	return nil
}

func (e *Executor) loadConnOptionMap(connIdent string) (map[string]any, error) {
	var convOk bool = true
	var identifier string
	var key string
	var value string

	m := make(map[string]any)
	for idx := range e.private.DbOptionRaw {
		identifier, convOk = e.private.DbOptionRaw[idx][0].(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(nil, "convert failed ident data %s", e.manageConf.Cmd.Db.Query.DbOption)
		}
		if connIdent != identifier {
			continue
		}

		key, convOk = e.private.DbOptionRaw[idx][1].(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(nil, "convert failed data %s", e.manageConf.Cmd.Db.Query.DbOption)
		}

		m[key] = value
	}
	if len(m) <= 0 {
		return nil, nil
	}
	return m, nil
}

func (e *Executor) loadConnPool() error {
	var convOk bool = true
	var identifier string
	var dbtype string
	var name string
	var server string
	var id string
	var password string
	var maxConn int

	for idx := range e.private.DbListRaw {
		bit := false
		identifier, bit = e.private.DbListRaw[idx][0].(string)
		convOk = convOk && bit
		dbtype, bit = e.private.DbListRaw[idx][1].(string)
		convOk = convOk && bit
		name, bit = e.private.DbListRaw[idx][2].(string)
		convOk = convOk && bit
		server, bit = e.private.DbListRaw[idx][3].(string)
		convOk = convOk && bit
		id, bit = e.private.DbListRaw[idx][4].(string)
		convOk = convOk && bit
		password, bit = e.private.DbListRaw[idx][5].(string)
		convOk = convOk && bit
		maxConn, bit = e.private.DbListRaw[idx][6].(int)
		convOk = convOk && bit

		if !convOk {
			return errlist.ErrG.NewError(nil, "convert failed data %s", e.manageConf.Cmd.Db.Query.DbList)
		}

		optionMap, optionErr := e.loadConnOptionMap(identifier)
		if optionErr != nil {
			return errlist.ErrG.NewError(optionErr, "load conn option failed : %s", identifier)
		}

		newPool, newPoolErr := conn.GetConnPool(&conn.ConnConfig{
			DBType:    dbtype,
			Name:      name,
			Server:    server,
			Id:        id,
			Password:  password,
			MaxConn:   maxConn,
			OptionMap: optionMap,
		})

		if newPoolErr != nil {
			return errlist.ErrG.NewError(newPoolErr, "identifier : %s", identifier)
		}

		e.shared.pMap.Store(identifier, newPool)
	}
	return nil
}

func (e *Executor) loadScript() error {
	var convOk bool = true
	var name string
	var data string

	for idx := range e.private.ScriptRaw {
		bit := false
		name, bit = e.private.DbListRaw[idx][0].(string)
		convOk = convOk && bit
		data, bit = e.private.DbListRaw[idx][1].(string)
		convOk = convOk && bit

		if !convOk {
			return errlist.ErrG.NewError(nil, "convert failed data %s", e.manageConf.Cmd.Db.Query.Script)
		}

		scriptConf, confErr := cfg.GetScriptConfigTomlFromData(data)
		if confErr != nil {
			return errlist.ErrG.NewError(confErr, "load script config failed")
		}

		hashSum := md5.Sum([]byte(data))

		if script, ok := e.shared.scriptMap.Load(name); ok {
			if bytes.Equal(hashSum[:], script.First[:]) {
				continue
			}
			e.shared.scriptMap.Delete(name)
		}

		e.shared.scriptMap.Store(name, generic.Pair[[16]byte, *cfg.ScriptConfig]{First: hashSum, Second: scriptConf})
	}

	return nil
}

func (e *Executor) LoadScriptLink() error {
	var convOk bool = true
	var dbName string
	var scriptName string

	e.private.newRunQ = make([]generic.Pair[string, string], len(e.private.ScriptLinkRaw))
	for idx := range e.private.ScriptLinkRaw {
		bit := false
		dbName, bit = e.private.DbListRaw[idx][0].(string)
		convOk = convOk && bit
		scriptName, bit = e.private.DbListRaw[idx][1].(string)
		convOk = convOk && bit

		if !convOk {
			return errlist.ErrG.NewError(nil, "convert failed data %s", e.manageConf.Cmd.Db.Query.DbScriptLink)
		}

		e.private.newRunQ[idx] = generic.Pair[string,string]{First: dbName, Second: scriptName}
	}
	return nil
}

func (e *Executor) sharedUpdate() error {
	if err := e.loadScript(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	if err := e.delNotUseConnPool(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	if err := e.loadConnPool(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	return nil
}

func (e *Executor) privateUpdate() error {
	if err := e.LoadScriptLink(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	return nil
}

func (e *Executor) Update(timeoutMs int) error {
	e.private.logicMutex.Lock()
	defer e.private.logicMutex.Unlock()

	start := time.Now()

	if err := e.sharedUpdate(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	if sub := start.Sub(time.Now()).Milliseconds(); sub > int64(timeoutMs) {
		return errlist.ErrG.NewError(nil, "update timeout : %d ms", sub)
	}

	if err := e.privateUpdate(); err != nil {
		return errlist.ErrG.NewError(err, "")
	}

	return nil
}

func (e *Executor) DisPatch() error {
	e.private.logicMutex.Lock()
	defer e.private.logicMutex.Unlock()

	for idx := range e.private.newRunQ {
		run, notExist := e.shared.isRunningThreadMap.Load(e.private.newRunQ[idx])
		if !notExist && run {continue}
		thCtx, thCancelFn := context.WithCancel(e.shared.ctx)

		e.shared.isRunningThreadMap.Store(e.private.newRunQ[idx], true)
		e.private.threadStopFnMap.Store(e.private.newRunQ[idx], thCancelFn)

		go func(k generic.Pair[string, string], state *execSharedState, ctx context.Context, cancelFn context.CancelFunc) {
			th := newScriptThread()
			if err := th.Run(); err != nil {
				zap.L().Error("executor.th", zap.Error(err))
			}

			cancelFn()
			state.isRunningThreadMap.Store(k, false)
		}(e.private.newRunQ[idx], e.shared, thCtx, thCancelFn)

	}

	return nil
}

/*
func NewSelfExecutor(conf *cfg.Config, manageConf *cfg.ProcessConfig, scripts []cfg.ScriptSelfConfig) *SelfExecutor {
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
*/
