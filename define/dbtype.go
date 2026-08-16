package define

type DBType string

const (
	SQLSVR   DBType = "sqlserver"
	POSTGRES DBType = "postgres"
	SQLITE   DBType = "sqlite"
	SAPHANA  DBType = "hanadb"
	MYSQL    DBType = "mysql"
	REDIS    DBType = "redis"
	SSH      DBType = "ssh"
)
