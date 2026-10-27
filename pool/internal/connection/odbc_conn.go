package connection

import (
	"context"
	"oncecall/pool/types"
	"strings"
	"sync"

	"oncecall/errlist"
	"oncecall/pool/internal/ffi"
)


type odbcConn struct {
	mu     sync.Mutex
	raw    *ffi.ODBCFfi
	closed bool
}


func NewOdbcConn(raw *ffi.ODBCFfi) types.Conn {
	return &odbcConn{raw: raw}
}

func (c *odbcConn) RunExecute(ctx context.Context, arg *types.Args) error {
	if arg == nil {
		return errlist.ErrG.NewError(nil, "ODBC execute arguments are nil")
	}
	if strings.TrimSpace(arg.Query) == "" {
		return errlist.ErrG.NewError(nil, "ODBC execute query is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.raw == nil {
		return errlist.ErrG.NewError(nil, "ODBC connection is closed")
	}
	if err := ctx.Err(); err != nil {
		return errlist.ErrG.NewError(err, "ODBC execute canceled")
	}

	if arg.IsTransaction {
		if err := c.raw.SetAutocommit(false); err != nil {
			return errlist.ErrG.NewError(err, "ODBC begin transaction failed")
		}
	}

	sets := arg.Args
	if len(sets) == 0 {
		sets = [][]any{nil}
	}

	var execErr error
	for _, params := range sets {
		if err := ctx.Err(); err != nil {
			execErr = err
			break
		}
		if err := c.raw.Execute(arg.Query, params); err != nil {
			execErr = err
			break
		}
	}

	if arg.IsTransaction {
		if execErr != nil {
			if err := c.raw.EndTransaction(false); err != nil {
				execErr = errlist.ErrG.NewError(
					err,
					"ODBC rollback failed after execution error: %v",
					execErr,
				)
			}
		} else if err := c.raw.EndTransaction(true); err != nil {
			execErr = errlist.ErrG.NewError(err, "ODBC commit failed")
		}

		if err := c.raw.SetAutocommit(true); err != nil {
			return errlist.ErrG.NewError(err, "ODBC restore autocommit failed")
		}
	}

	if execErr != nil {
		return errlist.ErrG.NewError(execErr, "ODBC execute failed")
	}
	return nil
}

func (c *odbcConn) RunQuery(
	ctx context.Context,
	arg *types.Args,
) ([][]any, []string, error) {
	if arg == nil {
		return nil, nil, errlist.ErrG.NewError(nil, "ODBC query arguments are nil")
	}
	if strings.TrimSpace(arg.Query) == "" {
		return nil, nil, errlist.ErrG.NewError(nil, "ODBC query is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.raw == nil {
		return nil, nil, errlist.ErrG.NewError(nil, "ODBC connection is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, errlist.ErrG.NewError(err, "ODBC query canceled")
	}

	sets := arg.Args
	if len(sets) == 0 {
		sets = [][]any{nil}
	}
	if len(sets) != 1 {
		return nil, nil, errlist.ErrG.NewError(
			nil,
			"ODBC query expects one parameter set, got %d",
			len(sets),
		)
	}

	rows, names, err := c.raw.Query(arg.Query, sets[0])
	if err != nil {
		return nil, nil, errlist.ErrG.NewError(err, "ODBC query failed")
	}

	if err := ctx.Err(); err != nil {
		return nil, nil, errlist.ErrG.NewError(err, "ODBC query canceled after execution")
	}

	return rows, names, nil
}

func (c *odbcConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.raw == nil {
		return nil
	}

	err := c.raw.Close()
	c.raw = nil

	if err != nil {
		return errlist.ErrG.NewError(err, "ODBC connection close failed")
	}
	return nil
}
