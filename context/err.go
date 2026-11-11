package context

import (
	"errors"
	"fmt"
	"oncecall/msg"
	"strings"
)

type wrapStack struct {
	msgDesc msg.MessageDesc

	funcName    string
	fileAndLine string
	packageName string
	logData     string
	realErr     error
}
type wrapError struct {
	stack []wrapStack
}

func (w *wrapError) Error() string {
	var builder strings.Builder
	if len(w.stack) <= 0 {
		cDesc := msg.GetMessage(msg.COMMON_TAGET, msg.COMMON_CONTEXT_WRAP_ERROR_EMPTY_CODE)
		panic(cDesc.SPrint())
	}
	top := w.stack[0]

	builder.WriteString(fmt.Sprintf("<Code:%d, Level:%s, Message:%s> : %s\n\t(package:%s, fileAndLine:%s, func:%s, log:%s)\n",
		top.msgDesc.Code,
		top.msgDesc.Level,
		top.msgDesc.Name,
		top.msgDesc.Text,
		top.packageName,
		top.fileAndLine,
		top.funcName,
		top.logData))

	if len(w.stack) == 1 {
		return builder.String()
	}

	for idx, item := range w.stack[1:] {
		if item.msgDesc.Code == msg.COMMON_CONTEXT_WRAP_ERROR_TOP_IS_DUMMY_CODE {
			panic(fmt.Sprintf(" (package:%s, fileAndLine:%s, func:%s) : %s",
				item.packageName,
				item.fileAndLine,
				item.funcName,
				item.msgDesc.SPrint()))
		}
		builder.WriteString(fmt.Sprintf("%d = [package:%s, fileAndLine:%s, func:%s, log:%s]\n", idx, item.packageName, item.fileAndLine, item.funcName, item.logData))
	}

	return builder.String()
}

func (w *wrapError) push(err error, msgDesc *msg.MessageDesc, funcName, fileAndLine, packageName, format string, args ...any) {
	var mD msg.MessageDesc
	if msgDesc == nil {
		mD = msg.GetMessage(msg.COMMON_TAGET, msg.COMMON_CONTEXT_WRAP_ERROR_TOP_IS_DUMMY_CODE)
	} else {
		mD = *msgDesc
	}

	w.stack = append(w.stack, wrapStack{
		msgDesc:     mD,
		funcName:    funcName,
		fileAndLine: fileAndLine,
		packageName: packageName,
		logData:     fmt.Sprintf(format, args...),
		realErr:     err,
	})
}

func newWrapError(err error, msgDesc *msg.MessageDesc, funcName, fileAndLine, packageName, format string, args ...any) *wrapError {
	var wrap *wrapError
	wrapOk := errors.As(err, &wrap)

	var mD msg.MessageDesc
	if msgDesc == nil {
		mD = msg.GetMessage(msg.COMMON_TAGET, msg.COMMON_CONTEXT_WRAP_ERROR_TOP_IS_DUMMY_CODE)
	} else {
		mD = *msgDesc
	}

	if wrapOk {
		wrap.stack = append(wrap.stack, wrapStack{
			msgDesc:     mD,
			funcName:    funcName,
			fileAndLine: fileAndLine,
			packageName: packageName,
			logData:     fmt.Sprintf(format, args...),
			realErr:     err,
		})
	} else {
		wrap = &wrapError{}
		wrap.stack = make([]wrapStack, 0, 3)
		wrap.stack = append(wrap.stack, wrapStack{
			msgDesc:     mD,
			funcName:    funcName,
			fileAndLine: fileAndLine,
			packageName: packageName,
			logData:     fmt.Sprintf(format, args...),
			realErr:     err,
		})
	}

	return wrap
}

var _ error = (*wrapError)(nil)
