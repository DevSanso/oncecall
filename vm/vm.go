package vm

import "oncecall/utils/generic"

type Vm interface {
	Do(cache *generic.GenericSyncMap[string, any], script string, data [][]any) ([][]any, error)
}
