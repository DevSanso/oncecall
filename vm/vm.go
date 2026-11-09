package vm

import "oncecall/utils/generic"

import _ "oncecall/vm/julia/ffi"

type Vm interface {
	Do(cache *generic.SyncMap[string, any], script string, dataCache *generic.SyncMap[int, [][]any]) ([][]any, error)
}
