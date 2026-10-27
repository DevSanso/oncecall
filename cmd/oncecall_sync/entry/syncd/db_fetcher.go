package syncd

import (
	"context"
	"crypto/md5"
	"oncecall/cmd/oncecall_sync/cfg"
	"oncecall/cmd/oncecall_sync/global/config"
	"oncecall/cmd/oncecall_sync/global/manage"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/parser"
	"oncecall/pool/types"
	"oncecall/utils/generic"
	"reflect"
	"unsafe"
)

type DbFetcher interface {
	Self(ctx context.Context) (types.ConnConfig, error)
	DBList(ctx context.Context) (map[string]cfg.ConnConfig, error)
	Script(ctx context.Context) (map[string]generic.Pair[[16]byte, cfg.ScriptConfig], error)
}

type implDbFetcher struct{}

func NewDbFetcher() DbFetcher {
	return &implDbFetcher{}
}

func (i implDbFetcher) parsingConnInfo(data [][]any, idx int, outputIdent *string, output *cfg.ConnConfig) error {
	var convOk bool = false

	*outputIdent, convOk = data[idx][0].(string)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db dbtype row content not matching %s", reflect.ValueOf(data[idx][0]).Type().Name())
	}

	output.DBType, convOk = data[idx][1].(string)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db dbtype row content not matching %s", reflect.ValueOf(data[idx][1]).Type().Name())
	}

	output.Server, convOk = data[idx][2].(string)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db server address row content not matching %s", reflect.ValueOf(data[idx][2]).Type().Name())
	}

	output.Name, convOk = data[idx][3].(string)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db dbname row content not matching %s", reflect.ValueOf(data[idx][3]).Type().Name())
	}

	output.Id, convOk = data[idx][4].(string)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db userid row content not matching %s", reflect.ValueOf(data[idx][4]).Type().Name())
	}

	output.Password, convOk = data[idx][5].(string)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db password row content not matching %s", reflect.ValueOf(data[idx][5]).Type().Name())
	}

	output.MaxConn, convOk = data[idx][6].(int)
	if !convOk {
		return errlist.ErrG.NewError(prefix.ParseError, "db maxconn row content not matching %s", reflect.ValueOf(data[idx][6]).Type().Name())
	}
	return nil
}

func (i implDbFetcher) dbOption(ctx context.Context, ident string) (map[string]any, error) {

	data, _, err := manage.ManageP.RunQuery(ctx, &types.Args{
		Query:         config.Config.Cmd.Db.Query.Sync.DbList,
		Args:          [][]any{{ident}},
		IsTransaction: false,
	})

	if err != nil {
		return nil, errlist.ErrG.NewError(err, "db option ident failed : %s", config.Config.Cmd.Db.Query.Manage.DbSyncList)
	}

	var ret = make(map[string]any, len(data))
	for idx := range data {
		if len(data[idx]) != 2 {
			return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "db script row length not matching %d", len(data[idx]))
		}

		key, convOk := data[idx][0].(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(prefix.ParseError, "db key row content not matching %s", reflect.ValueOf(data[idx][0]).Type().Name())
		}

		ret[key] = data[idx][1]
	}

	return ret, nil
}

