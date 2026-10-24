package types

import (
	"context"
	"oncecall/cmd/oncecall_sync/cfg"
	"oncecall/pool/types"
)

type DbFetcher interface {
	DbList(ctx context.Context, p types.ConnPool) ([]cfg.ConnConfig, error)
	Script(ctx context.Context, p types.ConnPool) ([]cfg.ScriptConfig, error)
	DbAndScriptLink(ctx context.Context, p types.ConnPool) ([]struct {
		DbUUID     string
		ScriptName string
	}, error)
}
