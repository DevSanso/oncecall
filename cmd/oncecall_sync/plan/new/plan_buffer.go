package new

import (
	"fmt"
	"slices"
	"sync"
)

type planParamBuffer interface {
	Set(idx int, col string, row int, data any, seq int64) error
	SetColsData(idx int, col string, data []any, seq int64) error
	SetTable(idx int, cols []string, data [][]any, seq int64) error

	Get(idx int, col string, row int, seq int64) (any, error)
	GetColsData(idx int, col string, seq int64) ([]any, error)

	MaxColSize(idx int, seq int64) (int, error)
	MaxRowSize(idx int, seq int64) (int, error)
	MaxSize(seq int64) (cols, rows int, err error)
}

type planParamValue struct {
	row     []any
	rowSize int
	seq     int64
}

type planParamBufferImpl struct {
	mu    sync.RWMutex
	table []map[string]planParamValue

	rowCap     int
	rowIdleSeq int
}

func newPlanParamBuffer(
	count, mapInitCap, rowInitCap, rowIdleSeq int,
) planParamBuffer {
	if count < 0 {
		count = 1
	}
	if mapInitCap < 0 {
		mapInitCap = 0
	}
	if rowInitCap < 0 {
		rowInitCap = 0
	}

	ppb := &planParamBufferImpl{
		table:      make([]map[string]planParamValue, count+1),
		rowCap:     rowInitCap,
		rowIdleSeq: rowIdleSeq,
	}

	for i := range ppb.table {
		ppb.table[i] = make(map[string]planParamValue, mapInitCap)
	}

	return ppb
}

func (ppb *planParamBufferImpl) Set(
	idx int,
	col string,
	row int,
	data any,
	seq int64,
) error {
	ppb.mu.Lock()
	defer ppb.mu.Unlock()

	if err := ppb.validateIndex(idx); err != nil {
		return err
	}
	if row < 0 {
		return fmt.Errorf("invalid row: %d", row)
	}

	ppb.deleteIdleLocked(seq)

	v := ppb.table[idx][col]

	if len(v.row) <= row {
		newRow := make([]any, row+1+ppb.rowCap)
		copy(newRow, v.row)
		v.row = newRow
	}

	v.row[row] = data
	v.rowSize = calculateRowSize(v.row)
	v.seq = seq

	ppb.table[idx][col] = v

	return nil
}

func (ppb *planParamBufferImpl) SetColsData(
	idx int,
	col string,
	data []any,
	seq int64,
) error {
	ppb.mu.Lock()
	defer ppb.mu.Unlock()

	if err := ppb.validateIndex(idx); err != nil {
		return err
	}

	ppb.deleteIdleLocked(seq)

	row := make([]any, len(data)+ppb.rowCap)
	copy(row, data)

	ppb.table[idx][col] = planParamValue{
		row:     row,
		rowSize: calculateRowSize(data),
		seq:     seq,
	}

	return nil
}

func (ppb *planParamBufferImpl) SetTable(
	idx int,
	cols []string,
	data [][]any,
	seq int64,
) error {
	ppb.mu.Lock()
	defer ppb.mu.Unlock()

	if err := ppb.validateIndex(idx); err != nil {
		return err
	}

	ppb.deleteIdleLocked(seq)

	for colIdx, col := range cols {
		row := make([]any, len(data)+ppb.rowCap)

		for rowIdx, values := range data {
			if colIdx < len(values) {
				row[rowIdx] = values[colIdx]
			}
		}

		ppb.table[idx][col] = planParamValue{
			row:     row,
			rowSize: calculateRowSize(row[:len(data)]),
			seq:     seq,
		}
	}

	return nil
}

func (ppb *planParamBufferImpl) Get(
	idx int,
	col string,
	row int,
	seq int64,
) (any, error) {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	if err := ppb.validateIndex(idx); err != nil {
		return nil, err
	}

	v, ok := ppb.table[idx][col]
	if !ok {
		return nil, fmt.Errorf("column not found: %s", col)
	}

	if v.seq <= seq {
		return nil, nil
	}

	if row < 0 || row >= len(v.row) {
		return nil, fmt.Errorf("invalid row: %d", row)
	}

	return v.row[row], nil
}

func (ppb *planParamBufferImpl) GetColsData(
	idx int,
	col string,
	seq int64,
) ([]any, error) {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	if err := ppb.validateIndex(idx); err != nil {
		return nil, err
	}

	v, ok := ppb.table[idx][col]
	if !ok {
		return nil, fmt.Errorf("column not found: %s", col)
	}

	if v.seq <= seq {
		return nil, nil
	}

	ret := make([]any, v.rowSize)
	copy(ret, v.row[:v.rowSize])

	return ret, nil
}

func (ppb *planParamBufferImpl) MaxColSize(
	idx int,
	seq int64,
) (int, error) {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	if err := ppb.validateIndex(idx); err != nil {
		return 0, err
	}

	return ppb.maxColSizeLocked(idx, seq), nil
}

func (ppb *planParamBufferImpl) MaxRowSize(
	idx int,
	seq int64,
) (int, error) {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	if err := ppb.validateIndex(idx); err != nil {
		return 0, err
	}

	return ppb.maxRowSizeLocked(idx, seq), nil
}

func (ppb *planParamBufferImpl) MaxSize(
	seq int64,
) (cols, rows int, err error) {
	ppb.mu.RLock()
	defer ppb.mu.RUnlock()

	for idx := range ppb.table {
		if c := ppb.maxColSizeLocked(idx, seq); c > cols {
			cols = c
		}

		if r := ppb.maxRowSizeLocked(idx, seq); r > rows {
			rows = r
		}
	}

	return cols, rows, nil
}

func (ppb *planParamBufferImpl) deleteIdleLocked(seq int64) {
	if ppb.rowIdleSeq <= 0 {
		return
	}

	for idx := range ppb.table {
		for col, v := range ppb.table[idx] {
			if seq > v.seq &&
				seq-v.seq > int64(ppb.rowIdleSeq) {
				delete(ppb.table[idx], col)
			}
		}
	}
}

func (ppb *planParamBufferImpl) maxColSizeLocked(
	idx int,
	seq int64,
) int {
	counts := make(map[int64]int)
	max := 0

	for _, v := range ppb.table[idx] {
		if v.seq <= seq {
			continue
		}

		counts[v.seq]++
		if counts[v.seq] > max {
			max = counts[v.seq]
		}
	}

	return max
}

func (ppb *planParamBufferImpl) maxRowSizeLocked(
	idx int,
	seq int64,
) int {
	max := 0

	for _, v := range ppb.table[idx] {
		if v.seq > seq && v.rowSize > max {
			max = v.rowSize
		}
	}

	return max
}

func (ppb *planParamBufferImpl) validateIndex(idx int) error {
	if idx < 0 || idx >= len(ppb.table) {
		return fmt.Errorf("invalid index: %d", idx)
	}
	return nil
}

func calculateRowSize(row []any) int {
	for i, r := range slices.Backward(row) {
		if r != nil {
			return i + 1
		}
	}
	return 0
}
