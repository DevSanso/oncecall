package connection

import (
	"context"

	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
	"oncecall/utils/generic"

	"github.com/IBM/sarama"
)

type SyncKafkaClient generic.Pair[sarama.Consumer, sarama.SyncProducer]


func NewSyncKafkaClient(addr string, isConsumer bool) (*SyncKafkaClient, error) {
	o := &SyncKafkaClient{}
	var err error
	if isConsumer {
		o.First, err = sarama.NewConsumer([]string{addr}, nil)		
	} else {
		o.Second,err = sarama.NewSyncProducer([]string{addr}, nil)
	}

	if err != nil {
		return nil, errlist.ErrG.NewError(err, "")
	}

	return o, nil
}


func (s *SyncKafkaClient) Close() error {
	if s.First != nil {
		_ = s.First.Close()
	} 

	if s.Second != nil {
		_ = s.Second.Close()
	}

	return nil
}

func (s *SyncKafkaClient) WriteData(topicName string, arg *types.Args) error {
	if s.Second == nil {
		return errlist.ErrG.NewError(prefix.NotExistsError, "is not exists Consumer %v", s)
	}

	messages := make([]*sarama.ProducerMessage, 0, len(arg.Args))
	
	for idx, data := range arg.Args {
		if len(data) < 1 {
			return errlist.ErrG.NewError(prefix.SentinelCatchError, "data is empty %d", idx)
		}

		if data[0] == nil {
			continue
		}
		var messageValue sarama.Encoder

		switch data[0].(type) {
		case string:
			messageValue = sarama.StringEncoder(data[0].(string))
		case []byte:
			messageValue = sarama.ByteEncoder(data[0].([]byte))
		default:
			return errlist.ErrG.NewError(prefix.NotMatchingError, "convert failed data (only support string, []byte)%v", data)
		}

		messages = append(messages, &sarama.ProducerMessage {
			Topic: topicName,
			Value : messageValue,
		})
	}

	if arg.IsTransaction {
		if txErr := s.Second.BeginTxn(); txErr != nil {
			_ = s.Second.AbortTxn()
			return errlist.ErrG.NewError(txErr, "txn is failed")
		}

		if sendErr := s.Second.SendMessages(messages); sendErr != nil {
			_ = s.Second.AbortTxn()
			return errlist.ErrG.NewError(sendErr, "tx send error, topic :%s", topicName)
		}

		if commitTxn := s.Second.CommitTxn(); commitTxn != nil {
			_ = s.Second.AbortTxn()
			return errlist.ErrG.NewError(commitTxn, "txn is commit failed")
		}


	} else {
		if sendErr := s.Second.SendMessages(messages); sendErr != nil {
			return errlist.ErrG.NewError(sendErr, "send error, topic :%s", topicName)
		}
	}

	return nil
}

func (s *SyncKafkaClient) ReadData(ctx context.Context, topicName string, count int, offset int64) ([][]any, []string, int64, error) {
	if s.First == nil {
		return nil, nil, offset, errlist.ErrG.NewError(prefix.NotExistsError, "is not exists Consumer %v", s)
	}


	data := make([][]any, 5)

	data[0] = make([]any, 0, count/10)
	data[1] = make([]any, 0, count/10)
	data[2] = make([]any, 0, count/10)
	data[3] = make([]any, 0, count/10)
	data[4] = make([]any, 0, count/10)
	timestamp := data[0]
	topic := data[1]
	partition := data[2]
	key := data[3]
	value := data[4]

	c :=  s.First
	
	partitions, partErr := c.Partitions(topicName)
	if partErr != nil {
		return nil, nil, offset, errlist.ErrG.NewError(partErr, "get partition failed")
	}

	if offset < -1 {
		offset = sarama.OffsetOldest
	}

	channels := make([]<-chan *sarama.ConsumerMessage, 0, len(partitions))
	errChannels := make([]<-chan *sarama.ConsumerError, 0, len(partitions))
	for _, part := range partitions {
		channel, chanErr := c.ConsumePartition(topicName, part, offset)
		if chanErr != nil {
			return nil, nil, offset, errlist.ErrG.NewError(chanErr, "get consume channel %d", part)
		}
		
		defer channel.Close()
		channels = append(channels, channel.Messages())
		errChannels = append(errChannels, channel.Errors())
		
	}

	currentOffset := int64(0)
	currentCount := 0 

	for {
		if currentCount >= count {
			break
		}

		select {
		case <-ctx.Done():
			break
		default:
		}

		for _, ch := range errChannels {
			select {
			case errMsg := <- ch:
				return nil, nil, offset, errlist.ErrG.NewError(errMsg.Err, "consume send error string, topic :%s", errMsg.Topic)
			default:
			}
		}

		for _, ch := range channels {
			select {
			case sendMsg := <- ch:
				timestamp = append(timestamp, sendMsg.Timestamp.UnixMilli())
				topic = append(topic, sendMsg.Topic)
				partition = append(partition, int(sendMsg.Partition))
				key = append(key, sendMsg.Key)
				value = append(value, sendMsg.Value)

				currentOffset = max(currentOffset, sendMsg.Offset)
				currentCount += 1
			default:
			}
		}
	}

	return data, []string{
		"timestamp",
		"topic",
		"partition",
		"key",
		"value",
	}, currentOffset, nil
}
