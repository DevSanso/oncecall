package executor

import (
	"context"
	"oncecall/utils/generic"
)

type scriptThread struct {
	state *execSharedState
	key generic.Pair[string, string]
}

func newScriptThread(key generic.Pair[string, string], state *execSharedState) *scriptThread {

}

func (s *scriptThread) Run(ctx context.Context) error {
	dbName := s.key.First
	scriptName := s.key.Second

	script := s.state.scriptMap.Load()

	for {
		select {
		case <-ctx.Done():
			break
		}


	}

	return nil
}
