package plan

type Plan interface {
	Run(planState *PlanState, isWait bool) error
}
