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
	//First : data, Second : error
	p *SyncPool[Pair[struct{data T; isEmpty bool},error]]
}

func NewGenericSyncUsePool[T io.Closer, R any](gen func() (T,error)) *SyncUsePool[T,R] {
	p := &SyncUsePool[T,R]{
		isClose: atomic.Bool{},
	}


	wrapGen := func() Pair[struct{data T; isEmpty bool},error] {
		var data T

		if p.isClose.Load() {
			return Pair[struct{data T; isEmpty bool},error]{
				struct {
					data    T
					isEmpty bool
				}{data: data, isEmpty: true}, nil,
			}
		}

		var err error
		data, err = gen()
		return Pair[struct{data T; isEmpty bool},error]{
			struct {
				data    T
				isEmpty bool
			}{data: data, isEmpty: false}, err,
		}
	}

	p.p = NewGenericSyncPool(wrapGen)
	return p
}

func (p *SyncUsePool[T, R]) Use(useFn func(data T) (R, error)) (ret R, err error) {
	if p.isClose.Load() {
		return ret, errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}

	pData := p.p.Get()

	if pData.Second != nil {
		return ret, errlist.ErrG.NewError(pData.Second, "get data failed")
	}

	ret, err = useFn(pData.First.data)
	if err != nil {
		_ = pData.First.data.Close()
		return ret, errlist.ErrG.NewError(err, "useFn failed")
	}

	if !p.isClose.Load() {
		pData.First.isEmpty = false
		pData.Second = nil
		p.p.Put(pData)
	} else {
		_ = pData.First.data.Close()
	}

	return
}

func (p *SyncUsePool[T,R]) Close() error {
	if p.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "SyncExpirePool already closed")
	}
	p.isClose.Store(true)


	return nil
}


type SyncMap[K comparable, V any] struct {
	sm sync.Map
}

func NewGenericSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{}
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

func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.sm.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

func (m *SyncMap[K, V]) Write(src *SyncMap[K, V]) {
	fn := func(k any,v any) bool {
		m.sm.Store(k,v)
		return true
	}
	
	src.sm.Range(fn)
}

func (m *SyncMap[K, V]) Read(dst *SyncMap[K, V]) {
	fn := func(k any,v any) bool {
		dst.sm.Store(k,v)
		return true
	}
	
	m.sm.Range(fn)
}

func (m *SyncMap[K, V]) RawRead(dst map[K]V) {
	fn := func(k any,v any) bool {
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