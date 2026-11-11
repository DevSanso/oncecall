package gen

import (
	"context"
	"oncecall/connpool/internal/connection"
	"oncecall/connpool/internal/ffi"
	"oncecall/connpool/types"
	"oncecall/errset"
	"oncecall/errset/prefix"
	"oncecall/extension/log"
	"reflect"
)

type odbcGenerator struct {
	info types.ConnConfig
}

func NewODBCGenerator(info types.ConnConfig, lExtension log.LoggerLogExtension[any]) (Generator, error) {
	return &odbcGenerator{info: info}, nil
}

func (c *odbcGenerator) Gen(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
	dsnBox, exists := c.info.OptionMap["dsn"]
	if !exists {
		return nil, errset.ErrG.NewError(prefix.NotExistsError, "not exists dsn")
	}

	dsn, convOk := dsnBox.(string)
	if !convOk {
		return nil, errset.ErrG.NewError(prefix.ParseError, "parsing failed dsn : %s", reflect.ValueOf(dsnBox).Type().Name())
	}

	raw, err := ffi.OpenODBC(dsn)
	if err != nil {
		return nil, errset.ErrG.NewError(
			err,
			"ODBC connection creation failed",
		)
	}

	return connection.NewOdbcConn(raw), nil
}
