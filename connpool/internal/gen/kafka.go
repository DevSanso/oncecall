package gen

import (
	"context"
	"oncecall/connpool/internal/connection"
	"oncecall/connpool/types"
	"oncecall/extension/log"
)

type kafkaGenerator struct {
	info types.ConnConfig
}

func NewKafkaGenerator(info types.ConnConfig, lExtension log.LoggerLogExtension[any]) (Generator, error) {
	return &kafkaGenerator{info: info}, nil
}

func (k *kafkaGenerator) Gen(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
	return connection.NewKafkaConn(k.info.Server, k.info.Name)
}
