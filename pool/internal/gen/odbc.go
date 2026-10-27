package gen

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/log"
	"oncecall/pool/internal/connection"
	"oncecall/pool/internal/ffi"
	"oncecall/pool/types"
	"reflect"
)

type odbcGenerator struct {
	info    types.ConnConfig
}

func NewODBCGenerator(info types.ConnConfig, lExtension log.LoggerLogExtension[any]) (Generator, error) {
	return &odbcGenerator{info : info}, nil
}

func (c *odbcGenerator) Gen(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
	dsnBox, exists := c.info.OptionMap["dsn"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "not exists dsn")
	}

	dsn, convOk := dsnBox.(string)
	if !convOk {
		return nil, errlist.ErrG.NewError(prefix.ParseError, "parsing failed dsn : %s", reflect.ValueOf(dsnBox).Type().Name())
	}

	raw, err := ffi.OpenODBC(dsn)
	if err != nil {
		return nil, errlist.ErrG.NewError(
			err,
			"ODBC connection creation failed",
		)
	}

	return connection.NewOdbcConn(raw), nil
}