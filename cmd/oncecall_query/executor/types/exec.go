package types

import "io"

type Executor interface {
	Update(timeoutMs int64) error
	Fetch(timeoutMs int64) error
	DisPatch() error
	io.Closer
}
