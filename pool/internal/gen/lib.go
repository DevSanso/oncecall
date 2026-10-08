package gen

import (
	"context"
	"oncecall/extension/log"
	"oncecall/pool/types"
)

type Generator interface {
	Gen(context.Context, log.LoggerDebugExtension[any]) (types.Conn, error)
}
