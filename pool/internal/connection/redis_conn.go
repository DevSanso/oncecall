package connection

import (
	"context"
	"oncecall/errlist"
	"oncecall/pool/internal/utils"
	"oncecall/pool/types"
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
			return errlist.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
		}
		return nil
	}

	var loopRetErr error = nil

	if arg.IsTransaction {
		if ret := r.conn.Do(ctx, "MULTI"); ret.Err() != nil {
			return errlist.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
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
			return errlist.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
		}
	}

	if loopRetErr != nil {
		return errlist.ErrG.NewError(loopRetErr, "query:[%s]", trimQuery)
	}

	return nil
}

func (r *redisConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	trimQuery := strings.ReplaceAll(arg.Query, "\n", "")
	trimQuery = strings.ReplaceAll(trimQuery, "\r", "")
	if arg.Args == nil || len(arg.Args) <= 0 {
		ret := r.conn.Do(ctx, r.commonUtils.SplitRespectQuotesAny(trimQuery)...)

		if ret.Err() != nil {
			return nil, nil, errlist.ErrG.NewError(ret.Err(), "query:[%s]", trimQuery)
		}
		return nil, nil, nil
	}

	if arg.IsTransaction {
		return nil, nil, errlist.ErrG.NewError(nil, "ERROR: RunExecute exec(tran multi) not support")
	}

	param := arg.Args[0]
	realP := make([]any, 0, len(param)+1)
	realP = append(realP, r.commonUtils.SplitRespectQuotesAny(trimQuery)...)
	realP = append(realP, param...)

	loopRet := r.conn.Do(ctx, realP...)
	if (loopRet != nil) && loopRet.Err() != nil {
		return nil, nil, errlist.ErrG.NewError(loopRet.Err(), "query:[%s]", trimQuery)
	}

	buf := make([][]any, 0, 1)
	if err := r.parseOutputAny(loopRet.Val(), 0, buf); err != nil {
		return nil, nil, errlist.ErrG.NewError(err, "query:[%s]", trimQuery)
	}

	name = make([]string, len(buf))

	for idx := range buf {
		name[idx] = strconv.Itoa(idx + 1)
	}

	return buf, name, nil
}

func (r *redisConn) parseOutputAny(val any, idx int, m [][]any) error {
	var ret [][]any = m
	var current = idx

	if current > len(ret) {
		for start := len(ret); start <= current; start++ {
			ret = append(ret, make([]any, 0))
		}
	}

	switch val.(type) {
	case int64:
		temp := append(ret[current], val.(int64))
		ret[current] = temp
	case float64:
		temp := append(ret[current], val.(float64))
		ret[current] = temp
	case string:
		temp := append(ret[current], val.(string))
		ret[current] = temp
	case []interface{}:
		var err error

		for _, v := range val.([]interface{}) {
			current += 1
			if err = r.parseOutputAny(v, current, m); err != nil {
				break
			}
		}
		if err != nil {
			return err
		}
	default:
		return errlist.ErrG.NewError(nil, "Parse not support %s", reflect.ValueOf(val).Type().Name())
	}

	return nil
}

func (r *redisConn) parseOutputStr(val interface{}, idx int, m [][]string) error {
	var ret [][]string = m
	var current = idx

	if current > len(ret) {
		for start := len(ret); start <= current; start++ {
			ret = append(ret, make([]string, 0, 10))
		}
	}

	switch val.(type) {
	case int64:
		temp := append(ret[current], strconv.FormatInt(val.(int64), 10))
		ret[current] = temp
	case float64:
		temp := append(ret[current], strconv.FormatFloat(val.(float64), 'f', 2, 64))
		ret[current] = temp
	case string:
		temp := append(ret[current], val.(string))
		ret[current] = temp
	case []interface{}:
		var err error
		for _, v := range val.([]interface{}) {
			current += 1
			if err = r.parseOutputStr(v, current, m); err != nil {
				break
			}
		}
		if err != nil {
			return err
		}
	default:
		return errlist.ErrG.NewError(nil, "parseOutputStr not support %s", reflect.ValueOf(val).Type().Name())
	}

	return nil
}

func (r *redisConn) Close() error {
	return r.conn.Close()
}
