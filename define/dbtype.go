package define

type POOLType string

const (
	SQLSVR    POOLType = "sqlserver"
	POSTGRES  POOLType = "postgres"
	SQLITE    POOLType = "sqlite"
	SAPHANA   POOLType = "hanadb"
	MYSQL     POOLType = "mysql"
	REDIS     POOLType = "redis"
	SSH       POOLType = "ssh"
	CASSANDRA POOLType = "cassandra"
	LOCAL     POOLType = "local"
	KAFKA     POOLType = "kafka"
)
