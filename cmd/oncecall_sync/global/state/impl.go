package state

import (
	"io"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/utils/generic"
	"sync/atomic"
)

type StatePoolGet[K comparable, V io.Closer] interface {
	Get(key K) (V, bool, error)
	Exist(key K) (bool, error)
	Exists(ks []K) (exists []K, noExists []K, err error)
}

type StatePoolSet[K comparable, V io.Closer] interface {
	Set(key K, v V) error
	Delete(key K) error
}

type StatePool[K comparable, V io.Closer] interface {
	StatePoolGet[K, V]
	StatePoolSet[K, V]
}

type StatePoolCloser[K comparable, V io.Closer] interface {
	io.Closer
	StatePool[K, V]
}

type implState[K comparable, V io.Closer] struct {
	isClose atomic.Bool
	m *generic.SyncMap[K, V]
}

func (i *implState[K, V]) Close() error {
	if i.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "")
	}
	i.isClose.Store(true)
	i.m.Range(func(key K, value V) bool {
		_ = value.Close()
		i.m.Delete(key)

		return true
	})
	return nil
}

func (i *implState[K, V]) Delete(key K) error {
	if i.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "")
	}
	i.m.Delete(key)
	return nil
}

func (i *implState[K, V]) Exist(key K) (bool, error) {
	if i.isClose.Load() {
		return false, errlist.ErrG.NewError(prefix.ClosedError, "")
	}
	_, exists := i.m.Load(key)
	return exists, nil
}

func (i *implState[K, V]) Exists(ks []K) (exists []K, noExists []K, err error) {
	if i.isClose.Load() {
		return nil,nil, errlist.ErrG.NewError(prefix.ClosedError, "")
	}

	exists = make([]K, 0, len(ks) / 2)
	noExists = make([]K, 0, len(ks)/2)

	for _, k := range ks {
		_, is := i.m.Load(k)
		if is {
			exists = append(exists, k)
		} else {
			noExists = append(noExists, k)
		}
	}

	return exists, noExists, nil
}

func (i *implState[K, V]) Get(key K) (V, bool, error) {
	var v V
	if i.isClose.Load() {
		return v, false, errlist.ErrG.NewError(prefix.ClosedError, "")
	}
	var exists bool
	v, exists = i.m.Load(key)
	return v, exists, nil
}

func (i *implState[K, V]) Set(key K, v V) error {
	if i.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "")
	}
	i.m.Store(key, v)
	return nil
}


