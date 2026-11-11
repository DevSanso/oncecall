package gen

import (
	"context"
	"oncecall/connpool/internal/connection"
	"oncecall/connpool/types"
	"oncecall/errset"
	"oncecall/extension/log"
	"strconv"
	"strings"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

type cassandraGenerator struct {
	info    types.ConnConfig
	cluster *gocql.ClusterConfig
}

func NewCassandraGenerator(info types.ConnConfig, lExtension log.LoggerLogExtension[any]) (Generator, error) {
	hosts := strings.Split(info.Server, ",")
	port := strings.Split(info.Server, ":")
	if len(hosts) <= 0 || len(port) < 2 {
		return nil, errset.ErrG.NewError(nil, "address check : %s", info.Server)
	}

	portNum, portErr := strconv.Atoi(port[1])
	if portErr != nil {
		return nil, errset.ErrG.NewError(portErr, "port convert failed : %s", port[1])
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = portNum
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: info.Id,
		Password: info.Password,
	}
	cluster.Keyspace = info.Name
	cluster.NumConns = info.MaxConn
	cluster.Consistency = gocql.Quorum

	return &cassandraGenerator{info: info, cluster: cluster}, nil
}

func (c *cassandraGenerator) Gen(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
	if sess, sessErr := c.cluster.CreateSession(); sessErr != nil {
		return nil, errset.ErrG.NewError(sessErr, "create session failed")
	} else {
		return connection.NewCassandraConn(sess)
	}
}
