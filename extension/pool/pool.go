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
	"time"
)

const (
	poolExtensionImplMetricFullKey   = "full"
	poolExtensionImplMetricUseKey    = "use"
	poolExtensionImplMetricIdleKey   = "idle"
	poolExtensionImplMetricAccErrKey = "accErr"
)

type PoolExtension[T io.Closer, R any] interface {
	Use(useFn func(data T) (R, error)) (ret R, err error)
	Close() error
}

type simplePoolItem[T io.Closer] struct {
	Data        T
	lastUseTime time.Duration
}

type SimplePoolExtension[T io.Closer, R any] struct {
	Name        string
	Gen         func(context.Context, log.LoggerDebugExtension[any]) (T, error)
	Max         int
	Metric      metric.MetricExtension
	Logger      log.LoggerLogExtension[any]
	Context     context.Context
	IdleTimeout time.Duration

	once      sync.Once
	initLock  sync.Mutex
	isInit    atomic.Bool
	isClose   atomic.Bool
	chanPool  chan simplePoolItem[T]
	useCnt    atomic.Int32
	accErrCnt atomic.Int64
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

	if !u.isInit.Load() {
		u.initLock.Lock()
		u.once.Do(func() {
			maxCnt := 10
			if u.Max > 0 {
				maxCnt = u.Max
			}
			u.chanPool = make(chan simplePoolItem[T], maxCnt)
		})
		u.initLock.Unlock()
		u.isInit.Store(true)
	}

	var item simplePoolItem[T]
	select {
	case item = <-u.chanPool:

	default:
		var genItemVal T
		genItemVal, err = u.gen()
		if err != nil {
			u.accErrCnt.Add(1)
			u.Metric.Assignment(u.Name, poolExtensionImplMetricAccErrKey, float64(u.accErrCnt.Load()))
			return ret, errlist.ErrG.NewError(err, "use, gen failed")
		}
		item.Data = genItemVal
		item.lastUseTime = 0
	}

	u.useCnt.Add(1)
	ret, useErr := useFn(item.Data)
	if useErr != nil {
		_ = item.Data.Close()
		u.useCnt.Add(-1)
		u.accErrCnt.Add(1)
		u.Metric.Assignments(u.Name, map[string]float64{
			poolExtensionImplMetricFullKey:   float64(u.Max),
			poolExtensionImplMetricIdleKey:   float64(len(u.chanPool)),
			poolExtensionImplMetricUseKey:    float64(u.useCnt.Load()),
			poolExtensionImplMetricAccErrKey: float64(u.accErrCnt.Load()),
		})
		return ret, errlist.ErrG.NewError(useErr, "pool.use execute failed")
	}

	if u.IdleTimeout > 0 && item.lastUseTime != 0 && (time.Duration(time.Now().Second())*time.Second)-item.lastUseTime > u.IdleTimeout {
		_ = item.Data.Close()
	} else if !u.isClose.Load() {
		u.useCnt.Add(-1)
		item.lastUseTime = time.Duration(time.Now().Second()) * time.Second
		u.chanPool <- item
		u.Metric.Assignments(u.Name, map[string]float64{
			poolExtensionImplMetricFullKey: float64(u.Max),
			poolExtensionImplMetricIdleKey: float64(len(u.chanPool)),
			poolExtensionImplMetricUseKey:  float64(u.useCnt.Load()),
		})
	}
	return
}

func (u *SimplePoolExtension[T, R]) Close() error {
	if u.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}
	u.isClose.Store(true)

loop:
	for {
		select {
		case item := <-u.chanPool:
			_ = item.Data.Close()

		default:
			break loop
		}
	}
	close(u.chanPool)
	u.Metric.DeleteFromName(u.Name)
	return nil
}

var _ PoolExtension[io.Closer, any] = (*SimplePoolExtension[io.Closer, any])(nil)
