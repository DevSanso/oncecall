package utils

import (
	"fmt"
	"oncecall/connpool/types"
	"oncecall/define"
	"oncecall/errset"
	"oncecall/errset/prefix"
)

type StdSqlUtils struct{}

func (StdSqlUtils) GetConnUrlAndDriver(c *types.ConnConfig) (driver string, url string, e error) {
	switch define.POOLType(c.DBType) {
	case define.SQLITE:
		driver = "sqlite3"
		url = c.Name
	case define.MYSQL:
		driver = "mysql"
		url = fmt.Sprintf("%s:%s@tcp(%s)/%s", c.Id, c.Password, c.Server, c.Name)
	case define.POSTGRES:
		driver = "postgres"
		url = fmt.Sprintf("postgres://%s:%s@%s/%s?application_name=%s&sslmode=disable",
			c.Id, c.Password, c.Server, c.Name, "oncecall")
	case define.SQLSVR:
		driver = "mssql"
		url = fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s; encrypt=disable; app name=%s",
			c.Server, c.Id, c.Password, c.Name, "oncecall")
	case define.SAPHANA:
		driver = "hdb"
		url = fmt.Sprintf("hdb://%s:%s@%s", c.Id, c.Password, c.Server)
	case define.DUCKDB:
		driver = "duckdb"
		url = c.Name
	default:
		e = errset.ErrG.NewError(prefix.NotMatchingError, "not supported driver %s", c.DBType)
	}
	return
}
