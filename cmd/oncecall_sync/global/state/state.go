package state

import (
	"oncecall/cmd/oncecall_sync/plan"
	"oncecall/pool/types"
)

type poolState = implState[string, types.ConnPoolCloser]
type planState = implState[string, plan.Plan]

type state struct {
	planS planState
	poolS poolState
}

func (s *state) Close() error {
	_ = s.planS.Close()
	_ = s.poolS.Close()
	return nil
}

func (s *state) Plan() StatePool[string, plan.Plan] {
	return &s.planS
}

func (s *state) Pool() StatePool[string, types.ConnPoolCloser] {
	return &s.poolS
}

var (
	State state
)
