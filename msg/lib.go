package msg

import (
	"sync"
)

type MessageCode int

type MessageTarget int
type MessageLevel string

type WrapMsg struct {
	highLevel MessageLevel

	first       *WrapMsg
	next        *WrapMsg
	funcName    string
	fileAndLine string
	typeName    string
	message     string
	realErr     error
}

type msgPackageNameMap struct {
	once        sync.Once
	m           map[string]string
	projectRoot string
}

var (
	C MessageLevel = "Critical"
	N MessageLevel = "Normal"
	W MessageLevel = "Warn"
	E MessageLevel = "Error"
)

type MessageDesc struct {
	Level MessageLevel
	Name  string
	Text  string
}

var messageMap map[MessageTarget]map[MessageCode]MessageDesc

func GetMessage(target MessageTarget, code MessageCode) MessageDesc {
	return messageMap[target][code]
}
