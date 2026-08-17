package conn

import (
	"context"
	"oncecall/cfg"
	"oncecall/errlist"
	"oncecall/utils"
	"strconv"
	"strings"
	"sync/atomic"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

type CassandraConn struct {
	cluster *gocql.ClusterConfig
	conf    *cfg.ConnConfig

	isClose    atomic.Bool
	baseCtx    context.Context
	baseCancel context.CancelFunc
}

func newCassandraConnPool(info *cfg.ConnConfig) (ConnPoolInterface, error) {
	hosts := strings.Split(info.Server, ",")
	port := strings.Split(info.Server, ":")
	if len(hosts) <= 0 || len(port) < 2 {
		return nil, errlist.ErrG.NewError(nil, "address check : %s", info.Server)
	}

	portNum, portErr := strconv.Atoi(port[1])
	if portErr != nil {
		return nil, errlist.ErrG.NewError(portErr, "port convert failed : %s", port[1])
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = portNum
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: info.Id,
		Password: info.Password,
	}
	cluster.Keyspace = info.Name
	cluster.NumConns = info.MaxConn
	cluster.Consistency = gocql.Quorum

	ctx, cancel := context.WithCancel(context.Background())

	return &CassandraConn{
		cluster:    cluster,
		conf:       info,
		isClose:    atomic.Bool{},
		baseCtx:    ctx,
		baseCancel: cancel,
	}, nil
}

func (c *CassandraConn) RunExecute(ctx context.Context, arg *Args) error {
	if c.isClose.Load() {
		return errlist.ErrG.NewError(nil, "connection is closed")
	}

	sess, sessErr := c.cluster.CreateSession()
	if sessErr != nil {
		return errlist.ErrG.NewError(sessErr, "connect failed : %s", c.conf.Name)
	}
	defer sess.Close()

	var execFn interface {
		ExecContext(ctx context.Context) error
	} = nil

	if arg.IsTransaction {
		batch := sess.Batch(gocql.LoggedBatch)
		for idx := range arg.Args {
			batch.Query(arg.Query, arg.Args[idx]...)
		}
		execFn = batch
	} else {
		var dummy []any = nil
		if len(arg.Args) > 0 {
			dummy = arg.Args[0]
		} else {
			dummy = []any{}
		}
		execFn = sess.Query(arg.Query, dummy...)
	}
	anyContext, anyCyxCancelFn := utils.AnyContext(c.baseCtx, ctx)
	defer anyCyxCancelFn()
	if execErr := execFn.ExecContext(anyContext); execErr != nil {
		return errlist.ErrG.NewError(execErr, "exec failed : %s", c.conf.Name)
	}
	return nil
}
func (c *CassandraConn) makeRowBuffer(cols []gocql.ColumnInfo) (data []any, err error) {
	data = make([]any, len(cols))

	for i, col := range cols {
		switch col.TypeInfo.Type() {
		case gocql.TypeVarchar, gocql.TypeText:
			data[i] = ""
		case gocql.TypeFloat, gocql.TypeDouble:
			data[i] = 0.0
		case gocql.TypeBlob:
			data[i] = []byte("")
		case gocql.TypeBigInt:
			data[i] = int64(0)
		case gocql.TypeInt, gocql.TypeSmallInt:
			data[i] = int(0)
		default:
			return nil, errlist.ErrG.NewError(nil, "not support type:%s", col.TypeInfo.Type())
		}

	}
	return
}

func (c *CassandraConn) RunQuery(ctx context.Context, arg *Args) ([][]any, error) {
	if c.isClose.Load() {
		return nil, errlist.ErrG.NewError(nil, "connection is closed")
	}
	if arg.IsTransaction {
		return nil, errlist.ErrG.NewError(nil, "exec sql transcation not support, name:%s", c.conf.Name)
	}

	sess, sessErr := c.cluster.CreateSession()
	if sessErr != nil {
		return nil, errlist.ErrG.NewError(sessErr, "connect failed : %s", c.conf.Name)
	}
	defer sess.Close()

	var dummy []any = nil
	if len(arg.Args) > 0 {
		dummy = arg.Args[0]
	} else {
		dummy = []any{}
	}

	query := sess.Query(arg.Query, dummy...)
	anyContext, anyCyxCancelFn := utils.AnyContext(c.baseCtx, ctx)
	defer anyCyxCancelFn()
	iter := query.IterContext(anyContext)

	cols := iter.Columns()
	var valuePtr []any = make([]any, len(cols))

	value, valueErr := c.makeRowBuffer(cols)
	if valueErr != nil {
		return nil, errlist.ErrG.NewError(valueErr, "query cols get buffer failed : %s", c.conf.Name)
	}

	for idx := range value {
		valuePtr[idx] = &value[idx]
	}

	retArr := make([][]any, 0, iter.NumRows())
	for iter.Scan(valuePtr...) {
		retArr = append(retArr, value)

		value, valueErr = c.makeRowBuffer(cols)
		if valueErr != nil {
			return nil, errlist.ErrG.NewError(valueErr, "query cols get buffer failed : %s", c.conf.Name)
		}

		for idx := range value {
			valuePtr[idx] = &value[idx]
		}
	}

	return retArr, nil
}

func (c *CassandraConn) GetConfig() cfg.ConnConfig {
	return *c.conf
}

func (c *CassandraConn) Close() error {
	c.isClose.Store(true)
	c.baseCancel()

	return nil
}

var _ ConnPoolInterface = (*CassandraConn)(nil)
