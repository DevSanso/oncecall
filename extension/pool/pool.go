package pool

import (
	"io"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/metric"
	"oncecall/utils/generic"
	"sync"
	"sync/atomic"
)

const (
	utilPoolExtensionImplMetricFullKey = "full"
	utilPoolExtensionImplMetricUseKey = "use"
	utilPoolExtensionImplMetricIdleKey = "idle"
	utilPoolExtensionImplMetricAccErrKey = "accErr"
)

type UtilPoolExtension[T io.Closer, R any] interface {
	Use(useFn func(data T) (R, error)) (ret R, err error)
	Close() error
}

type UtilPoolExtensionImpl[T io.Closer, R any] struct {
	isClose atomic.Bool

	Gen func() (T, error)
	Max int
	Metric metric.MetricExtension

	chanPool chan T
}

func (u *UtilPoolExtensionImpl[T, R]) gen() (T, error) {
	var data T

	if u.isClose.Load() {
		return data, nil
	}

	var err error
	data, err = u.Gen()

	if err != nil {
		u.Metric.Add(utilPoolExtensionImplMetricAccErrKey, 1)
		err = errlist.ErrG.NewError(err, "gen failed")
	}

	return data, err
}

func (u *UtilPoolExtensionImpl[T, R]) Use(useFn func(data T) (R, error)) (ret R, err error) {
	if u.isClose.Load() {
		return ret, errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}

	var item T
	select {
	case item = <-u.chanPool:
	default:
		item, err = u.gen()
		u.Metric.Adds(map[string]float64{
			utilPoolExtensionImplMetricFullKey : 1,
			utilPoolExtensionImplMetricIdleKey : 1,
		})
		if err != nil {

			return ret, errlist.ErrG.NewError(err, "use, gen failed")
		}
	}

	u.Metric.Adds(map[string]float64{
		utilPoolExtensionImplMetricUseKey : 1,
		utilPoolExtensionImplMetricIdleKey : -1,
	})
	ret, useErr := useFn(item)
	if useErr != nil {
		u.Metric.Adds(map[string]float64{
			utilPoolExtensionImplMetricUseKey : -1,
			utilPoolExtensionImplMetricFullKey : -1,
			utilPoolExtensionImplMetricAccErrKey : 1,
		})

		return ret, errlist.ErrG.NewError(useErr, "pool.use execute failed")
	}

	if !u.isClose.Load() {
		u.chanPool <- item

		u.Metric.Adds(map[string]float64{
			utilPoolExtensionImplMetricUseKey : -1,
			utilPoolExtensionImplMetricIdleKey : 1,
		})
	}
	return
}

func (u *UtilPoolExtensionImpl[T, R]) Close() error {
	if u.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}
	u.isClose.Store(true)

	loop:
	for {
		select {
		case item := <-u.chanPool:
			_ = item.Close()

		default:
			break loop
		}
	}
	close(u.chanPool)

	return nil
}

var _ UtilPoolExtension[io.Closer, any] = (*UtilPoolExtensionImpl[io.Closer, any])(nil)

type SyncUsePool[T io.Closer, R any] struct {
	isClose atomic.Bool
	statRw  sync.RWMutex

	stat struct {
		full int
		use  int
		idle int

		accumulatorErr int
	}

	//First : data, Second : error
	p *generic.SyncPool[generic.Pair[struct {
		data    T
		isEmpty bool
	}, error]]
}

func NewGenericSyncUsePool[T io.Closer, R any](gen func() (T, error)) *SyncUsePool[T, R] {
	p := &SyncUsePool[T, R]{
		isClose: atomic.Bool{},
	}

	wrapGen := func() Pair[struct {
		data    T
		isEmpty bool
	}, error] {
		var data T

		if p.isClose.Load() {
			return Pair[struct {
				data    T
				isEmpty bool
			}, error]{
				struct {
					data    T
					isEmpty bool
				}{data: data, isEmpty: true}, nil,
			}
		}

		var err error
		data, err = gen()

		if err == nil {
			p.addStat(1, 0, 1, 0)
		} else {
			p.addStat(0, 0, 0, 1)
		}

		return Pair[struct {
			data    T
			isEmpty bool
		}, error]{
			struct {
				data    T
				isEmpty bool
			}{data: data, isEmpty: false}, err,
		}
	}

	p.p = NewGenericSyncPool(wrapGen)
	return p
}
func (p *SyncUsePool[T, R]) addStat(full, use, idle, err int) {
	p.statRw.Lock()
	defer p.statRw.Unlock()

	p.stat.full += full
	p.stat.use += use
	p.stat.idle += idle
	p.stat.accumulatorErr += err
}
func (p *SyncUsePool[T, R]) Use(useFn func(data T) (R, error)) (ret R, err error) {
	if p.isClose.Load() {
		return ret, errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}

	pData := p.p.Get()
	if pData.Second != nil {
		return ret, errlist.ErrG.NewError(pData.Second, "get data failed")
	}

	p.addStat(0, 1, -1, 0)
	ret, err = useFn(pData.First.data)
	p.addStat(0, -1, 1, 0)

	if err != nil {
		_ = pData.First.data.Close()
		p.addStat(-1, 0, -1, 1)
		return ret, errlist.ErrG.NewError(err, "useFn failed")
	}

	if !p.isClose.Load() {
		pData.First.isEmpty = false
		pData.Second = nil
		p.p.Put(pData)
	} else {
		p.addStat(-1, 0, -1, 0)
		_ = pData.First.data.Close()
	}

	return
}

func (p *SyncUsePool[T, R]) Stat() (full, use, idle, accErr int) {
	p.statRw.RLock()
	defer p.statRw.RUnlock()

	return p.stat.full, p.stat.use, p.stat.idle, p.stat.accumulatorErr
}

func (p *SyncUsePool[T, R]) Close() error {
	if p.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}
	p.isClose.Store(true)
	full, use, idle, err := p.Stat()
	p.addStat(-full, -use, -idle, -err)

	return nil
}
