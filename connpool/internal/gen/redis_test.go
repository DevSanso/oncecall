package gen

import (
	"context"
	"oncecall/extension/log"

	"testing"

	"oncecall/connpool/types"
)

func setupRedisDB() (types.Conn, error) {
	genC, err := NewRedisGenerator(types.ConnConfig{
		DBType:    "redis",
		Name:      "0",
		Server:    "127.0.0.1:6379",
		Id:        "user",
		Password:  "password",
		MaxConn:   100,
		OptionMap: nil,
	}, &log.NoLogLoggerExtension{})

	if err != nil {
		return nil, err
	}

	return genC.Gen(context.Background(), &log.NoLogLoggerExtension{})
}

func TestRedisConnect(t *testing.T) {
	conn, connErr := setupRedisDB()
	if connErr != nil {
		t.Fatalf("%s", connErr.Error())
	}

	conn.Close()
}

func TestRedisExecute(t *testing.T) {
	conn, connErr := setupRedisDB()
	if connErr != nil {
		t.Fatalf("%s", connErr.Error())
	}
	defer conn.Close()
	data := "1"
	name := "1"

	eerr := conn.RunExecute(context.Background(), &types.Args{
		Query: "set ax",
		Args: [][]any{
			{1},
		},
		IsTransaction: false,
	})

	if eerr != nil {
		t.Fatalf("%s", eerr.Error())
	}

	rdata, rname, rerr := conn.RunQuery(context.Background(), &types.Args{
		Query:         "get ax",
		Args:          nil,
		IsTransaction: false,
	})

	if rerr != nil {
		t.Fatalf("%s", rerr.Error())
	}

	if len(rdata) <= 0 {
		t.Fatal("data is zero")
	}

	if data != rdata[0][0] {
		t.Errorf("data mismatch: got %v, want %v", data, rdata[0][0])
	}

	if rname[0] != name {
		t.Errorf("name mismatch: got %v, want %v", name, rname[0])
	}
}
