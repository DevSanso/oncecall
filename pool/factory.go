package pool

import (
	"context"
	"oncecall/define"
	"oncecall/errlist"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"oncecall/extension/pool"
	"oncecall/pool/internal/gen"
	"oncecall/pool/types"
)

type retWrapper struct {
	rows    [][]any
	colName []string
}
type connPool struct {
	ident string
	conf  types.ConnConfig

	connGenerator gen.Generator

	lExtension log.LoggerLogExtension[any]
	pExtension pool.PoolExtension[types.Conn, retWrapper]
}

func NewConnPool(ctx context.Context, ident string, info types.ConnConfig,
	metric metric.MetricExtension,
	poolExtensionLogger log.LoggerLogExtension[any],
	rawPoolLogger log.LoggerLogExtension[any]) (types.ConnPoolInterface, error) {
	var g gen.Generator
	var gErr error
	switch define.POOLType(info.DBType) {
	case define.REDIS:

		g, gErr = gen.NewRedisGenerator(info, rawPoolLogger)
	case define.LOCAL:
		g, gErr = gen.NewLocalGenerator(info, rawPoolLogger)
	case define.KAFKA:
		g, gErr = gen.NewKafkaGenerator(info, rawPoolLogger)
	case define.SSH:
		g, gErr = gen.NewSSHGenerator(info, rawPoolLogger)
	case define.CASSANDRA:
		g, gErr = gen.NewCassandraGenerator(info, rawPoolLogger)
	default:

		g, gErr = gen.NewStdGenerator(info, rawPoolLogger)
	}

	if gErr != nil {
		return nil, errlist.ErrG.NewError(gErr, "new conn pool failed ident:%s", ident)
	}

	cp := &connPool{
		ident:         ident,
		conf:          info,
		connGenerator: g,
		lExtension:    poolExtensionLogger,
	}

	cp.pExtension = &pool.SimplePoolExtension[types.Conn, retWrapper]{
		Name: ident,
		Gen: func(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
			return cp.connGenerator.Gen(ctx, l)
		},
		Max:     info.MaxConn,
		Metric:  metric,
		Logger:  poolExtensionLogger,
		Context: ctx,
	}

	poolExtensionLogger.Debug("create conn pool (ident:%s,server:%s,dbtype:%s)", ident, info.Server, info.DBType)

	return cp, nil
}

func (c *connPool) RunExecute(ctx context.Context, arg *types.Args) error {
	_, err := c.pExtension.Use(func(data types.Conn) (retWrapper, error) {
		err := data.RunExecute(ctx, arg)
		if err != nil {
			return retWrapper{}, errlist.ErrG.NewError(err, "RunExecute Failed : %s", c.ident)
		}
		return retWrapper{}, nil
	})

	if err == nil {
		c.lExtension.Debug("RunExecute Success: (ident:%s, query:%s, isTran:%v)",
			c.ident, arg.Query, arg.IsTransaction)
	}

	return err
}

func (c *connPool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	ret, err := c.pExtension.Use(func(data types.Conn) (retWrapper, error) {
		err := data.RunExecute(ctx, arg)
		if err != nil {
			return retWrapper{}, errlist.ErrG.NewError(err, "RunExecute Failed : %s", c.ident)
		}
		return retWrapper{}, nil
	})

	if err != nil {
		return nil, nil, err
	}

	c.lExtension.Debug("RunQuery Success: (ident:%s, rows:%d, cols:%d, query:%s, isTran:%v)",
		c.ident, len(ret.rows), len(ret.rows), arg.Query, arg.IsTransaction)

	return ret.rows, ret.colName, nil
}

func (c *connPool) GetConfig() types.ConnConfig {
	return c.conf
}

func (c *connPool) Close() error {
	return c.pExtension.Close()
}
