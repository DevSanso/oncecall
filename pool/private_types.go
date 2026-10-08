package pool

import "oncecall/pool/types"

type privateConnPool interface {
	types.ConnPoolInterface
	Alloc() (all int, used int, idle int)
}
