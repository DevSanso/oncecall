package conn

import (
	"context"
	"oncecall/define"
)

type Args struct {
	Query         string
	Args          [][]any
	IsTransaction bool
}

type ConnConfig struct {
	DBType string `toml:"db_type"`
	Name   string `toml:"name"`

	Server   string `toml:"server"`
	Id       string `toml:"id"`
	Password string `toml:"password"`

	MaxConn   int            `toml:"max_conn"`
	OptionMap map[string]any `toml:"option"`
}

type ConnPoolInterface interface {
	RunExecute(ctx context.Context, arg *Args) error
	RunQuery(ctx context.Context, arg *Args) (rows [][]any, name []string, err error)
	GetConfig() ConnConfig
	Close() error
}

func GetConnPool(info *ConnConfig) (ConnPoolInterface, error) {
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
