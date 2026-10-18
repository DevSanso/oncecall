package types

import (
	"io"
	"oncecall/utils/generic"
)

type IdentKey generic.Pair[string, string]

func NewIdentKey(dbID, scriptName string) IdentKey {
	return IdentKey{dbID, scriptName}
}

func (k *IdentKey) DbID() string {
	return k.First
}

func (k *IdentKey) ScriptName() string {
	return k.Second
}

type Executor interface {
	Update(timeoutMs int64) error
	Fetch(timeoutMs int64) error
	DisPatch() error
	io.Closer
}
