package gen

import (
	"context"
	"oncecall/connpool/types"
	"os"
	"reflect"
	"testing"
)

func createTestShell(t *testing.T) string {
	t.Helper()

	path := "/tmp/test.sh"

	script := `#!/bin/bash

hello() {
    echo "name,type,text"
    echo "sh,cmd,command"
    echo "awk,cmd,command"
    echo "custom,22,aasws"
    echo -n "E"
}

while read -r cmd; do
    case "$cmd" in
        hello)
            hello
			echo "$cmd" >> /tmp/testing
            ;;
        exit)
            exit 0
            ;;
    esac
done
`

	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.Remove(path)
	})

	return path
}

func TestNewLocalGenerator(t *testing.T) {
	info := types.ConnConfig{
		Name: "local-test",
		OptionMap: map[string]any{
			"interactive":      true,
			"output.splitchar": ",",
			"output.newline":   "\n",
			"output.endchar":   "",
			"input.nextline":   "\n",
			"input.division":   " ",
			"input.endchar":    "\n",
			"args":             createTestShell(t),
		},
	}

	var generator Generator

	var err error
	generator, err = NewLocalGenerator(info, nil)

	if err != nil {
		t.Fatalf("NewLocalGenerator failed: %v", err)
	}

	if generator == nil {
		t.Fatal("generator is nil")
	}
}

func TestLocalGen(t *testing.T) {
	scriptPath := createTestShell(t)

	info := types.ConnConfig{
		Name: "sh",
		OptionMap: map[string]any{
			"interactive":      true,
			"output.splitchar": ",",
			"output.newline":   "\n",
			"output.endchar":   "",
			"input.nextline":   "\n",
			"input.division":   " ",
			"input.endchar":    "\n",
			"args":             scriptPath,
		},
	}

	var generator Generator
	var conn types.Conn
	var err error

	generator, err = NewLocalGenerator(info, nil)
	if err != nil {
		t.Fatal(err)
	}

	conn, err = generator.Gen(context.Background(), nil)
	if err != nil {
		t.Fatalf("Gen failed: %v", err)
	}

	if conn == nil {
		t.Fatal("connection is nil")
	}

	defer conn.Close()
}

func TestLocalInteractiveHello(t *testing.T) {
	scriptPath := createTestShell(t)

	info := types.ConnConfig{
		Name: "sh",
		OptionMap: map[string]any{
			"interactive":      true,
			"output.splitchar": ",",
			"output.newline":   "\n",
			"output.endchar":   "E",
			"input.nextline":   "\n",
			"input.division":   " ",
			"input.endchar":    "",
			"args":             scriptPath,
		},
	}

	var generator Generator
	var conn types.Conn
	var err error

	generator, err = NewLocalGenerator(info, nil)
	if err != nil {
		t.Fatal(err)
	}

	conn, err = generator.Gen(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rows, names, _, err := conn.RunQuery(context.Background(), &types.Args{
		Args: [][]any{{"hello"}},
	})
	if err != nil {
		t.Fatalf("RunQuery failed: %v", err)
	}

	expectedNames := []string{"name", "type", "text"}
	expectedRows := [][]any{
		{"sh", "cmd", "command"},
		{"awk", "cmd", "command"},
		{"custom", "22", "aasws"},
	}

	if !reflect.DeepEqual(names, expectedNames) {
		t.Errorf("names mismatch: got %v, want %v", names, expectedNames)
	}

	if !reflect.DeepEqual(rows, expectedRows) {
		t.Errorf("rows mismatch: got %v, want %v", rows, expectedRows)
	}

	rows1, names1,_, err := conn.RunQuery(context.Background(), &types.Args{
		Args: [][]any{{"hello"}},
	})

	if !reflect.DeepEqual(names1, expectedNames) {
		t.Errorf("names mismatch: got %v, want %v", names, expectedNames)
	}

	if !reflect.DeepEqual(rows1, expectedRows) {
		t.Errorf("rows mismatch: got %v, want %v", rows, expectedRows)
	}
}
