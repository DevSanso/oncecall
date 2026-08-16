package manage

import (
	"context"
	"fmt"
	"oncecall/cfg"
	"oncecall/conn"
	"oncecall/errlist"
)

type ServerDbInfo struct {
	Identifier int            `toml:"identifier"`
	DBPrefix   string         `toml:"db_prefix"`
	DbType     string         `toml:"dbtype"`
	Name       string         `toml:"name"`
	IpAddress  string         `toml:"address"`
	DbPort     int            `toml:"port"`
	UserId     string         `toml:"user"`
	Password   string         `toml:"password"`
	Option     map[string]any `toml:"option"`
}

func (s *ServerDbInfo) ConvertConnConfig() *cfg.ConnConfig {
	return &cfg.ConnConfig{
		DBType:    string(s.DbType),
		Server:    fmt.Sprintf("%s:%d", s.IpAddress, s.DbPort),
		Name:      s.Name,
		Id:        s.UserId,
		Password:  s.Password,
		MaxConn:   10,
		DataMap:   s.ConvertMap(),
		OptionMap: s.Option,
	}
}

type DBSearch struct {
	manageDb          conn.ConnPoolInterface
	manageQuery       string
	manageOptionQuery string
}

func NewDBSearch(manageDb conn.ConnPoolInterface, manageQuery string, manageOptionQuery string) *DBSearch {
	return &DBSearch{manageDb: manageDb, manageQuery: manageQuery, manageOptionQuery: manageOptionQuery}
}

func (c *DBSearch) initDbOption(ctx context.Context, info []ServerDbInfo) error {
	if c.manageOptionQuery == "" {
		return nil
	}

	var rowBuf struct {
		Key   string
		Value string
	}

	var convert string
	var ok bool

	for idx := range info {
		data, err := c.manageDb.RunQuery(ctx, &conn.Args{
			Query: c.manageOptionQuery,
			Args:  [][]any{{info[idx].Identifier}},
		})

		if err != nil {
			return errlist.ErrG.NewError(err, "get failed manage db list")
		}

		for rowIdx := range data {
			if len(data[rowIdx]) < 2 {
				return errlist.ErrG.NewError(nil, "not size two : %s", c.manageOptionQuery)
			}

			convert, ok = data[rowIdx][0].(string)
			if !ok {
				return errlist.ErrG.NewError(nil, "convert failed : %d.1:%s", info[idx].Identifier, c.manageOptionQuery)
			}
			rowBuf.Key = convert
			convert, ok = data[rowIdx][1].(string)
			if !ok {
				return errlist.ErrG.NewError(nil, "not size two :  %d.2:%s", info[idx].Identifier, c.manageOptionQuery)
			}
			rowBuf.Value = convert

			info[idx].Option[rowBuf.Key] = rowBuf.Value
			rowBuf = struct {
				Key   string
				Value string
			}{"", ""}
		}
	}

	return nil

}

func (c *DBSearch) GetDb(ctx context.Context) ([]ServerDbInfo, error) {
	query := ""
	if c.manageQuery == "" {
		return nil, errlist.ErrG.NewError(nil, "not exists query")
	} else {
		query = c.manageQuery
	}

	data, dataErr := c.manageDb.RunQuery(ctx, &conn.Args{
		Query:         query,
		Args:          nil,
		IsTranscation: false,
	})

	if dataErr != nil {
		return nil, dataErr
	}

	res := make([]ServerDbInfo, len(data))

	if len(data) > 0 && len(data[0]) < 7 {
		return nil, errlist.ErrG.NewError(nil, "not match data length %d", len(data[0]))
	}

	for idx := range data {
		var convertOk = true
		var temp = false
		var intBuf int64

		intBuf, temp = data[idx][0].(int64)
		convertOk = convertOk && temp
		res[idx].Identifier = int(intBuf)
		res[idx].DBPrefix, temp = data[idx][1].(string)
		convertOk = convertOk && temp
		res[idx].Name, temp = data[idx][2].(string)
		convertOk = convertOk && temp
		res[idx].IpAddress, temp = data[idx][3].(string)
		convertOk = convertOk && temp
		intBuf, convertOk = data[idx][4].(int64)
		convertOk = convertOk && temp
		res[idx].DbPort = int(intBuf)
		res[idx].UserId, temp = data[idx][5].(string)
		convertOk = convertOk && temp
		res[idx].Password, temp = data[idx][6].(string)
		convertOk = convertOk && temp
		res[idx].DbType, temp = data[idx][7].(string)
		convertOk = convertOk && temp
		res[idx].Option = make(map[string]any)

		if !convertOk {
			return nil, errlist.ErrG.NewError(nil, "convert failed data")
		}
	}

	if optErr := c.initDbOption(ctx, res); optErr != nil {
		return nil, optErr
	}

	return res, nil
}

func (s *ServerDbInfo) ConvertMap() map[string]any {
	return map[string]any{
		"{Server.DbPrefix}":   s.DBPrefix,
		"{Server.Dbtype}":     s.DbType,
		"{Server.Identifier}": s.Identifier,
		"{Server.UserId}":     s.UserId,
		"{Server.Name}":       s.Name,
		"{Server.Address}":    fmt.Sprintf("%s:%d", s.IpAddress, s.DbPort),
	}
}
