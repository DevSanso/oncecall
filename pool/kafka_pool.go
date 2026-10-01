package pool

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type syncKafkaReadClient generic.Pair[*kafka.Consumer, sync.Mutex]

func (s *syncKafkaReadClient) Close() error {
	s.Second.Lock()
	defer s.Second.Unlock()

	return s.First.Close()
}
func (s *syncKafkaReadClient) Read(count int, limitPoolMs int) ([][]any, []string, error) {
	s.Second.Lock()
	defer s.Second.Unlock()

	data := make([][]any, 4)

	data[0] = make([]any, 0, count/10)
	data[1] = make([]any, 0, count/10)
	data[2] = make([]any, 0, count/10)
	data[3] = make([]any, 0, count/10)
	timestamp := data[0]
	topic := data[1]
	partition := data[2]
	value := data[3]

	for range count {
		ev := s.First.Poll(limitPoolMs)
		if ev == nil {
			continue
		}

		switch e := ev.(type) {
		case *kafka.Message:
			timestamp = append(timestamp, e.Timestamp.UnixMilli())
			topic = append(topic, e.TopicPartition.Topic)
			partition = append(partition, e.TopicPartition.Partition)
			value = append(value, e.Value)
		case kafka.Error:
			return nil, nil, errlist.ErrG.NewError(e, "")
		}
	}

	return data, []string{
		"timestamp",
		"topic",
		"partition",
		"value",
	}, nil
}

type kafkaConnPool struct {
	conf *types.ConnConfig

	kafkaConf *kafka.ConfigMap

	consumerMap  *generic.SyncMap[string, *syncKafkaReadClient]
	producerPool *generic.SyncPool[*generic.Pair[*kafka.Producer, error]]

	isClose atomic.Bool
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

func (*kafkaConnPool) splitTopicFromQuery(query string) []string {
	return strings.Split(query, ",")
}

func (*kafkaConnPool) doProducer(ctx context.Context, producer *kafka.Producer, topic string, arg *types.Args) error {
	for idx, data := range arg.Args {
		if len(data) < 1 {
			return errlist.ErrG.NewError(prefix.SentinelCatchError, "data is empty %d", idx)
		}

		if data[0] == nil {
			continue
		}
		var messageValue []byte

		switch data[0].(type) {
		case string:
			messageValue = []byte(data[0].(string))
		case []byte:
			messageValue = data[0].([]byte)
		default:
			return errlist.ErrG.NewError(prefix.NotMatchingError, "convert failed data (only support string, []byte)%v", data)
		}

		if err := producer.Produce(&kafka.Message{
			Value: messageValue,
			TopicPartition: kafka.TopicPartition{
				Topic:     &topic,
				Partition: kafka.PartitionAny,
			},
		}, nil); err != nil {
			return errlist.ErrG.NewError(err, "produce failed")
		}
	}
	return nil
}

func (k *kafkaConnPool) RunExecute(ctx context.Context, arg *types.Args) error {
	if k.isClose.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "kafka connection pool is closed")
	}
	if arg.Query == "" {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "topic is empty")
	}
	topic := arg.Query
	producerPair := k.producerPool.Get()

	if producerPair.Second != nil {
		return errlist.ErrG.NewError(producerPair.Second, "get failed producer %s", k.conf.Server)
	}

	producer := producerPair.First
	if arg.IsTransaction {
		if err := producer.BeginTransaction(); err != nil {
			producer.Close()
			return errlist.ErrG.NewError(err, "begin transaction failed")
		}

		if err := k.doProducer(ctx, producer, topic, arg); err != nil {
			_ = producer.AbortTransaction(ctx)
			producer.Close()
			return errlist.ErrG.NewError(err, "do producer failed")
		}

		if err := producer.CommitTransaction(ctx); err != nil {
			_ = producer.AbortTransaction(ctx)
			producer.Close()
			return errlist.ErrG.NewError(err, "commit transaction failed")
		}
	} else {
		if err := k.doProducer(ctx, producer, topic, arg); err != nil {
			producer.Close()
			return errlist.ErrG.NewError(err, "do producer failed")
		}
	}

	k.producerPool.Put(producerPair)
	return nil
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
		newConn, newErr := kafka.NewConsumer(k.kafkaConf)
		if newErr != nil {
			return nil, nil, errlist.ErrG.NewError(newErr, "create consumer failed %s", k.conf.Server)
		}
		topics := k.splitTopicFromQuery(arg.Query)
		if err = newConn.SubscribeTopics(topics, nil); err != nil {
			return nil, nil, errlist.ErrG.NewError(err, "subscribe topic failed %v", topics)
		}

		readConn = &syncKafkaReadClient{
			First:  newConn,
			Second: sync.Mutex{},
		}
		k.consumerMap.Store(arg.Query, readConn)
	}

	count, timeoutMs, argErr := k.getArg(arg)
	if argErr != nil {
		return nil, nil, errlist.ErrG.NewError(argErr, "get args failed")
	}

	data, cols, readErr := readConn.Read(count, timeoutMs)
	if readErr != nil {
		_ = readConn.Close()
		return nil, nil, errlist.ErrG.NewError(readErr, "read failed")
	}
	return data, cols, nil
}

func (k *kafkaConnPool) GetConfig() types.ConnConfig {
	return *k.conf
}

func (k *kafkaConnPool) Close() error {
	k.isClose.Store(true)
	return nil
}

var _ types.ConnPoolInterface = (*kafkaConnPool)(nil)
