package vm

import "oncecall/utils/generic"

type Vm[T any] interface {
	Do(cache *generic.GenericSyncMap[string, any], script string, data [][]any) ([][]any, error)
	Extend(func(raw *T) error) error
}
