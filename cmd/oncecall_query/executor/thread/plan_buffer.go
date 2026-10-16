package thread

import (
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"os"
	"time"
)

type planParamBuffer struct {
	m []map[string][]struct {
		data any
		seq  int64
	}
	rowCap     int
	rowIdleSec int
}

func newPlanParamBuffer(count int, mapInitSize int, rowInitCap int, rowIdleSec int) planParamBuffer {
	obj := make([]map[string][]struct {
		data any
		seq  int64
	}, count)
	for idx := range obj {
		obj[idx] = make(map[string][]struct {
			data any
			seq  int64
		}, mapInitSize)
	}

	return planParamBuffer{
		m:          obj,
		rowCap:     rowInitCap,
		rowIdleSec: rowIdleSec,
	}
}

func (ppb *planParamBuffer) get(idx int, col string, row int, seq int64) (any, error) {
	if len(ppb.m) <= idx {
		return nil, nil
	}

	planMap := ppb.m[idx]
	if len(planMap[col]) <= row {
		return nil, errlist.ErrG.NewError(prefix.OutLenError, "plan len :%d, search idx : %d", len(planMap[col]), row)
	}

	if planMap[col][row].seq > seq {
		return nil, nil
	}

	return planMap[col][row].data, nil
}

func (ppb *planParamBuffer) set(idx int, col string, row int, data any, seq int64) error {
	if len(ppb.m) <= idx {
		return errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]

	if len(planMap[col]) <= row {
		temp := make([]struct {
			data any
			seq  int64
		}, len(planMap[col])+ppb.rowCap)
		copy(temp, planMap[col])
		planMap[col] = temp
	} else {
		idle := time.Duration(ppb.rowIdleSec)
		if idle <= 0 {
			idle = 60
		}

		if seq-planMap[col][len(planMap[col])-1].seq > int64(time.Second*idle) {
			temp := make([]struct {
				data any
				seq  int64
			}, row+ppb.rowCap)
			copy(temp, planMap[col])
			planMap[col] = temp
		}
	}

	planMap[col][row] = struct {
		data any
		seq  int64
	}{data: data, seq: seq}

	return nil
}

func (ppb *planParamBuffer) maxColSize(idx int, seq int64) (int, error) {
	if len(ppb.m) <= idx {
		return -1, errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]
	return len(planMap), nil
}

func (ppb *planParamBuffer) maxRowSize(idx int, seq int64) (int, error) {
	if len(ppb.m) <= idx {
		return -1, errlist.ErrG.NewError(os.ErrInvalid, "overflow %d <= %d", len(ppb.m), idx)
	}

	planMap := ppb.m[idx]

	cnt := 0
	for _, val := range planMap {
		cnt = max(cnt, len(val))
	}

	return cnt, nil
}

func (ppb *planParamBuffer) maxSize(seq int64) (cols int, row int, err error) {
	maxC := 0
	maxR := 0

	for idx := range ppb.m {
		c, cerr := ppb.maxColSize(idx, seq)
		if cerr != nil {
			return -1, -1, errlist.ErrG.NewError(cerr, "")
		}
		r, rerr := ppb.maxRowSize(idx, seq)
		if rerr != nil {
			return -1, -1, errlist.ErrG.NewError(rerr, "")
		}

		maxC = max(maxC, c)
		maxR = max(maxR, r)
	}

	return maxC, maxR, nil
}
