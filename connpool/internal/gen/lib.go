package gen

import (
	"context"
	"oncecall/connpool/types"
	"oncecall/extension/log"
)

type Generator interface {
	Gen(context.Context, log.LoggerDebugExtension[any]) (types.Conn, error)
}
