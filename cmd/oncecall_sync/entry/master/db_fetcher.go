package master

import (
	"context"
	"oncecall/cmd/oncecall_sync/global/config"
	"oncecall/cmd/oncecall_sync/global/manage"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/pool/types"
)

type DbFetcher interface {
	DBListIdent(ctx context.Context) ([]string, error)
}

type implDbFetcher struct{}

func NewDbFetcher() DbFetcher {
	return &implDbFetcher{}
}

func (i implDbFetcher) DBListIdent(ctx context.Context) ([]string, error) {
	data, _, err := manage.ManageP.RunQuery(ctx, &types.Args{
		Query:         config.Config.Cmd.Db.Query.Manage.DbSyncList,
		Args:          nil,
		IsTransaction: false,
	})

	if err != nil {
		return nil, errlist.ErrG.NewError(err, "db list ident failed : %s", config.Config.Cmd.Db.Query.Manage.DbSyncList)
	}

	if len(data) == 0 {
		return []string{}, nil
	}
	ret := make([]string, 0, len(data))

	for idx := range data {
		if len(data[idx]) != 1 {
			return nil, errlist.ErrG.NewError(prefix.SentinelCatchError, "db ident row length not matching %d", len(data[idx]))
		}

		conv, convOk := data[idx][0].(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(prefix.ParseError, "db ident row content not matching %v", data[idx][0])
		}

		ret = append(ret, conv)
	}

	return ret, nil
}
