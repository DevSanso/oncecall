package pool

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/internal/connection"
	"oncecall/pool/internal/utils"
	"oncecall/pool/types"
	"oncecall/utils/generic"
)

type localConnPool struct {


	conf        types.ConnConfig
	connUsePool *generic.SyncUsePool[*connection.LocalShConn, any]
}

func newLocalConnPool(conf *types.ConnConfig) (types.ConnPoolInterface, error) {
	var exists bool
	var interactiveOpt, outputSplitCharOpt, outputNewlineCharOpt, inputNextLineCharOpt, inputDivisionCharOpt, argsOpt any

	interactiveOpt, exists = conf.OptionMap["interactive"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "interactive not exists")
	}
	outputSplitCharOpt, exists = conf.OptionMap["output.splitchar"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "output.splitchar not exists")
	}
	outputNewlineCharOpt, exists = conf.OptionMap["output.newline"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "output.newline not exists")
	}
	inputNextLineCharOpt, exists = conf.OptionMap["input.nextline"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.nextline not exists")
	}
	inputDivisionCharOpt, exists = conf.OptionMap["input.division"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.division not exists")
	}
	argsOpt, exists = conf.OptionMap["args"]
	var args []string = []string{}

	if exists {
		argStr, convOk := argsOpt.(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(prefix.ParseError, "args convert failed %v", argsOpt)
		}
		var commonUtils utils.CommonUtils
		args = commonUtils.SplitRespectQuotesStr(argStr)
	}
	p := &localConnPool{
		conf : *conf,
	}

	p.connUsePool = generic.NewGenericSyncUsePool[*connection.LocalShConn, any](func() (*connection.LocalShConn, error) {
		return 	connection.NewLocalShConn(
			interactiveOpt.(bool),
			conf.Name,
			outputSplitCharOpt.(string),
			outputNewlineCharOpt.(string),
			inputNextLineCharOpt.(string),
			inputDivisionCharOpt.(string), args...)
	})

	return p, nil
}

func (l *localConnPool) RunExecute(ctx context.Context, arg *types.Args) error {
	_, err := l.connUsePool.Use(func(data *connection.LocalShConn) (any, error) {
		err := data.RunExecute(ctx, arg)
		return nil, err
	})

	if err != nil {
		return errlist.ErrG.NewError(err, "useFn Failed")
	}

	return nil
}

func (l *localConnPool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	type QueryRet struct {
		rows [][]any
		name []string
	}

	useData, err := l.connUsePool.Use(func(data *connection.LocalShConn) (any, error) {
		queryRows, queryName, queryErr := data.RunQuery(ctx, arg)
		return QueryRet{rows: queryRows, name: queryName}, queryErr
	})

	if err != nil {
		return nil, nil, errlist.ErrG.NewError(err, "useFn Failed")
	}

	data := useData.(QueryRet)
	return data.rows, data.name, nil
}

func (l *localConnPool) GetConfig() types.ConnConfig {
	return l.conf
}

func (l *localConnPool) Close() error {
	_ = l.connUsePool.Close()
	return nil
}

var _ types.ConnPoolInterface = (*localConnPool)(nil)
