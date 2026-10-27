
package ffi

/*
#cgo windows LDFLAGS: -lodbc32
#cgo linux LDFLAGS: -lodbc

#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>
#endif

#include <stdint.h>
#include <sql.h>
#include <sqlext.h>
#include <stdlib.h>
#include <string.h>

static SQLRETURN go_sql_driver_connect(
    SQLHDBC dbc, const char *connstr, SQLSMALLINT len
) {
    return SQLDriverConnect(
        dbc, (SQLHWND)0, (SQLCHAR *)connstr, len,
        NULL, 0, NULL, SQL_DRIVER_NOPROMPT
    );
}

static SQLRETURN go_sql_exec(
    SQLHSTMT stmt, const char *query, SQLINTEGER len
) {
    return SQLExecDirect(stmt, (SQLCHAR *)query, len);
}
*/
import "C"

import (
	"runtime"
	"unsafe"

	"oncecall/errlist"
)

type ODBCFfi struct {
	env    C.SQLHENV
	dbc    C.SQLHDBC
	closed bool
}

type boundValue struct {
	ptr unsafe.Pointer
	ind *C.SQLLEN
}

func (o *ODBCFfi) ok(rc C.SQLRETURN) bool {
	return rc == C.SQL_SUCCESS || rc == C.SQL_SUCCESS_WITH_INFO
}

func (o *ODBCFfi) newError(err error, format string, args ...any) error {
	return errlist.ErrG.NewError(err, format, args...)
}

func (o *ODBCFfi) diag(ht C.SQLSMALLINT, h C.SQLHANDLE, op string) error {
	var state [6]C.SQLCHAR
	var msg [1024]C.SQLCHAR
	var native C.SQLINTEGER
	var n C.SQLSMALLINT

	rc := C.SQLGetDiagRec(
		ht, h, 1, &state[0], &native,
		&msg[0], C.SQLSMALLINT(len(msg)), &n,
	)
	if !o.ok(rc) {
		return o.newError(nil, "%s failed", op)
	}

	return o.newError(
		nil,
		"%s: SQLSTATE=%s native=%d message=%s",
		op,
		C.GoString((*C.char)(unsafe.Pointer(&state[0]))),
		int(native),
		C.GoStringN((*C.char)(unsafe.Pointer(&msg[0])), C.int(n)),
	)
}

func OpenODBC(connectionString string) (*ODBCFfi, error) {
	o := &ODBCFfi{}

	rc := C.SQLAllocHandle(
		C.SQL_HANDLE_ENV, nil,
		(*C.SQLHANDLE)(unsafe.Pointer(&o.env)),
	)
	if !o.ok(rc) {
		return nil, o.newError(nil, "SQLAllocHandle ENV failed: %d", int(rc))
	}

	rc = C.SQLSetEnvAttr(
		o.env, C.SQL_ATTR_ODBC_VERSION,
		C.SQLPOINTER(uintptr(C.SQL_OV_ODBC3)), 0,
	)
	if !o.ok(rc) {
		err := o.diag(C.SQL_HANDLE_ENV, C.SQLHANDLE(o.env), "SQLSetEnvAttr")
		C.SQLFreeHandle(C.SQL_HANDLE_ENV, C.SQLHANDLE(o.env))
		return nil, err
	}

	rc = C.SQLAllocHandle(
		C.SQL_HANDLE_DBC, C.SQLHANDLE(o.env),
		(*C.SQLHANDLE)(unsafe.Pointer(&o.dbc)),
	)
	if !o.ok(rc) {
		err := o.diag(C.SQL_HANDLE_ENV, C.SQLHANDLE(o.env), "SQLAllocHandle DBC")
		C.SQLFreeHandle(C.SQL_HANDLE_ENV, C.SQLHANDLE(o.env))
		return nil, err
	}

	cs := C.CString(connectionString)
	defer C.free(unsafe.Pointer(cs))

	rc = C.go_sql_driver_connect(
		o.dbc, cs, C.SQLSMALLINT(len(connectionString)),
	)
	runtime.KeepAlive(connectionString)

	if !o.ok(rc) {
		err := o.diag(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc), "SQLDriverConnect")
		C.SQLFreeHandle(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc))
		C.SQLFreeHandle(C.SQL_HANDLE_ENV, C.SQLHANDLE(o.env))
		return nil, err
	}

	return o, nil
}

