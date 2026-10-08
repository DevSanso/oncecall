package connection

import (
	"context"
	"database/sql"
	"oncecall/errlist"
	"oncecall/pool/types"
	"slices"
	"strings"
)

type StdSqlConn struct {
	conn *sql.Conn
}

func NewStdSqlConn(conn *sql.Conn) types.Conn {
	return &StdSqlConn{conn: conn}
}

func (*StdSqlConn) isTypeText(t string) bool {
	li := []string{
		"VARCHAR",
		"TEXT",
		"STRING",
		"VARCHAR",
		"BPCHAR",
	}
	uppr := strings.ToUpper(t)

	for _, chkT := range li {
		chk := strings.Contains(uppr, chkT)
		if chk {
			return true
		}
	}
	return false
}

func (*StdSqlConn) isTypeBigInt(t string) bool {
	li := []string{
		"INT8",
		"BIGINT",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func (*StdSqlConn) isTypeSInt(t string) bool {
	li := []string{
		"INT",
		"INT2",
		"INT4",
		"INTEGER",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func (*StdSqlConn) isTypeDouble(t string) bool {
	li := []string{
		"DOUBLE",
		"FLOAT",
		"FLOAT4",
		"FLOAT8",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func (*StdSqlConn) isTypeBytes(t string) bool {
	li := []string{
		"BLOB",
	}

	return slices.Index(li, strings.ToUpper(t)) != -1
}

func (s *StdSqlConn) RunExecute(ctx context.Context, arg *types.Args) error {
	conn := s.conn

	if !arg.IsTransaction {
		tx, txErr := conn.BeginTx(ctx, nil)
		if txErr != nil {
			return errlist.ErrG.NewError(txErr, "exec tx failed, query:%s", arg.Query)
		}
		var isNotErr = true

		defer func() {
			if isNotErr {
				_ = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}()

		if arg.Args != nil {
			for _, param := range arg.Args {
				_, retErr := tx.ExecContext(ctx, arg.Query, param...)

				if retErr != nil {
					isNotErr = false
					return errlist.ErrG.NewError(retErr, "exec sql failed query:%s", arg.Query)
				}
			}
		} else {
			_, retErr := tx.ExecContext(ctx, arg.Query)

			if retErr != nil {
				isNotErr = false
				return errlist.ErrG.NewError(retErr, "exec sql(no args) failed, query:%s", arg.Query)
			}
		}
	} else {
		if arg.Args != nil {
			for _, param := range arg.Args {
				_, retErr := conn.ExecContext(ctx, arg.Query, param...)

				if retErr != nil {
					return errlist.ErrG.NewError(retErr, "exec sql failed, query:%s", arg.Query)
				}
			}
		} else {
			_, retErr := conn.ExecContext(ctx, arg.Query)

			if retErr != nil {
				return errlist.ErrG.NewError(retErr, "exec sql(no args) failed, query:%s", arg.Query)
			}
		}
	}

	return nil
}

func (s *StdSqlConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	conn := s.conn

	if arg.IsTransaction {
		return nil, nil, errlist.ErrG.NewError(nil, "exec sql transcation not support")
	}

	var r *sql.Rows = nil
	if arg.Args != nil && len(arg.Args) > 0 {
		param := arg.Args[0]
		var retErr error
		r, retErr = conn.QueryContext(ctx, arg.Query, param...)
		if retErr != nil {
			return nil, nil, errlist.ErrG.NewError(retErr, "exec sql query failed")
		}
	} else {
		var retErr error
		r, retErr = conn.QueryContext(ctx, arg.Query)

		if retErr != nil {
			return nil, nil, errlist.ErrG.NewError(retErr, "exec sql(no args) query failed")
		}
	}
	defer r.Close()

	cType, colErr := r.ColumnTypes()
	if colErr != nil {
		return nil, nil, errlist.ErrG.NewError(colErr, "can't get column type")
	}

	ret := make([][]any, 0, 5)
	isFrist := false

	for r.Next() {
		rowD := make([]any, len(cType))
		if !isFrist {
			name = make([]string, len(cType))
		}

		for idx := range len(cType) {
			dType := cType[idx].DatabaseTypeName()
			if s.isTypeDouble(dType) {
				rowD[idx] = 0.0
			} else if s.isTypeSInt(dType) {
				rowD[idx] = int(0)
			} else if s.isTypeBigInt(dType) {
				rowD[idx] = int64(0)
			} else if s.isTypeText(dType) {
				rowD[idx] = ""
			} else if s.isTypeBytes(dType) {
				rowD[idx] = sql.RawBytes{}
			} else {
				return nil, nil, errlist.ErrG.NewError(nil, "not support type,type:%s", dType)
			}

			if !isFrist {
				name[idx] = cType[idx].Name()
			}
		}
		isFrist = true

		retP := make([]any, len(cType))
		for idx := range len(cType) {
			retP[idx] = &rowD[idx]
		}

		if scanErr := r.Scan(retP...); scanErr != nil {
			for idx := range retP {
				retP[idx] = 0
			}
			return nil, nil, errlist.ErrG.NewError(scanErr, "exec sql scan failed,")
		}

		ret = append(ret, rowD)
	}
	return ret, name, nil
}

func (s *StdSqlConn) Close() error {
	return s.conn.Close()
}
