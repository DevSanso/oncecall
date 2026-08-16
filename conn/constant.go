package conn

import (
	"errors"
	"fmt"

	"oncecall/cfg"
	"oncecall/define"
)

const CustomDriver = ""

var urlMap map[define.DBType]struct {
	Driver string
	Url    string
} = map[define.DBType]struct {
	Driver string
	Url    string
}{
	define.POSTGRES: {Driver: "postgres", Url: "postgres://%s:%s@%s/%s?application_name=%s&sslmode=disable"},
	define.SQLSVR:   {Driver: "mssql", Url: "server=%s;user id=%s;password=%s;database=%s; encrypt=disable; app name=%s"},
	define.SAPHANA:  {Driver: "hdb", Url: "hdb://%s:%s@%s"},
	define.SQLITE:   {Driver: "sqlite3", Url: "%s"},
	define.MYSQL:    {Driver: "mysql", Url: "%s:%s@tcp(%s)/%s"},
	define.REDIS:    {Driver: CustomDriver, Url: "redis://%s:%s@%s/%s"},
	define.SSH:      {Driver: CustomDriver, Url: "ssh://%s:%s@%s"},
}

var urlArgsFn map[define.DBType]func(*cfg.ConnConfig) []any = map[define.DBType]func(*cfg.ConnConfig) []any{
	define.POSTGRES: func(c *cfg.ConnConfig) []any { return []any{c.Id, c.Password, c.Server, c.Name, "oncecall"} },
	define.SQLSVR:   func(c *cfg.ConnConfig) []any { return []any{c.Server, c.Id, c.Password, c.Name, "oncecall"} },
	define.SAPHANA:  func(c *cfg.ConnConfig) []any { return []any{c.Id, c.Password, c.Server} },
	define.SQLITE:   func(c *cfg.ConnConfig) []any { return []any{c.Name} },
	define.MYSQL:    func(c *cfg.ConnConfig) []any { return []any{c.Id, c.Password, c.Server, c.Name} },
	define.REDIS:    func(c *cfg.ConnConfig) []any { return []any{c.Id, c.Password, c.Server, c.Name} },
}

func getConnUrlAndDriver(info *cfg.ConnConfig) (driver string, url string, e error) {
	dbtype := define.DBType(info.DBType)

	mapping, ok := urlMap[dbtype]
	if !ok {
		return "", "", errors.New("makeConnUrl - not support : " + string(dbtype))
	}
	argsFn, argsOk := urlArgsFn[dbtype]
	if !argsOk {
		return "", "", errors.New("makeConnUrl - not support : " + string(dbtype))
	}

	return mapping.Driver, fmt.Sprintf(mapping.Url, argsFn(info)...), nil
}

func getConnUrl(info *cfg.ConnConfig) (url string, e error) {
	_, url, e = getConnUrlAndDriver(info)
	return
}