func (o *ODBCFfi) Close() error {
	if o == nil || o.closed {
		return nil
	}
	o.closed = true

	var first error

	rc := C.SQLDisconnect(o.dbc)
	if !o.ok(rc) {
		first = o.diag(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc), "SQLDisconnect")
	}

	rc = C.SQLFreeHandle(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc))
	if !o.ok(rc) && first == nil {
		first = o.newError(nil, "SQLFreeHandle DBC failed: %d", int(rc))
	}

	rc = C.SQLFreeHandle(C.SQL_HANDLE_ENV, C.SQLHANDLE(o.env))
	if !o.ok(rc) && first == nil {
		first = o.newError(nil, "SQLFreeHandle ENV failed: %d", int(rc))
	}

	o.dbc = nil
	o.env = nil
	return first
}

func (o *ODBCFfi) SetAutocommit(enabled bool) error {
	var value C.SQLULEN = C.SQL_AUTOCOMMIT_ON
	if !enabled {
		value = C.SQL_AUTOCOMMIT_OFF
	}

	rc := C.SQLSetConnectAttr(
		o.dbc, C.SQL_ATTR_AUTOCOMMIT,
		C.SQLPOINTER(uintptr(value)), 0,
	)
	if !o.ok(rc) {
		return o.diag(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc), "SQLSetConnectAttr")
	}
	return nil
}

func (o *ODBCFfi) EndTransaction(commit bool) error {
	var completion C.SQLSMALLINT = C.SQL_ROLLBACK
	if commit {
		completion = C.SQL_COMMIT
	}

	rc := C.SQLEndTran(
		C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc), completion,
	)
	if !o.ok(rc) {
		return o.diag(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc), "SQLEndTran")
	}
	return nil
}

func (b *boundValue) free() {
	if b.ptr != nil {
		C.free(b.ptr)
		b.ptr = nil
	}
	if b.ind != nil {
		C.free(unsafe.Pointer(b.ind))
		b.ind = nil
	}
}

func (o *ODBCFfi) bind(
	stmt C.SQLHSTMT,
	index int,
	value any,
) (*boundValue, error) {
	b := &boundValue{}
	b.ind = (*C.SQLLEN)(C.malloc(C.sizeof_SQLLEN))
	if b.ind == nil {
		return nil, o.newError(nil, "allocate parameter indicator failed")
	}

	var ct C.SQLSMALLINT
	var st C.SQLSMALLINT
	var size C.SQLULEN
	var scale C.SQLSMALLINT
	var length C.SQLLEN

	switch v := value.(type) {
	case nil:
		ct, st, size, length = C.SQL_C_CHAR, C.SQL_VARCHAR, 1, 0
		b.ptr = C.calloc(1, 1)
		*b.ind = C.SQL_NULL_DATA

	case string:
		ct, st = C.SQL_C_CHAR, C.SQL_VARCHAR
		size, length = C.SQLULEN(max(1, len(v))), C.SQLLEN(len(v))
		b.ptr = C.CBytes([]byte(v))
		*b.ind = C.SQLLEN(len(v))

	case int:
		ct, st, size, length = C.SQL_C_SBIGINT, C.SQL_BIGINT, 19, C.sizeof_longlong
		b.ptr = C.malloc(C.sizeof_longlong)
		if b.ptr != nil {
			*(*C.longlong)(b.ptr) = C.longlong(v)
		}
		*b.ind = length

	case int64:
		ct, st, size, length = C.SQL_C_SBIGINT, C.SQL_BIGINT, 19, C.sizeof_longlong
		b.ptr = C.malloc(C.sizeof_longlong)
		if b.ptr != nil {
			*(*C.longlong)(b.ptr) = C.longlong(v)
		}
		*b.ind = length

	case float32:
		ct, st, size, scale, length = C.SQL_C_FLOAT, C.SQL_REAL, 7, 6, C.sizeof_float
		b.ptr = C.malloc(C.sizeof_float)
		if b.ptr != nil {
			*(*C.float)(b.ptr) = C.float(v)
		}
		*b.ind = length

	case float64:
		ct, st, size, scale, length = C.SQL_C_DOUBLE, C.SQL_DOUBLE, 15, 8, C.sizeof_double
		b.ptr = C.malloc(C.sizeof_double)
		if b.ptr != nil {
			*(*C.double)(b.ptr) = C.double(v)
		}
		*b.ind = length

	case bool:
		ct, st, size, length = C.SQL_C_BIT, C.SQL_BIT, 1, 1
		b.ptr = C.calloc(1, 1)
		if b.ptr != nil && v {
			*(*C.uchar)(b.ptr) = 1
		}
		*b.ind = 1

	default:
		b.free()
		return nil, o.newError(nil, "unsupported parameter type %T", value)
	}

	if b.ptr == nil {
		b.free()
		return nil, o.newError(nil, "allocate parameter value failed")
	}

	rc := C.SQLBindParameter(
		stmt, C.SQLUSMALLINT(index), C.SQL_PARAM_INPUT,
		ct, st, size, scale,
		C.SQLPOINTER(b.ptr), length, b.ind,
	)
	runtime.KeepAlive(value)

	if !o.ok(rc) {
		err := o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLBindParameter")
		b.free()
		return nil, err
	}

	return b, nil
}

