package state

import (
	"context"
	"oncecall/pool/types"
	"oncecall/vm"
)

type SharedStateCommonCtl interface {
	GetVm(lang string) (vm.Vm, error)
	GetContext() context.Context
	GetConnPool(ident string) (connPool types.ConnPool, err error)
}

type SharedStateExecutorCtl interface {
	SharedStateCommonCtl
	PushConnPool(ident string, pool types.ConnPoolCloser)
	PopConnPool(ident string)
	SetStartUpThreadState(key IdentKey)
	GetThreadState(key IdentKey) (isStartUp bool, isRun bool, exists bool)
}

type SharedStatePlanCtl interface {
	SharedStateCommonCtl
	SetStopThreadState(key IdentKey)
	SetRunningThreadState(key IdentKey)
}

type SharedState interface {
	SharedStateExecutorCtl
	SharedStatePlanCtl
}
