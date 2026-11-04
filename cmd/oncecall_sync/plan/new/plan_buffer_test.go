package new

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestPlanParamBufferSetGet(t *testing.T) {
	ppb := newPlanParamBuffer(2, 4, 10, 5)

	tests := []struct {
		idx  int
		col  string
		row  int
		data any
		seq  int64
	}{
		{0, "name", 0, "hello", 10},
		{0, "name", 1, "world", 10},
		{1, "age", 0, 20, 10},
	}

	for _, tt := range tests {
		if err := ppb.Set(tt.idx, tt.col, tt.row, tt.data, tt.seq); err != nil {
			t.Fatal(err)
		}
	}

	testsGet := []struct {
		idx  int
		col  string
		row  int
		seq  int64
		want any
	}{
		{0, "name", 0, 5, "hello"},
		{0, "name", 1, 5, "world"},
		{1, "age", 0, 5, 20},
	}

	for _, tt := range testsGet {
		got, err := ppb.Get(tt.idx, tt.col, tt.row, tt.seq)
		if err != nil {
			t.Fatal(err)
		}

		if got != tt.want {
			t.Errorf("got %v, want %v", got, tt.want)
		}
	}
}

func TestPlanParamBufferSetTable(t *testing.T) {
	ppb := newPlanParamBuffer(2, 4, 10, 5)

	names := []string{"n1", "n2", "n3"}
	data := [][]any{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}

	if err := ppb.SetTable(0, names, data, 5); err != nil {
		t.Fatal(err)
	}

	if d, dErr := ppb.GetColsData(0, "n1", 1); dErr != nil {
		t.Fatal(dErr)
	} else if !reflect.DeepEqual(d, []any{1, 4, 7}) {
		t.Errorf("got %v, want %v", d, []any{1, 4, 7})
	}
	if d, dErr := ppb.GetColsData(0, "n2", 1); dErr != nil {
		t.Fatal(dErr)
	} else if !reflect.DeepEqual(d, []any{2, 5, 8}) {
		t.Errorf("got %v, want %v", d, []any{2, 5, 8})
	}
	if d, dErr := ppb.GetColsData(0, "n3", 1); dErr != nil {
		t.Fatal(dErr)
	} else if !reflect.DeepEqual(d, []any{3, 6, 9}) {
		t.Errorf("got %v, want %v", d, []any{3, 6, 9})
	}
}

func TestPlanParamBufferSetOverwrite(t *testing.T) {
	ppb := newPlanParamBuffer(1, 4, 10, 5)

	if err := ppb.Set(0, "name", 0, "old", 100); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(0, "name", 0, "new", 10); err != nil {
		t.Fatal(err)
	}

	got, err := ppb.Get(0, "name", 0, 5)
	if err != nil {
		t.Fatal(err)
	}

	if got != "new" {
		t.Fatalf("got %v, want new", got)
	}
}

func TestPlanParamBufferGetRow(t *testing.T) {
	ppb := newPlanParamBuffer(2, 4, 10, 5)

	if err := ppb.Set(0, "name", 0, "old", 100); err != nil {
		t.Fatal(err)
	}

	dataSet := []any{
		"name", "name1", "name2", "name3",
	}

	dataSet1 := []any{
		1, 2, 3, 4, 5,
	}

	if err := ppb.SetColsData(0, "name", dataSet1, 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.SetColsData(1, "name", dataSet, 10); err != nil {
		t.Fatal(err)
	}

	got, gErr := ppb.GetColsData(1, "name", 1)
	if gErr != nil {
		t.Fatal(gErr)
	}

	if !reflect.DeepEqual(dataSet, got) {
		t.Fatalf("got %v, want %v", got, dataSet)
	}

}

func TestPlanParamBufferSequence(t *testing.T) {
	ppb := newPlanParamBuffer(1, 4, 10, 100)

	if err := ppb.Set(0, "name", 0, "value", 10); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		seq     int64
		wantErr bool
	}{
		{5, false},
		{9, false},
		{10, true},
		{11, true},
	}

	for _, tt := range tests {
		_, err := ppb.Get(0, "name", 0, tt.seq)

		if (err != nil) != tt.wantErr {
			t.Errorf("seq=%d wantErr=%v err=%v",
				tt.seq, tt.wantErr, err)
		}
	}
}

