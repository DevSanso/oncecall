package errlist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type errorTypeMap struct {
	once        sync.Once
	m           map[string]string
	projectRoot string
}

func (m *errorTypeMap) getPackagePath() (funcName, fileAndLine, packagePath string) {
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		panic("broken runtime caller, system panic")
	}

	funcName = runtime.FuncForPC(pc).Name()
	ptr := file[len(m.projectRoot):]
	fileAndLine = fmt.Sprintf("%s:%d", ptr, line)

	sp := strings.SplitN(ptr, string(os.PathSeparator), 3)

	if len(sp) < 2 {
		packagePath = "__unknown__"
	} else {
		packagePath = sp[1]
	}

	return
}

func (m *errorTypeMap) NewError(err error, format string, args ...any) error {
	fn, fileLine, packageName := m.getPackagePath()
	typeName, find := m.m[packageName]
	if !find {
		typeName = "Unknown"
	}

	return newWrapError(err, fn, fileLine, typeName, format, args...)
}

var (
	ErrG = errorTypeMap{m: map[string]string{
		"cfg":         "Config",
		"conn":        "Connection",
		"manage":      "Connection",
		"cmd":         "Proc",
		"initialize":  "ProcInit",
		"vm":          "Interpreter",
		"utils":       "PackageUtils",
		"tests":       "Tests",
		"__unknown__": "Unknown",
	}, once: sync.Once{}, projectRoot: ""}
)

func Init() (err error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		err = errors.New("failed to get caller, err filename")
		return
	}

	ErrG.once.Do(func() {
		p := filepath.FromSlash("/errlist/match.go")
		ErrG.projectRoot = strings.Replace(file, p, "", 1)
	})

	return
}
