package state

import (
	"oncecall/utils/generic"
)

type ThreadKey generic.Pair[string, string]

func NewIdentKey(dbID, scriptName string) ThreadKey {
	return ThreadKey{dbID, scriptName}
}

func (k *ThreadKey) DbID() string {
	return k.First
}

func (k *ThreadKey) ScriptName() string {
	return k.Second
}
