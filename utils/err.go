package utils

import (
	"fmt"
	"runtime"
)

func ErrorfPc(format string, data ...any) error {
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		panic("runtime.Caller failed")
	}
	fn := runtime.FuncForPC(pc)
	cutLen := len(file) - 35
	if cutLen < 0 {
		cutLen = 0
	}

	return fmt.Errorf("%s:%s:%d - %s", fn.Name(), file[cutLen:], line, fmt.Sprintf(format, data...))
}
