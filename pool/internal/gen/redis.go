package gen

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/log"
	"oncecall/pool/internal/connection"
	"oncecall/pool/types"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type redisGenerator struct {
	info types.ConnConfig
	p    *redis.Client
}

func NewRedisGenerator(info types.ConnConfig, lExtension log.LoggerExtension[any]) (Generator, error) {
	db, convOk := strconv.Atoi(info.Name)
	if convOk != nil {
		return nil, errlist.ErrG.NewError(prefix.ParseError, "db is not number : %s", info.Name)
	}

	c := redis.NewClient(&redis.Options{
		Addr:       info.Server,
		Network:    "tcp",
		ClientName: "oncecall",
		Username:   info.Id,
		Password:   info.Password,
		DB:         db,
	})

	return &redisGenerator{
		info: info,
		p:    c,
	}, nil
}

func (r *redisGenerator) Gen(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
	return connection.NewRedisConn(r.p.Conn()), nil
}
