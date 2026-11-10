package plan

import (
	"context"
	"fmt"
	"oncecall/connpool"
	"oncecall/connpool/types"
	"oncecall/define"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"strings"
)

type sqlCacheMap struct {
	sqlDB     types.ConnPoolCloser
	colLenMap map[int]int
}

func newSQLCacheMap(ctx context.Context, metricExt metric.MetricExtension, planLogger log.LoggerLogExtension[any]) (sqlCacheMap, error) {
	dbConn, connErr := connpool.NewConnPool(ctx, "cache", types.ConnConfig{DBType: string(define.DUCKDB), Name: ":memory:", MaxConn: 1}, metricExt, planLogger, planLogger, planLogger)
	if connErr != nil {
		return sqlCacheMap{}, connErr
	}

	return sqlCacheMap{dbConn, make(map[int]int, 3)}, nil
}

func (p *sqlCacheMap) CreateCacheTable(planIdx int, names []string, colTy []types.ConnPoolColType) error {
	buf := strings.Builder{}
	typeName := ""

	if _, exists := p.colLenMap[planIdx]; exists {
		return nil
	}

	namesMax := len(names)
	for idx := range namesMax {
		switch colTy[idx] {
		case types.ConnPoolColTypeBigInt:
			typeName = "INT8"
		case types.ConnPoolColTypeInt:
			typeName = "INT"
		case types.ConnPoolColTypeBool:
			typeName = "BOOL"
		case types.ConnPoolColTypeBytes:
			typeName = "BLOB"
		case types.ConnPoolColTypeText:
			typeName = "TEXT"
		case types.ConnPoolColTypeFloat32:
			typeName = "FLOAT4"
		case types.ConnPoolColTypeFloat64:
			typeName = "FLOAT8"
		default:
			return nil
		}
		buf.WriteString(fmt.Sprintf("%s %s", names[idx], typeName))
		if idx < namesMax-1 {
			buf.WriteString(", ")
		}
	}

	createSql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS plan%d ( %s )", planIdx, buf.String())

	if err := p.sqlDB.RunExecute(context.Background(), &types.Args{
		Query:         createSql,
		Args:          nil,
		IsTransaction: false,
	}); err != nil {
		return err
	}
	p.colLenMap[planIdx] = namesMax
	return nil
}

func (p *sqlCacheMap) InsertPlanData(planIdx int, data [][]any) error {
	if len(data) <= 0 {
		return nil
	}

	planLen, exists := p.colLenMap[planIdx]
	if !exists {
		return nil
	}
	buf := strings.Builder{}

	for idx := range planLen {
		buf.WriteRune('?')
		if idx < planLen-1 {
			buf.WriteRune(',')
		}
	}

	insertSQL := fmt.Sprintf("INSERT INTO plan%d values(%s)", buf.String())
	if err := p.sqlDB.RunExecute(context.Background(), &types.Args{
		Query:         insertSQL,
		Args:          data,
		IsTransaction: true,
	}); err != nil {
		return err
	}

	return nil
}

func (p *sqlCacheMap) Close() error {
	return p.sqlDB.Close()
}

func (p *sqlCacheMap) TruncCacheTables() error {
	for k, _ := range p.colLenMap {
		err := p.sqlDB.RunExecute(context.Background(), &types.Args{
			Query:         fmt.Sprintf("TRUNCATE TABLE IF EXISTS %d", k),
			Args:          nil,
			IsTransaction: false,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *sqlCacheMap) Select(ctx context.Context, query string, isTran bool) ([][]any, error) {
	data, _, _, dataErr := p.sqlDB.RunQuery(ctx, &types.Args{
		Query:         query,
		Args:          nil,
		IsTransaction: isTran,
	})

	return data, dataErr
}
