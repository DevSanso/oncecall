package conn

import (
	"context"
	"oncecall/cfg"
	"oncecall/define"
)

type Args struct {
	Query         string
	Args          [][]any
	IsTransaction bool
}

type ConnPoolInterface interface {
	RunExecute(ctx context.Context, arg *Args) error
	RunQuery(ctx context.Context, arg *Args) ([][]any, error)
	GetConfig() cfg.ConnConfig
	Close() error
}

func GetConnPool(info *cfg.ConnConfig) (ConnPoolInterface, error) {
	switch info.DBType {
	case string(define.REDIS):
		return newRedisConnPool(info)
	case string(define.SSH):
		return newSSHConnPool(info)
	case string(define.CASSANDRA):
		return newCassandraConnPool(info)
	default:
		return newStandardConnPool(info)
	}
}
