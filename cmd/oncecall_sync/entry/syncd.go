package entry

import (
	"context"
	"oncecall/cmd/oncecall_sync/entry/syncd"
	"oncecall/cmd/oncecall_sync/global/config"
	"oncecall/cmd/oncecall_sync/global/log"
	"oncecall/cmd/oncecall_sync/global/metric"
	"oncecall/cmd/oncecall_sync/plan"
	"oncecall/cmd/oncecall_sync/state"
	"oncecall/errlist"
	"oncecall/pool"
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

func syncdResetScript(s state.SharedState, private *syncd.PrivateState, fetcher syncd.DbFetcher) error {
	scriptList, listErr := fetcher.Script(s.GetContext())
	if listErr != nil {
		return errlist.ErrG.NewError(listErr, "get failed script list")
	}

	for k, data := range scriptList {
		if already, is := private.PlanMap.Load(k); is {
			if slices.Compare(already.Second[:], data.First[:]) == 0 {
				continue
			}

			_ = already.First.Close()
			private.PlanMap.Delete(k)
		}

		sPlan := plan.NewScriptPlan(data.Second, s)
		private.PlanMap.Store(k, generic.Pair[plan.Plan, [16]byte]{First: sPlan, Second: data.First})
	}
	return nil
}

func SyncdEntry(ctx context.Context) {
	shared := state.NewSharedState(ctx)
	private := syncd.NewPrivateState()
	fetcher := syncd.NewDbFetcher()

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
entry:
	for {
		select {
		case <-ctx.Done():
			break entry
		default:
		}

		if dbListResetErr := syncdResetDbList(shared, fetcher); dbListResetErr != nil {
			log.Log.Proc.Error("%s", dbListResetErr.Error())
			break entry
		}

		if scriptResetErr := syncdResetScript(shared, private, fetcher); scriptResetErr != nil {
			log.Log.Proc.Error("%s", scriptResetErr.Error())
			break entry
		}

		time.Sleep(10 * time.Second)
	}

}
