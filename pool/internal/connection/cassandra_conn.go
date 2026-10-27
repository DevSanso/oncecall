package connection

import (
	"context"
	"oncecall/errlist"
	"oncecall/pool/types"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

type cassandraConn struct {
	session *gocql.Session
}

func NewCassandraConn(session *gocql.Session) (types.Conn, error) {
	return &cassandraConn{session: session}, nil
}

func (c *cassandraConn) RunExecute(ctx context.Context, arg *types.Args) error {
	sess := c.session
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

	if execErr := execFn.ExecContext(ctx); execErr != nil {
		return errlist.ErrG.NewError(execErr, "exec failed")
	}
	return nil
}
func (c *cassandraConn) makeRowBuffer(cols []gocql.ColumnInfo) (data []any, err error) {
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

func (c *cassandraConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	sess := c.session

	var dummy []any = nil
	if len(arg.Args) > 0 {
		dummy = arg.Args[0]
	} else {
		dummy = []any{}
	}

	query := sess.Query(arg.Query, dummy...)
	iter := query.IterContext(ctx)

	cols := iter.Columns()
	var valuePtr []any = make([]any, len(cols))
	name = make([]string, len(cols))

	for idx := range cols {
		name[idx] = cols[idx].Name
	}

	value, valueErr := c.makeRowBuffer(cols)
	if valueErr != nil {
		return nil, nil, errlist.ErrG.NewError(valueErr, "query cols get buffer failed : ")
	}

	for idx := range value {
		valuePtr[idx] = &value[idx]
	}

	retArr := make([][]any, 0, iter.NumRows())
	for iter.Scan(valuePtr...) {
		retArr = append(retArr, value)

		value, valueErr = c.makeRowBuffer(cols)
		if valueErr != nil {
			return nil, nil, errlist.ErrG.NewError(valueErr, "query cols get buffer failed ")
		}

		for idx := range value {
			valuePtr[idx] = &value[idx]
		}
	}

	return retArr, name, nil
}

func (c *cassandraConn) Close() error {
	c.session.Close()
	return nil
}
