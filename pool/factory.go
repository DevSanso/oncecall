package pool

import (
	"context"
	"oncecall/define"
	"oncecall/errlist"
	"oncecall/extension/log"
	"oncecall/pool/types"
)

type wrapConnPoolInterface struct {
	realP types.ConnPoolInterface
	logger log.LoggerExtension[any]
}

func (w wrapConnPoolInterface) RunExecute(ctx context.Context, arg *types.Args) error {

}

func (w wrapConnPoolInterface) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {

}

func (w wrapConnPoolInterface) GetConfig() types.ConnConfig {

}

func (w wrapConnPoolInterface) Close() error {

}

var _ types.ConnPoolInterface = (*wrapConnPoolInterface)(nil)

func GetConnPool(info *types.ConnConfig) (types.ConnPoolInterface, error) {
	var p types.ConnPoolInterface
	var err error

	switch info.DBType {
	case string(define.REDIS):
		p, err = newRedisConnPool(info)
	case string(define.SSH):
		p, err = newNonInteractiveSSHConnPool(info)
	case string(define.CASSANDRA):

		p, err =  newCassandraConnPool(info)
	case string(define.LOCAL):

		p, err =  newLocalConnPool(info)
	case string(define.KAFKA):

		p, err =  newKafkaConnPool(info)
	default:

		p, err =  newStandardConnPool(info)
	}

	if err != nil {
		return nil, errlist.ErrG.NewError(err, "NewConnPool Failed")
	}


	return &wrapConnPoolInterface{
		realP: p,
	}, nil
}
