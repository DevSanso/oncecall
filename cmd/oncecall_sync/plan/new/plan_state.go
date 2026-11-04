package new

import (
	"context"
	"oncecall/utils/generic"
	"sync/atomic"
)

type PlanState struct {
	ctx context.Context

	isInitSelf  atomic.Bool
	isInitOther atomic.Bool

	planBuf    planParamBuffer
	vmCacheMap *generic.SyncMap[string, any]
}