func (i implDbFetcher) Self(ctx context.Context) (cfg.ConnConfig, error) {
	var ret cfg.ConnConfig

	data, _, err := manage.ManageP.RunQuery(ctx, &types.Args{
		Query:         config.Config.Cmd.Db.Query.Sync.DbList,
		Args:          [][]any{{config.SyncIdent}},
		IsTransaction: false,
	})

	if err != nil {
		return ret, errlist.ErrG.NewError(err, "db list ident failed : %s", config.Config.Cmd.Db.Query.Manage.DbSyncList)
	}

	if len(data) != 1 {
		return ret, errlist.ErrG.NewError(prefix.SentinelCatchError, "not unique data, ident=%s", config.SyncIdent)
	}

	var info cfg.ConnConfig
	var ident string

	if err := i.parsingConnInfo(data, 0, &ident, &info); err != nil {
		return ret, errlist.ErrG.NewError(err, "failed data[all:%d, current:%d]", len(data), 0)
	}

	if ident != config.SyncIdent {
		return ret, errlist.ErrG.NewError(prefix.SentinelCatchError, "%s != %s", ident, config.SyncIdent)
	}

	opt, optErr := i.dbOption(ctx, ident)
	if optErr != nil {
		return ret, errlist.ErrG.NewError(optErr, "get failed option")
	}
	ret.OptionMap = opt

	return ret, nil
}
func (i implDbFetcher) DBList(ctx context.Context) (map[string]cfg.ConnConfig, error) {
	data, _, err := manage.ManageP.RunQuery(ctx, &types.Args{
		Query:         config.Config.Cmd.Db.Query.Sync.DbList,
		Args:          nil,
		IsTransaction: false,
	})

	if err != nil {
		return nil, errlist.ErrG.NewError(err, "db list ident failed : %s", config.Config.Cmd.Db.Query.Manage.DbSyncList)
	}

	if len(data) == 0 {
		return nil, nil
	}
	ret := make(map[string]types.ConnConfig, len(data))

	for idx := range data {
		if len(data[idx]) != 1 {
			return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "db ident row length not matching %d", len(data[idx]))
		}

		var info cfg.ConnConfig
		var ident string

		if err := i.parsingConnInfo(data, idx, &ident, &info); err != nil {
			return nil, errlist.ErrG.NewError(err, "failed data[all:%d, current:%d]", len(data), idx)
		}
		opt, optErr := i.dbOption(ctx, ident)
		if optErr != nil {
			return ret, errlist.ErrG.NewError(optErr, "get failed option")
		}
		info.OptionMap = opt
		ret[ident] = info
	}

	return ret, nil
}

func (i implDbFetcher) Script(ctx context.Context) (map[string]generic.Pair[[16]byte, cfg.ScriptConfig], error) {
	data, _, err := manage.ManageP.RunQuery(ctx, &types.Args{
		Query:         config.Config.Cmd.Db.Query.Sync.Script,
		Args:          [][]any{{config.SyncIdent}},
		IsTransaction: false,
	})

	if err != nil {
		return nil, errlist.ErrG.NewError(err, "script ident failed : %s", config.Config.Cmd.Db.Query.Manage.DbSyncList)
	}

	if len(data) == 0 {
		return nil, nil
	}

	ret := make(map[string]generic.Pair[[16]byte, cfg.ScriptConfig], len(data))
	var parser parser.ParserExtension[cfg.ScriptConfig] = parser.TomlParserExtension[cfg.ScriptConfig]{}

	for idx := range data {
		if len(data[idx]) != 2 {
			return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "db script row length not matching %d", len(data[idx]))
		}

		var scriptName string
		var scriptData string
		var scriptConfig cfg.ScriptConfig
		var convOk bool

		scriptName, convOk = data[idx][0].(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(prefix.ParseError, "db dbtype row content not matching %s", reflect.ValueOf(data[idx][0]).Type().Name())
		}

		scriptData, convOk = data[idx][1].(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(prefix.ParseError, "db dbtype row content not matching %s", reflect.ValueOf(data[idx][1]).Type().Name())
		}

		if parserErr := parser.Parse(unsafe.Slice(unsafe.StringData(scriptData), len(scriptData)), &scriptConfig); parserErr != nil {
			return nil, errlist.ErrG.NewError(parserErr, "db script row content : %s", scriptConfig)
		}
		hash := md5.Sum(unsafe.Slice(unsafe.StringData(scriptData), len(scriptData)))
		ret[scriptName] = generic.Pair[[16]byte, cfg.ScriptConfig]{First: hash, Second: scriptConfig}
	}

	return ret, nil
}
