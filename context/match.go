package context

import (
	"errors"
	"fmt"
	"oncecall/msg"
	"os"
	"runtime"
	"strings"
	"sync"
)

type errorFuncPc struct {
	once        sync.Once
	projectRoot string
}

func (m *errorFuncPc) getPackagePath(skip int) (funcName, fileAndLine, packagePath string) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		cDesc := msg.GetMessage(msg.COMMON_TAGET, msg.COMMON___BROKEN_GO_SYSTEM_CALL_CODE)
		panic(cDesc.SPrint())
	}

	funcName = runtime.FuncForPC(pc).Name()
	if os.PathSeparator != '/' {
		file = strings.ReplaceAll(file, string(os.PathSeparator), "/")
	}

	ptr := file[len(m.projectRoot):]
	fileAndLine = fmt.Sprintf("%s:%d", ptr, line)

	sp := strings.SplitN(ptr, "/", 3)

	if len(sp) < 2 {
		packagePath = "__unknown__"
	} else {
		packagePath = sp[1]
	}

	return
}

var (
	gFuncPC = errorFuncPc{once: sync.Once{}, projectRoot: ""}
)

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		err := errors.New("failed to get caller, err filename")
		panic(err)
	}

	gFuncPC.once.Do(func() {
		p := "/conext/match.go"
		if os.PathSeparator != '/' {
			file = strings.ReplaceAll(file, string(os.PathSeparator), "/")
		}
		gFuncPC.projectRoot = strings.Replace(file, p, "", 1)
	})
}
