package connection

import (
	"context"
	"oncecall/connpool/internal/utils"
	"oncecall/connpool/types"
	"oncecall/errset"
	"reflect"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type redisConn struct {
	commonUtils utils.CommonUtils
	conn        *redis.Conn
}

func NewRedisConn(conn *redis.Conn) types.Conn {
	return &redisConn{conn: conn}
}

func (r *redisConn) RunExecute(ctx context.Context, arg *types.Args) error {
	trimQuery := strings.ReplaceAll(arg.Query, "\n", "")
	trimQuery = strings.ReplaceAll(trimQuery, "\r", "")
	if arg.Args == nil || len(arg.Args) <= 0 {
		ret := r.conn.Do(ctx, r.commonUtils.SplitRespectQuotesAny(trimQuery)...)

		if ret.Err() != nil {
			return errset.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
		}
		return nil
	}

	var loopRetErr error = nil

	if arg.IsTransaction {
		if ret := r.conn.Do(ctx, "MULTI"); ret.Err() != nil {
			return errset.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
		}
	}

	for _, param := range arg.Args {
		realP := make([]any, 0, len(param)+1)
		realP = append(realP, r.commonUtils.SplitRespectQuotesAny(trimQuery)...)
		realP = append(realP, param...)

		loopRet := r.conn.Do(ctx, realP...)
		if (loopRet != nil) && loopRet.Err() != nil {
			loopRetErr = loopRet.Err()
			break
		}
	}

	if arg.IsTransaction {
		if ret := r.conn.Do(ctx, "EXEC"); ret.Err() != nil {
			return errset.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
		}
	}

	if loopRetErr != nil {
		return errset.ErrG.NewError(loopRetErr, "query:[%s]", trimQuery)
	}

	return nil
}

func (r *redisConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, ty []types.ConnPoolColType, err error) {
	trimQuery := strings.ReplaceAll(arg.Query, "\n", "")
	trimQuery = strings.ReplaceAll(trimQuery, "\r", "")

	if arg.IsTransaction {
		return nil, nil, nil, errset.ErrG.NewError(nil, "ERROR: RunExecute exec(tran multi) not support")
	}

	var doRet *redis.Cmd
	if arg.Args == nil || len(arg.Args) <= 0 {
		doRet = r.conn.Do(ctx, r.commonUtils.SplitRespectQuotesAny(trimQuery)...)

	} else {
		param := arg.Args[0]
		realP := make([]any, 0, len(param)+1)
		realP = append(realP, r.commonUtils.SplitRespectQuotesAny(trimQuery)...)
		realP = append(realP, param...)

		doRet = r.conn.Do(ctx, realP...)
	}
	if (doRet != nil) && doRet.Err() != nil {
		return nil, nil, nil, errset.ErrG.NewError(doRet.Err(), "query:[%s]", trimQuery)
	}

	var buf [][]any
	if buf, ty, err = r.parseOutputAny(doRet.Val()); err != nil {
		return nil, nil, nil, errset.ErrG.NewError(err, "query:[%s]", trimQuery)
	}

	name = make([]string, len(buf))

	for idx := range buf {
		name[idx] = strconv.Itoa(idx + 1)
	}

	return buf, name, ty, nil
}

func (r *redisConn) parseOutputAny(val any) ([][]any, []types.ConnPoolColType, error) {
	var ret [][]any = make([][]any, 1)
	var ty []types.ConnPoolColType = nil

	switch v := val.(type) {
	case []any:
		ret[0] = make([]any, len(v))
		ty = make([]types.ConnPoolColType, len(v))

		for idx, data := range v {
			ret[0][idx] = data
			ty[idx] = types.ConnPoolColTypeText
		}
	case int64:
		ret[0] = []any{v}
		ty = make([]types.ConnPoolColType, 1)
		ty[0] = types.ConnPoolColTypeBigInt
	case float64:
		ret[0] = []any{v}
		ty = make([]types.ConnPoolColType, 1)
		ty[0] = types.ConnPoolColTypeFloat64
	case string:
		ret[0] = []any{v}
		ty = make([]types.ConnPoolColType, 1)
		ty[0] = types.ConnPoolColTypeText
	case bool:
		ret[0] = []any{v}
		ty = make([]types.ConnPoolColType, 1)
		ty[0] = types.ConnPoolColTypeBool
	default:
		return nil, nil, errset.ErrG.NewError(nil, "Parse not support %s", reflect.ValueOf(val).Type().Name())
	}

	return ret, ty, nil
}

func (r *redisConn) Close() error {
	return r.conn.Close()
}
