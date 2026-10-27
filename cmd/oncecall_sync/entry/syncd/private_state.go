package syncd

import (
	"context"
	"oncecall/cmd/oncecall_sync/plan"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"oncecall/utils/generic"
)

type PlanData struct {
	Hash  [16]byte
	Plan  plan.Plan
	State plan.PlanState
}
type PrivateState struct {
	PlanMap *generic.SyncMap[string, PlanData]
}

func NewPrivateState(ctx context.Context, m metric.MetricExtension, logger log.LoggerLogExtension[any]) *PrivateState {
	return &PrivateState{
		PlanMap: generic.NewGenericSyncMap[string, PlanData](),
	}
}