func (o *ODBCFfi) Execute(query string, params []any) error {
	stmt, err := o.newStatement()
	if err != nil {
		return err
	}
	defer C.SQLFreeHandle(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt))

	cs := C.CString(query)
	defer C.free(unsafe.Pointer(cs))

	if len(params) == 0 {
		rc := C.go_sql_exec(stmt, cs, C.SQLINTEGER(len(query)))
		if !o.ok(rc) {
			return o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLExecDirect")
		}
		return nil
	}

	rc := C.SQLPrepare(stmt, (*C.SQLCHAR)(unsafe.Pointer(cs)), C.SQLINTEGER(len(query)))
	if !o.ok(rc) {
		return o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLPrepare")
	}

	var bound []*boundValue
	defer func() {
		for _, b := range bound {
			b.free()
		}
	}()

	for i, v := range params {
		b, err := o.bind(stmt, i+1, v)
		if err != nil {
			return err
		}
		bound = append(bound, b)
	}

	rc = C.SQLExecute(stmt)
	if !o.ok(rc) {
		return o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLExecute")
	}
	return nil
}

func (o *ODBCFfi) newStatement() (C.SQLHSTMT, error) {
	var stmt C.SQLHSTMT
	rc := C.SQLAllocHandle(
		C.SQL_HANDLE_STMT, C.SQLHANDLE(o.dbc),
		(*C.SQLHANDLE)(unsafe.Pointer(&stmt)),
	)
	if !o.ok(rc) {
		return nil, o.diag(C.SQL_HANDLE_DBC, C.SQLHANDLE(o.dbc), "SQLAllocHandle STMT")
	}
	return stmt, nil
}

func (o *ODBCFfi) Query(
	query string,
	params []any,
) ([][]any, []string, error) {
	stmt, err := o.newStatement()
	if err != nil {
		return nil, nil, err
	}
	defer C.SQLFreeHandle(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt))

	cs := C.CString(query)
	defer C.free(unsafe.Pointer(cs))

	var bound []*boundValue
	defer func() {
		for _, b := range bound {
			b.free()
		}
	}()

	if len(params) == 0 {
		rc := C.go_sql_exec(stmt, cs, C.SQLINTEGER(len(query)))
		if !o.ok(rc) {
			return nil, nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLExecDirect")
		}
	} else {
		rc := C.SQLPrepare(stmt, (*C.SQLCHAR)(unsafe.Pointer(cs)), C.SQLINTEGER(len(query)))
		if !o.ok(rc) {
			return nil, nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLPrepare")
		}

		for i, v := range params {
			b, err := o.bind(stmt, i+1, v)
			if err != nil {
				return nil, nil, err
			}
			bound = append(bound, b)
		}

		rc = C.SQLExecute(stmt)
		if !o.ok(rc) {
			return nil, nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLExecute")
		}
	}

	names, err := o.columnNames(stmt)
	if err != nil {
		return nil, nil, err
	}

	rows := make([][]any, 0)
	for {
		rc := C.SQLFetch(stmt)
		if rc == C.SQL_NO_DATA {
			break
		}
		if !o.ok(rc) {
			return nil, nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLFetch")
		}

		row := make([]any, len(names))
		for i := range names {
			v, err := o.readColumn(stmt, i+1)
			if err != nil {
				return nil, nil, err
			}
			row[i] = v
		}
		rows = append(rows, row)
	}

	return rows, names, nil
}

