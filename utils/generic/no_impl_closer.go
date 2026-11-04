package generic

type NoImplCloser[T any] struct {
	Val T
}


func (NoImplCloser[T]) Close() error {
	return nil
}