package context

import (
	std "context"
	"oncecall/msg"
	"time"
)

type CustomContext struct {
	base std.Context
	err  *wrapError
}

type CustomContextMsg struct {
	Target msg.MessageTarget
	Code   msg.MessageCode
}

type CustomContextLogFormat struct {
	Fmt  string
	Args []any
}

func WithStdContext(ctx std.Context) *CustomContext {
	return &CustomContext{base: ctx}
}

func (c *CustomContext) InsertErrMsg(err error, message *CustomContextMsg, logData CustomContextLogFormat) {
	var desc *msg.MessageDesc
	if message != nil {
		desc = new(msg.MessageDesc)
		*desc = msg.GetMessage(message.Target, message.Code)
	} else {
		if c.err == nil {
			cDesc := msg.GetMessage(msg.COMMON_TAGET, msg.COMMON___NIL_POINTER_CODE)
			panic(cDesc.SPrint())
		}
		desc = nil
	}

	fn, fileLine, packageName := gFuncPC.getPackagePath(1)
	if c.err == nil {
		c.err = newWrapError(err, desc, fn, fileLine, packageName, logData.Fmt, logData.Args...)
	} else {
		c.err.push(err, desc, fn, fileLine, packageName, logData.Fmt, logData.Args...)
	}
}

func (c *CustomContext) Deadline() (deadline time.Time, ok bool) {

	return c.base.Deadline()
}

func (c *CustomContext) Done() <-chan struct{} {
	return c.base.Done()
}

func (c *CustomContext) Err() error {
	return c.err
}

func (c *CustomContext) Value(key any) any {
	return c.base.Value(key)
}

var _ std.Context = (*CustomContext)(nil)
