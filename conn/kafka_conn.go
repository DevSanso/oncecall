package conn

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/utils/generic"
	"strings"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type syncKafkaReadClient generic.Pair[*kafka.Consumer, sync.Mutex]
type syncKafkaWriteClient generic.Pair[*kafka.Consumer, sync.Mutex]

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
	conf *ConnConfig

	kafkaConf *kafka.ConfigMap

	readTimeoutMs  int
	writeTimeoutMs int

	consumerMap *generic.GenericSyncMap[string, *syncKafkaReadClient]
	producerMap *generic.GenericSyncMap[string, *syncKafkaWriteClient]
}

func (*kafkaConnPool) getArg(arg *Args) (count int, err error) {
	if len(arg.Args) <= 0 || len(arg.Args[0]) < 1 {
		return -1, errlist.ErrG.NewError(prefix.SentinelCatchError, "kafka need count args")
	}

	var convOk bool
	count, convOk = arg.Args[0][0].(int)
	if !convOk {
		return -1, errlist.ErrG.NewError(prefix.NotMatchingError, "kafka convert failed arg %v", arg.Args[0][0])
	}

	return
}

func (*kafkaConnPool) splitTopicFromQuery(query string) []string {
	return strings.Split(query, ",")
}

func (k *kafkaConnPool) RunExecute(ctx context.Context, arg *Args) error {
	if arg.Query == "" {
		return errlist.ErrG.NewError(prefix.SentinelCatchError, "topic is empty")
	}
	//topics := k.splitTopicFromQuery(arg.Query)

	return nil
}

func (k kafkaConnPool) RunQuery(ctx context.Context, arg *Args) (rows [][]any, name []string, err error) {
	if arg.Query == "" {
		return nil, nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "topic is empty")
	}
	//topics := k.splitTopicFromQuery(arg.Query)

	return nil, nil, nil
}

func (k kafkaConnPool) GetConfig() ConnConfig {
	//TODO implement me
	panic("implement me")
}

func (k kafkaConnPool) Close() error {
	//TODO implement me
	panic("implement me")
}

var _ ConnPoolInterface = (*kafkaConnPool)(nil)
