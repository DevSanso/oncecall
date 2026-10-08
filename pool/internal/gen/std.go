package gen

import (
	"context"
	"database/sql"
	"oncecall/errlist"
	"oncecall/extension/log"
	"oncecall/pool/internal/connection"
	"oncecall/pool/internal/utils"
	"oncecall/pool/types"
	"time"
)

type stdGenerator struct {
	info types.ConnConfig
	db   *sql.DB
}

func NewStdGenerator(info types.ConnConfig, lExtension log.LoggerExtension[any]) (Generator, error) {
	var db *sql.DB = nil
	var dbErr error = nil

	var util utils.StdSqlUtils

	driver, url, err := util.GetConnUrlAndDriver(&info)
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

	return &stdGenerator{info: info, db: db}, nil
}

func (s *stdGenerator) Gen(ctx context.Context, logger log.LoggerDebugExtension[any]) (types.Conn, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, errlist.ErrG.NewError(err, "get conn err, server(%s):name(%s)", s.info.Server, s.info.Name)
	}

	return connection.NewStdSqlConn(conn), nil
}
