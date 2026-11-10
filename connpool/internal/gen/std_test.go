package gen

import (
	"context"
	"fmt"
	"oncecall/extension/log"
	"testing"

	"oncecall/connpool/types"
)

const testTable = "std_sql_conn_test"

func setupTestDB(t *testing.T) types.Conn {

	generator, genErr := NewStdGenerator(types.ConnConfig{
		DBType:    "postgres",
		Name:      "postgres",
		Server:    "127.0.0.1",
		Id:        "postgres",
		Password:  "postgres",
		MaxConn:   100,
		OptionMap: nil,
	}, &log.NoLogLoggerExtension{})

	if genErr != nil {
		t.Fatal(genErr)
	}

	conn, connErr := generator.Gen(context.Background(), &log.NoLogLoggerExtension{})
	if connErr != nil {
		t.Fatal(connErr)
	}

	t.Helper()

	err := conn.RunExecute(context.Background(), &types.Args{
		Query: fmt.Sprintf(
			"DROP TABLE IF EXISTS %s", testTable,
		),
		Args:          nil,
		IsTransaction: false,
	})

	if err != nil {
		conn.Close()
		t.Fatalf("DROP TABLE failed: %v", err)
	}

	err = conn.RunExecute(context.Background(), &types.Args{
		Query: fmt.Sprintf(
			`
			CREATE TABLE %s (
				id BIGINT PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				score INTEGER NOT NULL
			)
		`, testTable,
		),
		Args:          nil,
		IsTransaction: false,
	})

	if err != nil {
		conn.Close()
		t.Fatalf("CREATE TABLE failed: %v", err)
	}

	return conn
}

func TestNewStdSqlConn(t *testing.T) {
	conn := setupTestDB(t)

	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
}

func TestStdSqlConnInsert(t *testing.T) {
	conn := setupTestDB(t)
	ctx := context.Background()

	err := conn.RunExecute(ctx, &types.Args{
		Query: fmt.Sprintf(`
			INSERT INTO %s (id, name, score)
			VALUES ($1, $2, $3)
		`, testTable),
		Args: [][]any{
			{1, "user1", 100},
		},
	})

	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}
}

func TestStdSqlConnSelect(t *testing.T) {
	conn := setupTestDB(t)
	ctx := context.Background()

	err := conn.RunExecute(ctx, &types.Args{
		Query: fmt.Sprintf(`
			INSERT INTO %s (id, name, score)
			VALUES ($1, $2, $3)
		`, testTable),
		Args: [][]any{
			{1, "user1", 100},
			{2, "user2", 200},
		},
	})
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	rows, names,_, err := conn.RunQuery(ctx, &types.Args{
		Query: fmt.Sprintf(`
			SELECT id, name, score
			FROM %s
			ORDER BY id
		`, testTable),
	})
	if err != nil {
		t.Fatalf("SELECT failed: %v", err)
	}

	expectedNames := []string{"id", "name", "score"}

	if len(names) != len(expectedNames) {
		t.Fatalf("expected 3 columns, got %d", len(names))
	}

	for i, name := range expectedNames {
		if names[i] != name {
			t.Errorf("expected column %s, got %s", name, names[i])
		}
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	expected := [][]string{
		{"1", "user1", "100"},
		{"2", "user2", "200"},
	}

	for i, row := range rows {
		if len(row) != 3 {
			t.Fatalf("row %d: expected 3 columns, got %d", i, len(row))
		}

		for j, value := range row {
			if fmt.Sprint(value) != expected[i][j] {
				t.Errorf(
					"row %d column %d: expected %s, got %v",
					i, j, expected[i][j], value,
				)
			}
		}
	}
}

func TestStdSqlConnUpdate(t *testing.T) {
	conn := setupTestDB(t)
	ctx := context.Background()

	err := conn.RunExecute(ctx, &types.Args{
		Query: fmt.Sprintf(`
			INSERT INTO %s (id, name, score)
			VALUES ($1, $2, $3)
		`, testTable),
		Args: [][]any{
			{1, "user1", 100},
		},
	})
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	err = conn.RunExecute(ctx, &types.Args{
		Query: fmt.Sprintf(`
			UPDATE %s
			SET name = $1, score = $2
			WHERE id = $3
		`, testTable),
		Args: [][]any{
			{"updated_user", 300, 1},
		},
	})
	if err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}

	rows, _, _, err := conn.RunQuery(ctx, &types.Args{
		Query: fmt.Sprintf(`
			SELECT name, score
			FROM %s
			WHERE id = $1
		`, testTable),
		Args: [][]any{
			{1},
		},
	})
	if err != nil {
		t.Fatalf("SELECT after UPDATE failed: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if fmt.Sprint(rows[0][0]) != "updated_user" {
		t.Errorf("expected updated_user, got %v", rows[0][0])
	}

	if fmt.Sprint(rows[0][1]) != "300" {
		t.Errorf("expected score 300, got %v", rows[0][1])
	}
}

func TestStdSqlConnDelete(t *testing.T) {
	conn := setupTestDB(t)
	ctx := context.Background()

	err := conn.RunExecute(ctx, &types.Args{
		Query: fmt.Sprintf(`
			INSERT INTO %s (id, name, score)
			VALUES ($1, $2, $3)
		`, testTable),
		Args: [][]any{
			{1, "user1", 100},
		},
	})
	if err != nil {
		t.Fatalf("INSERT failed: %v", err)
	}

	err = conn.RunExecute(ctx, &types.Args{
		Query: fmt.Sprintf(`
			DELETE FROM %s WHERE id = $1
		`, testTable),
		Args: [][]any{
			{1},
		},
	})
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}

	rows, _,_, err := conn.RunQuery(ctx, &types.Args{
		Query: fmt.Sprintf(`
			SELECT id FROM %s
		`, testTable),
	})
	if err != nil {
		t.Fatalf("SELECT after DELETE failed: %v", err)
	}

	if len(rows) != 0 {
		t.Errorf("expected 0 rows after DELETE, got %d", len(rows))
	}
}
