package pool

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/internal/connection"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"sync/atomic"
	"time"
)


type kafkaConnPool struct {
	conf *types.ConnConfig

	consumerMap  *generic.SyncMap[string, generic.Pair[int64, *connection.SyncKafkaClient]]
	producerPool *generic.SyncUsePool[*connection.SyncKafkaClient, struct{}]

	isClose atomic.Bool
}

func (k *kafkaConnPool) Alloc() (all int, used int, idle int) {
	k.consumerMap.
}

func newKafkaConnPool(conf *types.ConnConfig) (privateConnPool, error) {
	o := &kafkaConnPool{}
	o.conf = conf
	o.producerPool = generic.NewGenericSyncUsePool[*connection.SyncKafkaClient, struct{}](func() (*connection.SyncKafkaClient, error) {
		c, cErr := connection.NewSyncKafkaClient(o.conf.Server, false)
		if cErr != nil {
			return nil, errlist.ErrG.NewError(cErr, "")
		}
		return c, nil
	})
	o.consumerMap = generic.NewGenericSyncMap[string, generic.Pair[int64, *connection.SyncKafkaClient]]()
	o.isClose = atomic.Bool{}
	return o, nil
}

func (*kafkaConnPool) getArg(arg *types.Args) (count int, readTimeoutMs int, err error) {
	if len(arg.Args) <= 0 || len(arg.Args[0]) < 2 {
		return -1, -1, errlist.ErrG.NewError(prefix.SentinelCatchError, "kafka need count args")
	}

	var convOk bool
	count, convOk = arg.Args[0][0].(int)
	if !convOk {
		return -1, -1, errlist.ErrG.NewError(prefix.NotMatchingError, "kafka convert failed count %v", arg.Args[0][0])
	}

	readTimeoutMs, convOk = arg.Args[0][1].(int)
	if !convOk {
		return -1, -1, errlist.ErrG.NewError(prefix.NotMatchingError, "kafka convert failed readTimeoutMs %v", arg.Args[0][0])
	}

	return
}

func (k *kafkaConnPool) RunExecute(ctx context.Context, arg *types.Args) error {
	if k.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "kafka connection pool is closed")
	}
	if arg.Query == "" {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "topic is empty")
	}
	_, err := k.producerPool.Use(func(data *connection.SyncKafkaClient) (struct{}, error) {
		if writeErr := data.WriteData(k.conf.Name, arg);writeErr != nil {
			return struct{}{}, errlist.ErrG.NewError(writeErr, "write error")
		}

		return struct{}{}, nil
	})

	return err
}

func (k *kafkaConnPool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	if k.isClose.Load() {
		return nil, nil, errlist.ErrG.NewError(prefix.ClosedError, "kafka connection pool is closed")
	}
	if arg.Query == "" {
		return nil, nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "topic is empty")
	}

	readConn, ok := k.consumerMap.Load(arg.Query)
	if !ok {
		newConn, newErr := connection.NewSyncKafkaClient(k.conf.Server, true)
		if newErr != nil {
			return nil, nil, errlist.ErrG.NewError(newErr, "create consumer failed %s", k.conf.Server)
		}
		k.consumerMap.Store(arg.Query, generic.Pair[int64, *connection.SyncKafkaClient]{First: -2, Second: newConn})
	}

	count, timeoutMs, argErr := k.getArg(arg)
	if argErr != nil {
		return nil, nil, errlist.ErrG.NewError(argErr, "get args failed")
	}

	readCtx, cancelFn := context.WithTimeout(ctx, time.Duration(timeoutMs) * time.Millisecond)
	defer cancelFn()

	rows, name, readConn.First, err = readConn.Second.ReadData(readCtx, k.conf.Name, count, readConn.First)
	if err != nil {
		
		return nil, nil, errlist.ErrG.NewError(err, "read failed")
	}
	return
}

func (k *kafkaConnPool) GetConfig() types.ConnConfig {
	return *k.conf
}

func (k *kafkaConnPool) Close() error {
	k.isClose.Store(true)
	_ = k.producerPool.Close()

	k.consumerMap.Range(func(key string, value generic.Pair[int64, *connection.SyncKafkaClient]) bool {
		_ = value.Second.Close()
		return true
	})

	return nil
}

var _ privateConnPool = (*kafkaConnPool)(nil)
