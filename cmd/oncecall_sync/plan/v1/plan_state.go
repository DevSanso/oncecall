package plan

import (
	"context"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"oncecall/utils/generic"
	"sync/atomic"
)

type PlanState struct {
	ctx context.Context

	isInitSelf  atomic.Bool
	isInitOther atomic.Bool

	vmGlobalMap *generic.SyncMap[string, any]
	paramMap    *generic.SyncMap[int, [][]any]
	sqlDB       sqlCacheMap
}

func NewPlanState(ctx context.Context, planMetric metric.MetricExtension, planLogger log.LoggerLogExtension[any]) (*PlanState, error) {
	cacheSQL, sqlErr := newSQLCacheMap(ctx, planMetric, planLogger)
	if sqlErr != nil {
		return nil, sqlErr
	}

	return &PlanState{
		ctx: ctx,
		isInitOther: atomic.Bool{},
		isInitSelf:  atomic.Bool{},
		vmGlobalMap: generic.NewGenericSyncMap[string, any](),
		paramMap : generic.NewGenericSyncMap[int, [][]any](),
		sqlDB : cacheSQL,
	}, nil
}
