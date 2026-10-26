package syncd

import (
	"context"
	"oncecall/cmd/oncecall_sync/plan"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"oncecall/utils/generic"
)

type PrivateState struct {
	PlanMap *generic.SyncMap[string, generic.Pair[plan.Plan, [16]byte]]
}

func NewPrivateState(ctx context.Context, m metric.MetricExtension, logger log.LoggerLogExtension[any]) *PrivateState {
	return &PrivateState{
		PlanMap: generic.NewGenericSyncMap[string, generic.Pair[plan.Plan, [16]byte]](),
	}
}
