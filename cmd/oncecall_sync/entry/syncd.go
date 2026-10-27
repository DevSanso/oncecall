package entry

import (
	"context"
	"maps"
	"oncecall/cmd/oncecall_sync/cfg"
	"oncecall/cmd/oncecall_sync/entry/syncd"
	"oncecall/cmd/oncecall_sync/global/config"
	"oncecall/cmd/oncecall_sync/global/log"
	"oncecall/cmd/oncecall_sync/global/metric"
	"oncecall/cmd/oncecall_sync/plan"
	"oncecall/cmd/oncecall_sync/state"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool"
	"oncecall/pool/types"
	"oncecall/utils"
	"oncecall/utils/generic"
	"slices"
	"time"
)

func syncdResetDbList(s state.SharedState, fetcher syncd.DbFetcher) error {
	list, listErr := fetcher.DBList(s.GetContext())
	if listErr != nil {
		return errlist.ErrG.NewError(listErr, "get failed db list")
	}

	for ident, info := range list {
		p, pErr := s.GetConnPool(ident)
		if pErr != nil {
			return errlist.ErrG.NewError(pErr, "get failed conn pool")
		}

		cmp := p.GetConfig()
		if cmp.Server == info.Server && cmp.Name == info.Name {
			continue
		}
		newP, newErr := pool.NewConnPool(s.GetContext(), ident, info, metric.Metric, log.Log.Gen, log.Log.Gen, log.Log.Conn)
		if newErr != nil {
			return errlist.ErrG.NewError(newErr, "new conn pool failed")
		}
		s.PushConnPool(ident, newP)
	}

	return nil
}

func syncdResetScriptPlan(s state.SharedState, private *syncd.PrivateState,  ks []string, scriptList map[string]generic.Pair[[16]byte, cfg.ScriptConfig]) {
	private.PlanMap.Range(func(key string, _ syncd.PlanData) bool {
		if !slices.Contains(ks, key) {
			private.PlanMap.Delete(key)
		}
		return true
	})

	for _, k := range ks {
		data := scriptList[k]
		if already, is := private.PlanMap.Load(k); is {
			if slices.Compare(already.Hash[:], data.First[:]) == 0 {
				continue
			}

			_ = already.Plan.Close()
			private.PlanMap.Delete(k)
		}

		sPlan := plan.NewScriptPlan(data.Second, s)
		pState := plan.NewPlanState(s.GetContext(), state.NewIdentKey(config.SyncIdent, k),
			len(data.Second.Plans.Read), data.Second.Option.PlanColsBufAlloc, data.Second.Option.PlanRowBufCap, data.Second.Option.PlanRowBufIdleTime)

		private.PlanMap.Store(k, syncd.PlanData{
			Hash:  data.First,
			Plan:  sPlan,
			State: pState,
		})
	}
}



func syncdResetScript(s state.SharedState, private *syncd.PrivateState, fetcher syncd.DbFetcher) error {
	scriptList, listErr := fetcher.Script(s.GetContext())
	if listErr != nil {
		return errlist.ErrG.NewError(listErr, "get failed script list")
	}

	var ks []string = slices.Collect(maps.Keys(scriptList))
	syncdResetScriptPlan(s, private, ks, scriptList)
	return nil
}

func syncdBackground(self types.ConnPool, shared state.SharedStateManageCtl, private *syncd.PrivateState) {
	for {
		select {
		case <-shared.GetContext().Done():
			return
		default:
		}

		private.PlanMap.Range(func(key string, v syncd.PlanData) bool {
			ik := state.NewIdentKey(config.SyncIdent, key)
			isRun, exists := shared.GetThreadState(ik)
			if exists && isRun {
				return true
			}

			go func() {
				shared.SetRunningThreadState(ik)
				err := v.Plan.Run(&v.State, true)
				if err != nil {
					frame := errlist.ErrG.NewError(err, "plan failed")
					log.Log.Proc.Error("%s", frame)
				}
				shared.SetStopThreadState(ik)
			}()

			return true
		})

		utils.IntervalSleep(time.Second, time.Millisecond * 10)
	}
}

func SyncdEntry(ctx context.Context) {
	syncdCtx, syncdCancel := context.WithCancel(ctx)

	defer syncdCancel()
	shared := state.NewSharedState(syncdCtx)
	private := syncd.NewPrivateState(syncdCtx, metric.Metric, log.Log.Proc)
	fetcher := syncd.NewDbFetcher()
	proc := syncd.ProcSync{}

	selfConnInfo, selfErr := fetcher.Self(shared.GetContext())
	if selfErr != nil {
		frame := errlist.ErrG.NewError(selfErr, "self get conn info failed %s", config.SyncIdent)
		log.Log.Proc.Error("%s", frame.Error())
		return
	}

	selfConn, selfConnErr := pool.NewConnPool(ctx, config.SyncIdent, selfConnInfo, metric.Metric, log.Log.Gen, log.Log.Gen, log.Log.Conn)
	if selfConnErr != nil {
		frame := errlist.ErrG.NewError(selfConnErr, "self get conn failed %s", config.SyncIdent)
		log.Log.Proc.Error("%s", frame.Error())
		return
	}

	go syncdBackground(selfConn, shared, private)

entry:
	for {
		select {
		case <-syncdCtx.Done():
			break entry
		default:
		}

		if !proc.IsParentRunning() {
			frame := errlist.ErrG.NewError(prefix.ClosedError, "master process is down")
			log.Log.Proc.Error("%s", frame.Error())
			break entry
		}

		if dbListResetErr := syncdResetDbList(shared, fetcher); dbListResetErr != nil {
			log.Log.Proc.Error("%s", dbListResetErr.Error())
			break entry
		}

		if scriptResetErr := syncdResetScript(shared, private, fetcher); scriptResetErr != nil {
			log.Log.Proc.Error("%s", scriptResetErr.Error())
			break entry
		}

		utils.IntervalSleep(time.Minute, time.Millisecond * 100)
	}
}
