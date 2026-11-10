package vm

import "oncecall/utils/generic"

type Vm interface {
	Do(cache *generic.SyncMap[string, any], script string, dataCache *generic.SyncMap[int, [][]any]) ([][]any, error)
}
