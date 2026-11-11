package pool

import (
	"context"
	"io"
	"oncecall/errset"
	"oncecall/errset/prefix"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"sync"
	"sync/atomic"
	"time"
)

type timeoutPoolItem[T io.Closer] struct {
	Data        T
	lastUseTime time.Duration
}

type TimeoutPoolExtension[T io.Closer, R any] struct {
	Name        string
	Gen         func(context.Context, log.LoggerDebugExtension[any]) (T, error)
	Max         int
	Metric      metric.MetricExtension
	Logger      log.LoggerLogExtension[any]
	Context     context.Context
	IdleTimeout time.Duration

	once         sync.Once
	initLock     sync.Mutex
	isInit       atomic.Bool
	isClose      atomic.Bool
	chanPool     chan timeoutPoolItem[T]
	useCnt       atomic.Int32
	accErrCnt    atomic.Int64
	realMaxCnt   int
	chanPushLock sync.Mutex
}

func (u *TimeoutPoolExtension[T, R]) gen() (T, error) {
	var data T

	if u.isClose.Load() {
		return data, nil
	}

	var err error
	data, err = u.Gen(u.Context, u.Logger)

	if err != nil {
		err = errset.ErrG.NewError(err, "gen failed")
	}

	return data, err
}

func (u *TimeoutPoolExtension[T, R]) Use(useFn func(data T) (R, error)) (ret R, err error) {
	if u.isClose.Load() {
		return ret, errset.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}

	select {
	case <-u.Context.Done():
		_ = u.Close()
		return ret, errset.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already camcel context")
	default:
	}

	if !u.isInit.Load() {
		u.initLock.Lock()
		u.once.Do(func() {
			u.realMaxCnt = 10
			if u.Max > 0 {
				u.realMaxCnt = u.Max
			}

			u.chanPool = make(chan timeoutPoolItem[T], u.realMaxCnt)
		})
		u.Metric.Assignment(u.Name, poolExtensionImplMetricFullKey, float64(u.realMaxCnt))
		u.initLock.Unlock()
		u.isInit.Store(true)
	}

	var item timeoutPoolItem[T]

getLoop:
	for {
		select {
		case item = <-u.chanPool:
			if u.IdleTimeout > 0 && item.lastUseTime != 0 && (time.Duration(time.Now().Second())*time.Second)-item.lastUseTime > u.IdleTimeout {
				_ = item.Data.Close()
				u.Metric.Assignment(u.Name, poolExtensionImplMetricIdleKey, float64(len(u.chanPool)))
				continue
			}
			break getLoop
		default:
			var genItemVal T
			genItemVal, err = u.gen()
			if err != nil {
				u.accErrCnt.Add(1)
				u.Metric.Assignment(u.Name, poolExtensionImplMetricAccErrKey, float64(u.accErrCnt.Load()))
				return ret, errset.ErrG.NewError(err, "use, gen failed")
			}
			item.Data = genItemVal
			item.lastUseTime = 0
			break getLoop
		}
	}

	u.useCnt.Add(1)
	u.Metric.Assignments(u.Name, map[string]float64{
		poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
		poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
	})

	ret, useErr := useFn(item.Data)

	u.useCnt.Add(-1)
	u.Metric.Assignments(u.Name, map[string]float64{
		poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
		poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
	})

	if useErr != nil {
		_ = item.Data.Close()
		u.accErrCnt.Add(1)
		u.Metric.Assignments(u.Name, map[string]float64{
			poolExtensionImplMetricIdleKey:   float64(len(u.chanPool)),
			poolExtensionImplMetricUseKey:    float64(u.useCnt.Load()),
			poolExtensionImplMetricAccErrKey: float64(u.accErrCnt.Load()),
		})
		return ret, errset.ErrG.NewError(useErr, "pool.use execute failed")
	}

	if !u.isClose.Load() {

		u.chanPushLock.Lock()
		if len(u.chanPool) < u.realMaxCnt {
			item.lastUseTime = time.Duration(time.Now().Second()) * time.Second
			u.chanPool <- item
		} else {
			_ = item.Data.Close()
		}
		u.Metric.Assignments(u.Name, map[string]float64{
			poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
			poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
		})
		u.chanPushLock.Unlock()
	}
	return
}

func (u *TimeoutPoolExtension[T, R]) Close() error {
	if u.isClose.Swap(true) {
		return errset.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}
loop:
	for {
		select {
		case item := <-u.chanPool:
			_ = item.Data.Close()

		default:
			break loop
		}
	}
	if u.chanPool != nil {
		close(u.chanPool)
	}
	u.Metric.DeleteFromName(u.Name)
	return nil
}

var _ PoolExtension[io.Closer, any] = (*TimeoutPoolExtension[io.Closer, any])(nil)
