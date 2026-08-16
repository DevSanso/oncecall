package conn

import (
	"context"
	"database/sql"
	"oncecall/errlist"
	"time"

	"oncecall/cfg"

	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"
)

type standardConnPool struct {
	conn *sql.DB
	name string

	conf *cfg.ConnConfig
}

func newStandardConnPool(info *cfg.ConnConfig) (ConnPoolInterface, error) {

	var db *sql.DB = nil
	var dbErr error = nil

	driver, url, err := getConnUrlAndDriver(info)
	if err != nil {
		return nil, errlist.ErrG.NewError(err, "driver not exists, name:%s", info.Name)
	}

	db, dbErr = sql.Open(driver, url)
	if dbErr != nil {
		return nil, errlist.ErrG.NewError(dbErr, "driver open failed, name:%s", info.Name)
	}

	db.SetMaxIdleConns(info.MaxConn)
	db.SetMaxOpenConns(info.MaxConn)
	db.SetConnMaxIdleTime(time.Second * 10)
	return &standardConnPool{conn: db, name: info.Name, conf: info}, nil

}

func (p *standardConnPool) GetConfig() cfg.ConnConfig {
	return *p.conf
}

func (p *standardConnPool) RunExecute(ctx context.Context, arg *Args) error {
	conn, connErr := p.conn.Conn(ctx)
	if connErr != nil {
		return errlist.ErrG.NewError(connErr, "exec conn failed, name:%s", p.name)
	}
	defer conn.Close()

	if !arg.IsTranscation {
		tx, txErr := conn.BeginTx(ctx, nil)
		if txErr != nil {
			return errlist.ErrG.NewError(txErr, "exec tx failed, name:%s, query:%s", p.name, arg.Query)
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
					return errlist.ErrG.NewError(retErr, "exec sql failed, name:%s, query:%s", p.name, arg.Query)
				}
			}
		} else {
			_, retErr := tx.ExecContext(ctx, arg.Query)

			if retErr != nil {
				isNotErr = false
				return errlist.ErrG.NewError(retErr, "exec sql(no args) failed, name:%s, query:%s", p.name, arg.Query)
			}
		}
	} else {
		if arg.Args != nil {
			for _, param := range arg.Args {
				_, retErr := conn.ExecContext(ctx, arg.Query, param...)

				if retErr != nil {
					return errlist.ErrG.NewError(retErr, "exec sql failed, name:%s, query:%s", p.name, arg.Query)
				}
			}
		} else {
			_, retErr := conn.ExecContext(ctx, arg.Query)

			if retErr != nil {
				return errlist.ErrG.NewError(retErr, "exec sql(no args) failed, name:%s, query:%s", p.name, arg.Query)
			}
		}
	}

	return nil
}

func (p *standardConnPool) RunQuery(ctx context.Context, arg *Args) ([][]any, error) {
	conn, connErr := p.conn.Conn(ctx)
	if connErr != nil {

		return nil, errlist.ErrG.NewError(connErr, "exec conn failed, name:%s", p.name)
	}
	defer conn.Close()

	if arg.IsTranscation {
		return nil, errlist.ErrG.NewError(nil, "exec sql transcation failed, name:%s", p.name)
	}

	var r *sql.Rows = nil
	if arg.Args != nil {
		param := arg.Args[len(arg.Args)-1]
		var retErr error
		r, retErr = conn.QueryContext(ctx, arg.Query, param...)
		if retErr != nil {
			return nil, errlist.ErrG.NewError(retErr, "exec sql query failed, name:%s", p.name)
		}
	} else {
		var retErr error
		r, retErr = conn.QueryContext(ctx, arg.Query)

		if retErr != nil {
			return nil, errlist.ErrG.NewError(retErr, "exec sql(no args) query failed, name:%s", p.name)
		}
	}
	defer r.Close()

	cType, colErr := r.ColumnTypes()
	if colErr != nil {
		return nil, errlist.ErrG.NewError(colErr, "can't get column type, name:%s", p.name)
	}

	ret := make([][]any, 0, 5)

	for r.Next() {
		rowD := make([]any, len(cType))
		for idx := range len(cType) {
			dType := cType[idx].DatabaseTypeName()
			if isTypeDouble(dType) {
				rowD[idx] = 0.0
			} else if isTypeSInt(dType) {
				rowD[idx] = int(0)
			} else if isTypeBigInt(dType) {
				rowD[idx] = int64(0)
			} else if isTypeText(dType) {
				rowD[idx] = ""
			} else if isTypeBytes(dType) {
				rowD[idx] = sql.RawBytes{}
			} else {
				return nil, errlist.ErrG.NewError(nil, "not support type, name:%s, type:%s", p.name, dType)
			}
		}

		retP := make([]any, len(cType))
		for idx := range len(cType) {
			retP[idx] = &rowD[idx]
		}

		if scanErr := r.Scan(retP...); scanErr != nil {
			for idx := range retP {
				retP[idx] = 0
			}
			return nil, errlist.ErrG.NewError(scanErr, "exec sql scan failed, name:%s", p.name)
		}

		ret = append(ret, rowD)
	}
	return ret, nil
}

func (p *standardConnPool) Close() error {
	return p.conn.Close()
}
