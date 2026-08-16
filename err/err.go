package err

import (
	"errors"
	"fmt"
	"strings"
)

type wrapError struct {
	first       *wrapError
	next        *wrapError
	funcName    string
	fileAndLine string
	typeName    string
	message     string
	realErr     error
}

func (w wrapError) Error() string {
	var builder strings.Builder
	var idx = 1
	var realErrMsg = ""
	if w.realErr != nil {
		realErrMsg = w.realErr.Error()
	}
	builder.WriteString(fmt.Sprintf("(idx:0,type:%s), (msg:%s,real:%s) [fn:%s,file:%s]", w.typeName, w.message, realErrMsg, w.funcName, w.fileAndLine))

	for e := w.next; e != nil; e = e.next {
		if e.realErr != nil {
			realErrMsg = e.realErr.Error()
		}
		builder.WriteString(fmt.Sprintf("\t\n(idx:%d,type:%s), (msg:%s,real:%s) [fn:%s,file:%s]", idx, e.typeName, e.message, realErrMsg, e.funcName, e.fileAndLine))
		idx += 1
	}

	return builder.String()
}

func newWrapError(err error, funcName, fileAndLine, typeName, format string, args ...any) *wrapError {
	newErr := &wrapError{}
	var wrap *wrapError
	wrapOk := errors.As(err, &wrap)

	newErr.fileAndLine = fileAndLine
	newErr.typeName = typeName
	newErr.funcName = funcName
	newErr.message = fmt.Sprintf(format, args...)

	if wrapOk {
		if wrap.first != nil {
			newErr.first = wrap.first
		}
		newErr.next = wrap
	} else {
		newErr.realErr = err
	}

	return newErr
}

var _ error = (*wrapError)(nil)
