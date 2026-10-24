package entry

import (
	"context"
	"oncecall/cmd/oncecall_sync/entry/master"
	"oncecall/cmd/oncecall_sync/global/log"
	"oncecall/errlist"
	"oncecall/utils/generic"
	"time"
)

func masterCurrentProcList(m master.ProcManager, list []string) (alive []string, down []string) {
	alive = make([]string, 0, len(list))
	down = make([]string, 0, len(list))

	for _, s := range list {
		if m.Alive(s) {
			alive = append(alive, s)
		} else {
			down = append(down, s)
		}
	}

	return
}

func masterDownStart(m master.ProcManager, down []string) error {
	for _, l := range down {
		pid, err := m.Start(l)

		if err != nil {
			return errlist.ErrG.NewError(err, "start %s failed", l)
		}

		log.Log.Manage.Info("start (ident:%s, pid:%d) success", l, pid)
	}
	return nil
}

func masterDownAliveProc(m master.ProcManager, killList []string) error {
	for _, l := range killList {
		if err := m.Kill(l); err != nil {
			return errlist.ErrG.NewError(err, "kill %s failed", l)
		}
	}

	return nil
}

func MasterEntry(ctx context.Context) {
	procManager := master.NewProcManager()
	dbFetcher := master.NewDbFetcher()

entry:
	for {
		select {
		case <-ctx.Done():
			break entry
		default:
		}

		list, listErr := dbFetcher.DBListIdent(ctx)
		if listErr != nil {
			frame := errlist.ErrG.NewError(listErr, "master entry load failed")
			log.Log.Manage.Error(frame.Error())
			continue
		}

		alive, down := masterCurrentProcList(procManager, list)
		killList := generic.Difference(alive, procManager.List())

		if startErr := masterDownStart(procManager, down); startErr != nil {
			frame := errlist.ErrG.NewError(startErr, "master entry start process failed")
			log.Log.Manage.Error(frame.Error())
		}

		if killErr := masterDownAliveProc(procManager, killList); killErr != nil {
			frame := errlist.ErrG.NewError(killErr, "master entry kill process failed")
			log.Log.Manage.Error(frame.Error())
		}

		time.Sleep(10 * time.Second)
	}
}
