package state

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"oncecall/vm"
	"oncecall/vm/lua"
)

type SharedStateCommonCtl interface {
	GetContext() context.Context
	GetConnPool(ident string) (connPool types.ConnPool, err error)
}

type SharedStateManageCtl interface {
	SharedStateCommonCtl
	PushConnPool(ident string, pool types.ConnPoolCloser)
	PopConnPool(ident string)
	SetStartUpThreadState(key ThreadKey)
	DeleteThreadState(key ThreadKey)
	GetThreadState(key ThreadKey) (isStartUp bool, isRun bool, exists bool)
	ExistsConnPool(ident string) (is bool)
}

type SharedStatePlanCtl interface {
	SharedStateCommonCtl
	GetVm(lang string) (vm.Vm, error)
	SetStopThreadState(key ThreadKey)
	SetRunningThreadState(key ThreadKey)
}

type SharedState interface {
	SharedStateManageCtl
	SharedStatePlanCtl
}
type ThreadState int

const (
	ThreadStateStartup ThreadState = 1 << iota
	ThreadStateRunning
	ThreadStateStop
)

type implSharedState struct {
	ctx context.Context

	connPool    *generic.SyncMap[string, types.ConnPoolCloser]
	threadState *generic.SyncMap[ThreadKey, ThreadState]
}

func NewSharedState(ctx context.Context) SharedState {
	return &implSharedState{
		ctx,
		generic.NewGenericSyncMap[string, types.ConnPoolCloser](),
		generic.NewGenericSyncMap[ThreadKey, ThreadState](),
	}
}

func (i *implSharedState) ExistsConnPool(ident string) (is bool) {
	_, exists := i.connPool.Load(ident)
	return exists
}

func (i *implSharedState) GetConnPool(ident string) (connPool types.ConnPool, err error) {
	conn, exists := i.connPool.Load(ident)
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "%s not exists", ident)
	}

	return conn, nil
}

func (i *implSharedState) GetContext() context.Context {
	return i.ctx
}

func (i *implSharedState) GetThreadState(key ThreadKey) (isStartUp bool, isRun bool, exists bool) {
	state, is := i.threadState.Load(key)
	if !is {
		return false, false, false
	}

	return state == ThreadStateStartup, state == ThreadStateRunning, true
}

func (i *implSharedState) GetVm(lang string) (vm.Vm, error) {
	return lua.NewLuaVM(), nil
}

func (i *implSharedState) PopConnPool(ident string) {
	i.connPool.Delete(ident)
}

func (i *implSharedState) PushConnPool(ident string, pool types.ConnPoolCloser) {
	if p, exists := i.connPool.Load(ident); exists {
		_ = p.Close()
		i.connPool.Delete(ident)
	}

	i.connPool.Store(ident, pool)
}

func (i *implSharedState) SetRunningThreadState(key ThreadKey) {
	i.threadState.Store(key, ThreadStateRunning)
}

func (i *implSharedState) SetStartUpThreadState(key ThreadKey) {
	i.threadState.Store(key, ThreadStateStartup)
}

func (i *implSharedState) SetStopThreadState(key ThreadKey) {
	i.threadState.Store(key, ThreadStateStop)
}

func (i *implSharedState) DeleteThreadState(key ThreadKey) {
	i.threadState.Delete(key)
}
