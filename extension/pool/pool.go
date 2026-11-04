package pool

import "io"

const (
	poolExtensionImplMetricFullKey   = "full"
	poolExtensionImplMetricUseKey    = "use"
	poolExtensionImplMetricIdleKey   = "idle"
	poolExtensionImplMetricAccErrKey = "accErr"
)

type PoolExtension[T io.Closer, R any] interface {
	Use(useFn func(data T) (R, error)) (ret R, err error)
	Close() error
}

