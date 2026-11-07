package gen

import (
	"context"
	"oncecall/extension/log"
	"slices"
	"testing"

	"oncecall/connpool/types"
)

func setupCassandraDB() (types.Conn, error) {
	genC, err := NewCassandraGenerator(types.ConnConfig{
		DBType:    "cassandra",
		Name:      "system",
		Server:    "127.0.0.1:9042",
		Id:        "cassandra",
		Password:  "cassandra",
		MaxConn:   100,
		OptionMap: nil,
	}, &log.NoLogLoggerExtension{})

	if err != nil {
		return nil, err
	}

	return genC.Gen(context.Background(), &log.NoLogLoggerExtension{})
}

func TestCassandraConnect(t *testing.T) {
	conn, connErr := setupCassandraDB()
	if connErr != nil {
		t.Fatalf("%s", connErr.Error())
	}

	conn.Close()
}

func TestCassandraQuery(t *testing.T) {
	conn, connErr := setupCassandraDB()
	if connErr != nil {
		t.Fatalf("%s", connErr.Error())
	}
	defer conn.Close()
	data := []any{1, "name", 0.123}
	name := []string{"n1", "n2", "n3"}

	rdata, rname, rerr := conn.RunQuery(context.Background(), &types.Args{
		Query: "select (int)? as n1, (varchar)? as n2, (double)? as n3 from system.local",
		Args: [][]any{
			data,
		},
		IsTransaction: false,
	})

	if rerr != nil {
		t.Fatalf("%s", rerr.Error())
	}

	if !slices.Equal(data, rdata[0]) {
		t.Errorf("data mismatch: got %v, want %v", data, rdata[0])
	}

	if !slices.Equal(name, rname) {
		t.Errorf("name mismatch: got %v, want %v", name, rname)
	}
}
