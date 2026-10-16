package types

import (
	"context"
	"oncecall/pool/types"
	"oncecall/vm"
)

type SharedStateSetter interface {
	SetRunningThreadState(key IdentKey)
	SetStopThreadState(key IdentKey)

	PushConnPool(ident string, pool types.ConnPoolCloser)
	PopConnPool(ident string)
}

type SharedStateGetter interface {
	GetThreadState(key IdentKey) (isRun bool, exists bool)
	GetConnPool(ident string) (connPool types.ConnPool, err error)
	GetVm(lang string) (vm.Vm, error)
	GetContext() context.Context
}
type SharedState interface {
	SharedStateSetter
	SharedStateGetter
}
