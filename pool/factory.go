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
	"time"
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
	poolLogExtension log.LoggerLogExtension[any],
	genLogExtension log.LoggerLogExtension[any],
	connLogExtension log.LoggerLogExtension[any]) (types.ConnPoolCloser, error) {
	var g gen.Generator
	var gErr error
	switch define.POOLType(info.DBType) {
	case define.REDIS:
		g, gErr = gen.NewRedisGenerator(info, connLogExtension)
	case define.LOCAL:
		g, gErr = gen.NewLocalGenerator(info, connLogExtension)
	case define.KAFKA:
		g, gErr = gen.NewKafkaGenerator(info, connLogExtension)
	case define.SSH:
		g, gErr = gen.NewSSHGenerator(info, connLogExtension)
	case define.CASSANDRA:
		g, gErr = gen.NewCassandraGenerator(info, connLogExtension)
	case define.ODBC:
		g, gErr = gen.NewODBCGenerator(info, connLogExtension)
	default:
		g, gErr = gen.NewStdGenerator(info, connLogExtension)
	}

	if gErr != nil {
		return nil, errlist.ErrG.NewError(gErr, "new conn pool failed ident:%s", ident)
	}

	cp := &connPool{
		ident:         ident,
		conf:          info,
		connGenerator: g,
		lExtension:    poolLogExtension,
	}

	cp.pExtension = &pool.SimplePoolExtension[types.Conn, retWrapper]{
		Name: ident,
		Gen: func(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
			return cp.connGenerator.Gen(ctx, l)
		},
		Max:         info.MaxConn,
		Metric:      metric,
		Logger:      genLogExtension,
		Context:     ctx,
		IdleTimeout: time.Second * 61,
	}

	poolLogExtension.Debug("create conn pool (ident:%s,server:%s,dbtype:%s)", ident, info.Server, info.DBType)

	return cp, nil
}

func (c *connPool) RunExecute(ctx context.Context, arg *types.Args) error {
	c.lExtension.Debug("Try RunExecute (ident:%s, query:%s)", c.ident, arg.Query)

	now := time.Now()

	_, err := c.pExtension.Use(func(data types.Conn) (retWrapper, error) {
		err := data.RunExecute(ctx, arg)
		if err != nil {
			return retWrapper{}, errlist.ErrG.NewError(err, "RunExecute Failed : %s", c.ident)
		}
		return retWrapper{}, nil
	})

	if err == nil {
		epel := time.Now().Sub(now).Seconds()
		c.lExtension.Debug("RunExecute Success: (ident:%s, epel:%f, query:%s, isTran:%v)",
			c.ident, epel, arg.Query, arg.IsTransaction)
	}

	return err
}

func (c *connPool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	c.lExtension.Debug("Try RunQuery (ident:%s, query:%s)", c.ident, arg.Query)
	now := time.Now()
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
	epel := time.Now().Sub(now).Seconds()
	c.lExtension.Debug("RunQuery Success: (ident:%s, epel:%f, rows:%d, cols:%d, query:%s, isTran:%v)",
		c.ident, epel, len(ret.rows), len(ret.rows), arg.Query, arg.IsTransaction)

	return ret.rows, ret.colName, nil
}

func (c *connPool) GetConfig() types.ConnConfig {
	return c.conf
}

func (c *connPool) Close() error {
	return c.pExtension.Close()
}
