package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"

	"github.com/IBM/sarama"
)

type kafkaConn struct {
	consumer sarama.Consumer
	producer sarama.SyncProducer

	consumerOffset int64
	topic          string
}

func NewKafkaConn(addr string, topic string) (types.Conn, error) {
	c, cErr := sarama.NewConsumer([]string{addr}, nil)
	if cErr != nil {
		return nil, errlist.ErrG.NewError(cErr, "kafka consumer get failed")
	}

	p, pErr := sarama.NewSyncProducer([]string{addr}, nil)
	if pErr != nil {
		_ = c.Close()
		return nil, errlist.ErrG.NewError(cErr, "kafka consumer get failed")
	}

	return &kafkaConn{
		consumer:       c,
		producer:       p,
		topic:          topic,
		consumerOffset: sarama.OffsetNewest,
	}, nil
}

func (k *kafkaConn) RunExecute(ctx context.Context, arg *types.Args) error {
	err := k.WriteData(k.topic, arg)
	if err != nil {
		return errlist.ErrG.NewError(err, "write data failed")
	}
	return nil
}

func (k *kafkaConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	var res struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(arg.Query), &res); err != nil {
		return nil, nil, errlist.ErrG.NewError(err, fmt.Sprintf("unmarshal query failed, %s", arg.Query))
	}
	var temp int64
	rows, name, temp, err = k.ReadData(ctx, k.topic, res.Count, k.consumerOffset)
	if err != nil {
		return nil, nil, errlist.ErrG.NewError(err, fmt.Sprintf("read data failed (topic:%s, offset:%d)", k.topic, k.consumerOffset))
	}
	k.consumerOffset = temp
	return rows, name, nil
}

func (k *kafkaConn) Close() error {
	_ = k.consumer.Close()
	_ = k.producer.Close()
	return nil
}

func (k *kafkaConn) WriteData(topicName string, arg *types.Args) error {
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

		messages = append(messages, &sarama.ProducerMessage{
			Topic: topicName,
			Value: messageValue,
		})
	}

	if arg.IsTransaction {
		if txErr := k.producer.BeginTxn(); txErr != nil {
			_ = k.producer.AbortTxn()
			return errlist.ErrG.NewError(txErr, "txn is failed")
		}

		if sendErr := k.producer.SendMessages(messages); sendErr != nil {
			_ = k.producer.AbortTxn()
			return errlist.ErrG.NewError(sendErr, "tx send error, topic :%s", topicName)
		}

		if commitTxn := k.producer.CommitTxn(); commitTxn != nil {
			_ = k.producer.AbortTxn()
			return errlist.ErrG.NewError(commitTxn, "txn is commit failed")
		}

	} else {
		if sendErr := k.producer.SendMessages(messages); sendErr != nil {
			return errlist.ErrG.NewError(sendErr, "send error, topic :%s", topicName)
		}
	}

	return nil
}

func (k *kafkaConn) ReadData(ctx context.Context, topicName string, count int, offset int64) ([][]any, []string, int64, error) {
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

	c := k.consumer

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
			case errMsg := <-ch:
				return nil, nil, offset, errlist.ErrG.NewError(errMsg.Err, "consume send error string, topic :%s", errMsg.Topic)
			default:
			}
		}

		for _, ch := range channels {
			select {
			case sendMsg := <-ch:
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
