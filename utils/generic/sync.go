package generic

import "sync"

type GenericSyncPool[T any] struct {
	pool sync.Pool
}

func NewGenericSyncPool[T any](newFn func() T) *GenericSyncPool[T] {
	return &GenericSyncPool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFn()
			},
		},
	}
}

func (p *GenericSyncPool[T]) Get() T {
	return p.pool.Get().(T)
}

func (p *GenericSyncPool[T]) Put(x T) {
	p.pool.Put(x)
}

type GenericSyncMap[K comparable, V any] struct {
	sm sync.Map
}

func NewGenericSyncMap[K comparable, V any]() *GenericSyncMap[K, V] {
	return &GenericSyncMap[K, V]{}
}

func (m *GenericSyncMap[K, V]) Load(key K) (value V, ok bool) {
	val, ok := m.sm.Load(key)
	if !ok {
		return value, false
	}
	return val.(V), true
}

func (m *GenericSyncMap[K, V]) Store(key K, value V) {
	m.sm.Store(key, value)
}

func (m *GenericSyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	val, loaded := m.sm.LoadOrStore(key, value)
	return val.(V), loaded
}

func (m *GenericSyncMap[K, V]) Delete(key K) {
	m.sm.Delete(key)
}

func (m *GenericSyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.sm.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

func (m *GenericSyncMap[K, V]) Write(src *GenericSyncMap[K, V]) {
	fn := func(k any,v any) bool {
		m.sm.Store(k,v)
		return true
	}
	
	src.sm.Range(fn)
}

func (m *GenericSyncMap[K, V]) Read(dst *GenericSyncMap[K, V]) {
	fn := func(k any,v any) bool {
		dst.sm.Store(k,v)
		return true
	}
	
	m.sm.Range(fn)
}

func (m *GenericSyncMap[K, V]) RawRead(dst map[K]V) {
	fn := func(k any,v any) bool {
		convK := k.(K)
		convV := v.(V)
		dst[convK] = convV
		return true
	}
	
	m.sm.Range(fn)
}

func (m *GenericSyncMap[K, V]) RawWrtie(src map[K]V) {
	for k, v := range src {
		m.sm.Store(k, v)
	}
}