package syncd

import (
	"context"

	"oncecall/extension/log"
	"oncecall/extension/metric"
	"oncecall/utils/generic"
)

type PlanData struct {
	Hash [16]byte
}
type PrivateState struct {
	PlanMap *generic.SyncMap[string, PlanData]
}

func NewPrivateState(ctx context.Context, m metric.MetricExtension, logger log.LoggerLogExtension[any]) *PrivateState {
	return &PrivateState{
		PlanMap: generic.NewGenericSyncMap[string, PlanData](),
	}
}
