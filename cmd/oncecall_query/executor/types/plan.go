package types

import "context"

type Plan interface {
	Run(ctx context.Context, isWait bool) error
}
