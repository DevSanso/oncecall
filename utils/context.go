package utils

import (
	"context"
)

func AnyContext(ctxs ...context.Context) (context.Context, context.CancelFunc) {
	mergedCtx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		for _, ctx := range ctxs {
			go func(c context.Context) {
				select {
				case <-c.Done():
					cancel()
				case <-mergedCtx.Done():
					return
				}
			}(ctx)
		}
		<-mergedCtx.Done()
	}()

	return mergedCtx, cancel
}
