package msg

import (
	"fmt"
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
	W MessageLevel = "Warn"
	E MessageLevel = "Error"
)

type MessageDesc struct {
	Code  MessageCode
	Level MessageLevel
	Name  string
	Text  string
}

func (md *MessageDesc) SPrint() string {
	return fmt.Sprintf("[code:%d, level:%s] %s", md.Code, md.Level, md.Text)
}

var messageMap map[MessageTarget]map[MessageCode]MessageDesc

func GetMessage(target MessageTarget, code MessageCode) MessageDesc {
	return messageMap[target][code]
}
