package plan

import "io"

type Plan interface {
	Run(planState *PlanState, isWait bool) error
	io.Closer
}
