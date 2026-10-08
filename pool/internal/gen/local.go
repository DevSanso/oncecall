package gen

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/log"
	"oncecall/pool/internal/connection"
	"oncecall/pool/internal/utils"
	"oncecall/pool/types"
)

type localGenerator struct {
	info types.ConnConfig

	interactive bool
	outputSplitChar string
	outputNewlineChar string
	inputNextLineChar string
	inputDivisionChar string
	args []string
}

func NewLocalGenerator(info types.ConnConfig, lExtension log.LoggerExtension[any]) (Generator, error) {
	var exists bool
	var interactiveOpt, outputSplitCharOpt, outputNewlineCharOpt, inputNextLineCharOpt, inputDivisionCharOpt, argsOpt any

	interactiveOpt, exists = info.OptionMap["interactive"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "interactive not exists")
	}
	outputSplitCharOpt, exists = info.OptionMap["output.splitchar"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "output.splitchar not exists")
	}
	outputNewlineCharOpt, exists = info.OptionMap["output.newline"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "output.newline not exists")
	}
	inputNextLineCharOpt, exists = info.OptionMap["input.nextline"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.nextline not exists")
	}
	inputDivisionCharOpt, exists = info.OptionMap["input.division"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.division not exists")
	}
	argsOpt, exists = info.OptionMap["args"]
	var args []string = []string{}

	if exists {
		argStr, convOk := argsOpt.(string)
		if !convOk {
			return nil, errlist.ErrG.NewError(prefix.ParseError, "args convert failed %v", argsOpt)
		}
		var commonUtils utils.CommonUtils
		args = commonUtils.SplitRespectQuotesStr(argStr)
	}

	return &localGenerator{
		info : info,
		interactive: interactiveOpt.(bool),
		outputSplitChar: outputSplitCharOpt.(string),
		outputNewlineChar: outputNewlineCharOpt.(string),
		inputNextLineChar: inputNextLineCharOpt.(string),
		inputDivisionChar: inputDivisionCharOpt.(string),
		args: args,

	}, nil

}

func (l *localGenerator) Gen(ctx context.Context, l2 log.LoggerDebugExtension[any]) (types.Conn, error) {
	return connection.NewLocalShConn(
		l.interactive, l.info.Name, l.outputSplitChar, l.outputNewlineChar, l.inputNextLineChar, l.inputDivisionChar, l.args... )
}