func (o *ODBCFfi) columnNames(stmt C.SQLHSTMT) ([]string, error) {
	var count C.SQLSMALLINT
	rc := C.SQLNumResultCols(stmt, &count)
	if !o.ok(rc) {
		return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLNumResultCols")
	}

	names := make([]string, 0, int(count))
	for i := 1; i <= int(count); i++ {
		var name [1024]C.SQLCHAR
		var n C.SQLSMALLINT
		var typ C.SQLSMALLINT
		var size C.SQLULEN
		var scale C.SQLSMALLINT
		var nullable C.SQLSMALLINT

		rc := C.SQLDescribeCol(
			stmt, C.SQLUSMALLINT(i), &name[0], C.SQLSMALLINT(len(name)),
			&n, &typ, &size, &scale, &nullable,
		)
		if !o.ok(rc) {
			return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLDescribeCol")
		}

		names = append(
			names,
			C.GoStringN((*C.char)(unsafe.Pointer(&name[0])), C.int(n)),
		)
	}
	return names, nil
}

func (o *ODBCFfi) readColumn(stmt C.SQLHSTMT, col int) (any, error) {
	var name [2]C.SQLCHAR
	var n C.SQLSMALLINT
	var typ C.SQLSMALLINT
	var size C.SQLULEN
	var scale C.SQLSMALLINT
	var nullable C.SQLSMALLINT

	rc := C.SQLDescribeCol(
		stmt, C.SQLUSMALLINT(col), &name[0], C.SQLSMALLINT(len(name)),
		&n, &typ, &size, &scale, &nullable,
	)
	if !o.ok(rc) {
		return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLDescribeCol")
	}

	switch typ {
	case C.SQL_TINYINT, C.SQL_SMALLINT, C.SQL_INTEGER:
		var v C.int
		var ind C.SQLLEN
		rc = C.SQLGetData(
			stmt, C.SQLUSMALLINT(col), C.SQL_C_SLONG,
			C.SQLPOINTER(unsafe.Pointer(&v)), C.SQLLEN(C.sizeof_int), &ind,
		)
		if !o.ok(rc) {
			return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLGetData")
		}
		if ind == C.SQL_NULL_DATA {
			return nil, nil
		}
		return int(v), nil

	case C.SQL_BIGINT:
		var v C.longlong
		var ind C.SQLLEN
		rc = C.SQLGetData(
			stmt, C.SQLUSMALLINT(col), C.SQL_C_SBIGINT,
			C.SQLPOINTER(unsafe.Pointer(&v)), C.SQLLEN(C.sizeof_longlong), &ind,
		)
		if !o.ok(rc) {
			return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLGetData")
		}
		if ind == C.SQL_NULL_DATA {
			return nil, nil
		}
		return int64(v), nil

	case C.SQL_REAL:
		var v C.float
		var ind C.SQLLEN
		rc = C.SQLGetData(
			stmt, C.SQLUSMALLINT(col), C.SQL_C_FLOAT,
			C.SQLPOINTER(unsafe.Pointer(&v)), C.SQLLEN(C.sizeof_float), &ind,
		)
		if !o.ok(rc) {
			return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLGetData")
		}
		if ind == C.SQL_NULL_DATA {
			return nil, nil
		}
		return float32(v), nil

	case C.SQL_FLOAT, C.SQL_DOUBLE, C.SQL_DECIMAL, C.SQL_NUMERIC:
		var v C.double
		var ind C.SQLLEN
		rc = C.SQLGetData(
			stmt, C.SQLUSMALLINT(col), C.SQL_C_DOUBLE,
			C.SQLPOINTER(unsafe.Pointer(&v)), C.SQLLEN(C.sizeof_double), &ind,
		)
		if !o.ok(rc) {
			return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLGetData")
		}
		if ind == C.SQL_NULL_DATA {
			return nil, nil
		}
		return float64(v), nil
	}

	const chunk = 4096
	var out []byte

	for {
		buf := C.malloc(chunk)
		if buf == nil {
			return nil, o.newError(nil, "allocate result buffer failed")
		}
		C.memset(buf, 0, chunk)

		var ind C.SQLLEN
		rc = C.SQLGetData(
			stmt, C.SQLUSMALLINT(col), C.SQL_C_CHAR,
			C.SQLPOINTER(buf), C.SQLLEN(chunk), &ind,
		)

		if rc == C.SQL_NO_DATA {
			C.free(buf)
			break
		}
		if ind == C.SQL_NULL_DATA {
			C.free(buf)
			return nil, nil
		}
		if !o.ok(rc) {
			C.free(buf)
			return nil, o.diag(C.SQL_HANDLE_STMT, C.SQLHANDLE(stmt), "SQLGetData")
		}

		n := int(C.strnlen((*C.char)(buf), chunk-1))
		if n > 0 {
			out = append(out, C.GoBytes(buf, C.int(n))...)
		}
		C.free(buf)

		if rc == C.SQL_SUCCESS {
			break
		}
	}

	return string(out), nil
}
