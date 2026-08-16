package conn

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"

	"oncecall/cfg"
	"oncecall/define"
	"oncecall/utils"

	"github.com/redis/go-redis/v9"
)

type redisConnPool struct {
	conn *redis.Client
	name string

	conf *cfg.ConnConfig
}

func newRedisConnPool(info *cfg.ConnConfig) (ConnPoolInterface, error) {
	if info.DBType != string(define.REDIS) {
		return nil, utils.ErrorfPc("NewRedisConnPool[name:%s] - not support redis dbtype(%s)", info.Name, info.DBType)
	}

	_, convertErr := strconv.Atoi(info.Name)
	if convertErr != nil {
		return nil, utils.ErrorfPc("NewRedisConnPool[name:%s] - %s", info.Name, convertErr.Error())
	}

	_, url, err := getConnUrlAndDriver(info)
	if err != nil {
		return nil, utils.ErrorfPc("%s", err)
	}

	opt, optErr := redis.ParseURL(url)
	if optErr != nil {

		return nil, utils.ErrorfPc("NewRedisConnPool - [name:%s] parsing url error", info.Name)
	}

	opt.MaxActiveConns = info.MaxConn
	opt.PoolSize = info.MaxConn
	opt.ConnMaxIdleTime = time.Second * 60
	client := redis.NewClient(opt)

	return &redisConnPool{
		conn: client,
		name: info.Name,
		conf: info,
	}, nil
}

func (r *redisConnPool) splitRespectQuotes(s string) []any {
	var result []any
	var current []rune

	inDouble := false
	inSingle := false

	for _, r := range s {
		switch r {
		case '"':
			if !inSingle {
				inDouble = !inDouble
				continue
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				continue
			}
		case ' ':
			if !inDouble && !inSingle {
				if len(current) > 0 {
					result = append(result, string(current))
					current = nil
				}
				continue
			}
		}
		current = append(current, r)
	}

	if len(current) > 0 {
		result = append(result, string(current))
	}

	return result
}
func (r *redisConnPool) GetConfig() cfg.ConnConfig {
	return *r.conf
}
func (r *redisConnPool) RunExecute(ctx context.Context, arg *Args) error {
	trimQuery := strings.ReplaceAll(arg.Query, "\n", "")
	trimQuery = strings.ReplaceAll(trimQuery, "\r", "")
	if arg.Args == nil || len(arg.Args) <= 0 {
		ret := r.conn.Do(ctx, r.splitRespectQuotes(trimQuery)...)

		if ret.Err() != nil {
			return utils.ErrorfPc("[err:%s] query:[%s]", ret.Err(), trimQuery)
		}
		return nil
	}

	var loopRetErr error = nil

	if arg.IsTranscation {
		if ret := r.conn.Do(ctx, "MULTI"); ret.Err() != nil {
			return utils.ErrorfPc("[err:%s] query:[%s]", ret.Err(), trimQuery)
		}
	}

	for _, param := range arg.Args {
		realP := make([]any, 0, len(param)+1)
		realP = append(realP, r.splitRespectQuotes(trimQuery)...)
		realP = append(realP, param...)

		loopRet := r.conn.Do(ctx, realP...)
		if (loopRet != nil) && loopRet.Err() != nil {
			loopRetErr = loopRet.Err()
			break
		}
	}

	if arg.IsTranscation {
		if ret := r.conn.Do(ctx, "EXEC"); ret.Err() != nil {
			return utils.ErrorfPc("[err:%s] query:[%s]", ret.Err(), trimQuery)
		}
	}

	if loopRetErr != nil {
		return utils.ErrorfPc("[err:%s] query:[%s]", loopRetErr.Error(), trimQuery)
	}

	return nil
}

func (r *redisConnPool) RunQuery(ctx context.Context, arg *Args) ([][]any, error) {
	trimQuery := strings.ReplaceAll(arg.Query, "\n", "")
	trimQuery = strings.ReplaceAll(trimQuery, "\r", "")
	if arg.Args == nil || len(arg.Args) <= 0 {
		ret := r.conn.Do(ctx, r.splitRespectQuotes(trimQuery)...)

		if ret.Err() != nil {
			return nil, utils.ErrorfPc("[err:%s] query:[%s]", ret.Err(), trimQuery)
		}
		return nil, nil
	}

	if arg.IsTranscation {
		return nil, utils.ErrorfPc("ERROR: [name:%s]  RunExecute exec(tran multi) not support", r.name)
	}

	param := arg.Args[len(arg.Args)-1]
	realP := make([]any, 0, len(param)+1)
	realP = append(realP, r.splitRespectQuotes(trimQuery)...)
	realP = append(realP, param...)

	loopRet := r.conn.Do(ctx, realP...)
	if (loopRet != nil) && loopRet.Err() != nil {
		return nil, utils.ErrorfPc("[err:%s] query:[%s]", loopRet.Err(), trimQuery)
	}

	buf := make([][]any, 1)
	if err := r.parseOutputAny(loopRet.Val(), 0, buf); err != nil {
		return nil, utils.ErrorfPc("[err:%s] query:[%s]", err.Error(), trimQuery)
	}

	return buf, nil
}

func (r *redisConnPool) parseOutputAny(val interface{}, idx int, m [][]any) error {
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
		return utils.ErrorfPc("redisConnPool - Parse not support %s", reflect.ValueOf(val).Type().Name())
	}

	return nil
}

func (r *redisConnPool) parseOutputStr(val interface{}, idx int, m [][]string) error {
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
		return utils.ErrorfPc("redisConnPool - parseOutputStr not support %s", reflect.ValueOf(val).Type().Name())
	}

	return nil
}

func (r *redisConnPool) Close() error {
	return r.conn.Close()
}
