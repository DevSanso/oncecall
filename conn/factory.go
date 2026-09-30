package conn

import (
	"oncecall/conn/types"
	"oncecall/define"
)

func GetConnPool(info *types.ConnConfig) (types.ConnPoolInterface, error) {
	switch info.DBType {
	case string(define.REDIS):
		return newRedisConnPool(info)
	case string(define.SSH):
		return newNonInteractiveSSHConnPool(info)
	case string(define.CASSANDRA):
		return newCassandraConnPool(info)
	default:
		return newStandardConnPool(info)
	}
}
