package generic

import (
	"io"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"sync"
	"sync/atomic"
)

type SyncPool[T any] struct {
	pool sync.Pool
}

func NewGenericSyncPool[T any](newFn func() T) *SyncPool[T] {
	return &SyncPool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFn()
			},
		},
	}
}

func (p *SyncPool[T]) Get() T {
	return p.pool.Get().(T)
}

func (p *SyncPool[T]) Put(x T) {
	p.pool.Put(x)
}

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
	p *SyncPool[Pair[struct {
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

type SyncMap[K comparable, V any] struct {
	sm sync.Map
}

func NewGenericSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{}
}
func NewGenericSyncMapWValue[K comparable, V any]() SyncMap[K, V] {
	return SyncMap[K, V]{}
}

func (m *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	val, ok := m.sm.Load(key)
	if !ok {
		return value, false
	}
	return val.(V), true
}

func (m *SyncMap[K, V]) Store(key K, value V) {
	m.sm.Store(key, value)
}

func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	val, loaded := m.sm.LoadOrStore(key, value)
	return val.(V), loaded
}

func (m *SyncMap[K, V]) Delete(key K) {
	m.sm.Delete(key)
}

func (m *SyncMap[K, V]) LoadAndDelete(key K) (actual V, loaded bool) {
	val, loaded := m.sm.LoadAndDelete(key)
	return val.(V), loaded
}

func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.sm.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

func (m *SyncMap[K, V]) Write(src *SyncMap[K, V]) {
	fn := func(k any, v any) bool {
		m.sm.Store(k, v)
		return true
	}

	src.sm.Range(fn)
}

func (m *SyncMap[K, V]) Read(dst *SyncMap[K, V]) {
	fn := func(k any, v any) bool {
		dst.sm.Store(k, v)
		return true
	}

	m.sm.Range(fn)
}

func (m *SyncMap[K, V]) RawRead(dst map[K]V) {
	fn := func(k any, v any) bool {
		convK := k.(K)
		convV := v.(V)
		dst[convK] = convV
		return true
	}

	m.sm.Range(fn)
}

func (m *SyncMap[K, V]) RawWrite(src map[K]V) {
	for k, v := range src {
		m.sm.Store(k, v)
	}
}
