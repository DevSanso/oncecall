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

	vmGlobalMap *generic.SyncMap[string, any]
	paramMap *generic.SyncMap[int, [][]any]
}
