package log

import (
	"context"
	pool "oncecall/pool/types"
	"time"

	"go.uber.org/zap"
)

type LoggerDebugExtension[PARAM any] interface {
	Debug(msg string, param ...PARAM)
}

type LoggerExtension[PARAM any] interface {
	LoggerDebugExtension[PARAM]
	Info(msg string, param ...PARAM)
	Warn(msg string, param ...PARAM)
	Error(msg string, param ...PARAM)
}

type ZapSugaredLoggerExtension struct{ L zap.SugaredLogger }

func (z *ZapSugaredLoggerExtension) Debug(msg string, param ...any) {
	z.L.Debug(msg, param)
}

func (z *ZapSugaredLoggerExtension) Error(msg string, param ...any) {
	z.L.Error(msg, param)
}

func (z *ZapSugaredLoggerExtension) Info(msg string, param ...any) {
	z.L.Info(msg, param)
}

func (z *ZapSugaredLoggerExtension) Warn(msg string, param ...any) {
	z.L.Warn(msg, param)
}

var _ LoggerExtension[any] = ((*ZapSugaredLoggerExtension)(nil))

type PoolLoggerExtension struct {
	Query     string
	TimeoutMs time.Duration
	P         pool.ConnPoolInterface
}

func (c *PoolLoggerExtension) Debug(msg string, param ...any) {
	ctx, cancelFn := context.WithTimeout(context.Background(), c.TimeoutMs)
	defer cancelFn()

	_ = c.P.RunExecute(ctx, &pool.Args{
		Query:         c.Query,
		IsTransaction: false,
		Args: [][]any{
			append([]any{"DEBUG", msg}, param...),
		},
	})
}

func (c *PoolLoggerExtension) Error(msg string, param ...any) {
	ctx, cancelFn := context.WithTimeout(context.Background(), c.TimeoutMs)
	defer cancelFn()

	_ = c.P.RunExecute(ctx, &pool.Args{
		Query:         c.Query,
		IsTransaction: false,
		Args: [][]any{
			append([]any{"ERROR", msg}, param...),
		},
	})
}

func (c *PoolLoggerExtension) Info(msg string, param ...any) {
	ctx, cancelFn := context.WithTimeout(context.Background(), c.TimeoutMs)
	defer cancelFn()

	_ = c.P.RunExecute(ctx, &pool.Args{
		Query:         c.Query,
		IsTransaction: false,
		Args: [][]any{
			append([]any{"INFO", msg}, param...),
		},
	})
}

func (c *PoolLoggerExtension) Warn(msg string, param ...any) {
	ctx, cancelFn := context.WithTimeout(context.Background(), c.TimeoutMs)
	defer cancelFn()

	_ = c.P.RunExecute(ctx, &pool.Args{
		Query:         c.Query,
		IsTransaction: false,
		Args: [][]any{
			append([]any{"WARN", msg}, param...),
		},
	})
}

var _ LoggerExtension[any] = ((*PoolLoggerExtension)(nil))
