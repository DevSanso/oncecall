package pool

import (
	"context"
	"io"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"sync"
	"sync/atomic"
)

type SimplePoolExtension[T io.Closer, R any] struct {
	Name    string
	Gen     func(context.Context, log.LoggerDebugExtension[any]) (T, error)
	Max     int
	Metric  metric.MetricExtension
	Logger  log.LoggerLogExtension[any]
	Context context.Context

	once         sync.Once
	initLock     sync.Mutex
	isInit       atomic.Bool
	isClose      atomic.Bool
	chanPool     chan T
	useCnt       atomic.Int32
	accErrCnt    atomic.Int64
	realMaxCnt   int
	chanPushLock sync.Mutex
}

func (u *SimplePoolExtension[T, R]) gen() (T, error) {
	var data T

	if u.isClose.Load() {
		return data, nil
	}

	var err error
	data, err = u.Gen(u.Context, u.Logger)

	if err != nil {
		err = errlist.ErrG.NewError(err, "gen failed")
	}

	return data, err
}

func (u *SimplePoolExtension[T, R]) Use(useFn func(data T) (R, error)) (ret R, err error) {
	if u.isClose.Load() {
		return ret, errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}

	select {
	case <-u.Context.Done():
		_ = u.Close()
		return ret, errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already camcel context")
	default:
	}

	if !u.isInit.Load() {
		u.initLock.Lock()
		u.once.Do(func() {
			u.realMaxCnt = 10
			if u.Max > 0 {
				u.realMaxCnt = u.Max
			}

			u.chanPool = make(chan T, u.realMaxCnt)
		})
		u.Metric.Assignment(u.Name, poolExtensionImplMetricFullKey, float64(u.realMaxCnt))
		u.initLock.Unlock()
		u.isInit.Store(true)
	}

	var item T

	select {
	case item = <-u.chanPool:
	default:
		item, err = u.gen()
		if err != nil {
			u.accErrCnt.Add(1)
			u.Metric.Assignment(u.Name, poolExtensionImplMetricAccErrKey, float64(u.accErrCnt.Load()))
			return ret, errlist.ErrG.NewError(err, "use, gen failed")
		}
	}

	u.useCnt.Add(1)
	u.Metric.Assignments(u.Name, map[string]float64{
		poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
		poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
	})

	ret, useErr := useFn(item)

	u.useCnt.Add(-1)
	u.Metric.Assignments(u.Name, map[string]float64{
		poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
		poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
	})

	if useErr != nil {
		_ = item.Close()
		u.accErrCnt.Add(1)
		u.Metric.Assignments(u.Name, map[string]float64{
			poolExtensionImplMetricIdleKey:   float64(len(u.chanPool)),
			poolExtensionImplMetricUseKey:    float64(u.useCnt.Load()),
			poolExtensionImplMetricAccErrKey: float64(u.accErrCnt.Load()),
		})
		return ret, errlist.ErrG.NewError(useErr, "pool.use execute failed")
	}

	if !u.isClose.Load() {

		u.chanPushLock.Lock()
		if len(u.chanPool) < u.realMaxCnt {
			u.chanPool <- item
		} else {
			_ = item.Close()
		}
		u.Metric.Assignments(u.Name, map[string]float64{
			poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
			poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
		})
		u.chanPushLock.Unlock()
	}
	return
}

func (u *SimplePoolExtension[T, R]) Close() error {
	if u.isClose.Swap(true) {
		return errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}
loop:
	for {
		select {
		case item := <-u.chanPool:
			_ = item.Close()

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

var _ PoolExtension[io.Closer, any] = (*SimplePoolExtension[io.Closer, any])(nil)