func TestPlanParamBufferMaxColSize(t *testing.T) {
	ppb := newPlanParamBuffer(1, 4, 10, 100)

	if err := ppb.Set(0, "a", 0, "a0", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(0, "a", 3, "a3", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(0, "b", 0, "b0", 5); err != nil {
		t.Fatal(err)
	}

	got, err := ppb.MaxColSize(0, 5)
	if err != nil {
		t.Fatal(err)
	}

	if got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

func TestPlanParamBufferMaxRowSize(t *testing.T) {
	ppb := newPlanParamBuffer(1, 4, 10, 100)

	if err := ppb.Set(0, "a", 0, "a0", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(0, "a", 5, "a5", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(0, "b", 8, "b8", 5); err != nil {
		t.Fatal(err)
	}

	got, err := ppb.MaxRowSize(0, 5)
	if err != nil {
		t.Fatal(err)
	}

	if got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}

func TestPlanParamBufferMaxSize(t *testing.T) {
	ppb := newPlanParamBuffer(2, 4, 10, 100)

	if err := ppb.Set(0, "a", 3, "a3", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(1, "b", 5, "b5", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(1, "b1", 5, "b5", 10); err != nil {
		t.Fatal(err)
	}

	if err := ppb.Set(1, "c", 8, "c8", 5); err != nil {
		t.Fatal(err)
	}

	cols, row, err := ppb.MaxSize(5)
	if err != nil {
		t.Fatal(err)
	}

	if cols != 2 {
		t.Fatalf("cols=%d, want 2", cols)
	}

	if row != 6 {
		t.Fatalf("row=%d, want 6", row)
	}
}

func TestPlanParamBufferInvalidIndex(t *testing.T) {
	ppb := newPlanParamBuffer(1, 4, 10, 5)

	if err := ppb.Set(-1, "a", 0, 1, 1); err == nil {
		t.Fatal("expected Set error")
	}

	if err := ppb.Set(1, "a", 0, 1, 1); err == nil {
		t.Fatal("expected Set error")
	}

	if _, err := ppb.Get(-1, "a", 0, 1); err == nil {
		t.Fatal("expected Get error")
	}

	if _, err := ppb.MaxColSize(1, 0); err == nil {
		t.Fatal("expected MaxColSize error")
	}

	if _, err := ppb.MaxRowSize(-1, 0); err == nil {
		t.Fatal("expected MaxRowSize error")
	}
}

func TestPlanParamBufferParallelSetGet(t *testing.T) {
	const (
		goroutines = 32
		iterations = 1000
	)

	ppb := newPlanParamBuffer(goroutines, 4, 10, 100000)

	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			col := fmt.Sprintf("col-%d", id)

			for j := 0; j < iterations; j++ {
				seq := int64(j + 100000)

				if err := ppb.Set(id, col, 0, j, seq); err != nil {
					t.Errorf("Set: %v", err)
					return
				}

				got, err := ppb.Get(id, col, 0, seq-1)
				if err != nil {
					t.Errorf("Get: %v", err)
					return
				}

				if got != j {
					t.Errorf("got %v, want %d", got, j)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestPlanParamBufferParallelMixedOperations(t *testing.T) {
	const (
		goroutines = 64
		iterations = 500
	)

	ppb := newPlanParamBuffer(8, 16, 100, 1000000)

	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			idx := id % 8
			col := fmt.Sprintf("col-%d", id%8)

			for j := 0; j < iterations; j++ {
				seq := int64(j + 100000)

				switch id % 4 {
				case 0:
					if err := ppb.Set(idx, col, j%100, j, seq); err != nil {
						t.Errorf("Set: %v", err)
						return
					}

				case 1:
					_, _ = ppb.Get(idx, col, 0, -1)

				case 2:
					_, _ = ppb.MaxColSize(idx, -1)

				case 3:
					_, _ = ppb.MaxRowSize(idx, -1)
					_, _, _ = ppb.MaxSize(-1)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestPlanParamBufferParallelSameColumn(t *testing.T) {
	const (
		goroutines = 32
		iterations = 500
	)

	ppb := newPlanParamBuffer(1, 4, 100, 1000000)

	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				seq := int64(id*iterations + j + 1)

				if err := ppb.Set(0, "shared", id%100, j, seq); err != nil {
					t.Errorf("Set: %v", err)
					return
				}

				_, _ = ppb.Get(0, "shared", id%100, -1)
				_, _, _ = ppb.MaxSize(-1)
			}
		}(i)
	}

	wg.Wait()
}
