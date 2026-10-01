package pool

import (
	"oncecall/define"
	"oncecall/pool/types"
)

func GetConnPool(info *types.ConnConfig) (types.ConnPoolInterface, error) {
	switch info.DBType {
	case string(define.REDIS):
		return newRedisConnPool(info)
	case string(define.SSH):
		return newNonInteractiveSSHConnPool(info)
	case string(define.CASSANDRA):
		return newCassandraConnPool(info)
	case string(define.LOCAL):
		return newLocalConnPool(info)
	default:
		return newStandardConnPool(info)
	}
}
