package gen

import (
	"context"
	"fmt"
	"oncecall/extension/log"
	"reflect"
	"testing"

	"oncecall/connpool/types"
)

func setupOdbcDB() (types.Conn, error) {
	g, gErr := NewODBCGenerator(types.ConnConfig{
		OptionMap: map[string]any{
			"dsn": "DRIVER={PostgreSQL Unicode};SERVER=127.0.0.1;PORT=5432;DATABASE=postgres;UID=postgres;PWD=postgres;",
		},
	}, &log.NoLogLoggerExtension{})
	if gErr != nil {
		return nil, gErr
	}

	return g.Gen(context.Background(), &log.NoLogLoggerExtension{})
}

func TestOdbcConnect(t *testing.T) {
	conn, err := setupOdbcDB()
	if err != nil {
		t.Fatalf("gen err : %s", err.Error())
	}
	err = conn.Close()
	if err != nil {
		t.Fatalf("close error : %s", err.Error())
	}
}

func TestOdbcConnectRunQuery(t *testing.T) {
	conn, err := setupOdbcDB()
	if err != nil {
		t.Fatalf("gen err : %s", err.Error())
	}
	defer conn.Close()

	data := [][]any{
		{1, "test", 0.2},
		{2, "ssss", 1.1},
	}

	rowName := []string{"num", "name", "num2"}

	cData, rRow,_, rErr := conn.RunQuery(context.Background(), &types.Args{
		Query: `SELECT cast(? as int4) as num, ? as name, cast(? as numeric) as num2 union all SELECT cast(? as int4) as num, ? as name, cast(? as numeric) as num2`,
		Args: [][]any{
			{data[0][0], data[0][1], data[0][2], data[1][0], data[1][1], data[1][2]},
		},
		IsTransaction: false,
	})
	if rErr != nil {
		t.Fatalf("run error:%s", rErr.Error())
	}

	if !reflect.DeepEqual(data[0], cData[0]) {
		t.Errorf("data mismatch: got %v, want %v", data[0], cData[0])
	}

	if !reflect.DeepEqual(data[1], cData[1]) {
		t.Errorf("data 1 mismatch: got %v, want %v", data[1], cData[1])
	}

	if !reflect.DeepEqual(rowName, rRow) {
		t.Errorf("name mismatch: got %v, want %v", rowName, rRow)
	}
}

func TestOdbcConnectRunExecute(t *testing.T) {
	conn, err := setupOdbcDB()
	if err != nil {
		t.Fatalf("gen err : %s", err.Error())
	}
	defer conn.Close()

	appName := "testing"

	err = conn.RunExecute(context.Background(), &types.Args{
		Query:         fmt.Sprintf("set application_name  = '%s'", appName),
		Args:          nil,
		IsTransaction: false,
	})

	if err != nil {
		t.Fatalf("%s", err.Error())
	}

	data, _, _, rErr := conn.RunQuery(context.Background(), &types.Args{
		Query: `SELECT application_name 
		FROM pg_stat_activity 
		WHERE pid = pg_backend_pid()`,
		Args:          nil,
		IsTransaction: false,
	})

	if rErr != nil {
		t.Fatalf("%s", rErr.Error())
	}

	if appName != data[0][0] {
		t.Errorf("app name mismatch: got %v, want %v", appName, data[0][0])
	}

}
