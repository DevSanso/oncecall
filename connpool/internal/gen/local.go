package gen

import (
	"context"
	"oncecall/connpool/internal/connection"
	"oncecall/connpool/internal/utils"
	"oncecall/connpool/types"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/log"
)

type localGenerator struct {
	info types.ConnConfig

	interactive       bool
	outputSplitChar   string
	outputNewlineChar string
	outputEndChar     string
	inputNextLineChar string
	inputDivisionChar string
	inputEndChar      string
	args              []string
}

func NewLocalGenerator(info types.ConnConfig, lExtension log.LoggerLogExtension[any]) (Generator, error) {
	var exists bool
	var interactiveOpt, outputSplitCharOpt, outputNewlineCharOpt, outputEndCharOpt, inputNextLineCharOpt, inputDivisionCharOpt, inputEndCharOpt, argsOpt any

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
	outputEndCharOpt, exists = info.OptionMap["output.endchar"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.endchar not exists")
	}
	inputNextLineCharOpt, exists = info.OptionMap["input.nextline"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.nextline not exists")
	}
	inputDivisionCharOpt, exists = info.OptionMap["input.division"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.division not exists")
	}
	inputEndCharOpt, exists = info.OptionMap["input.endchar"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "input.endchar not exists")
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
		info:              info,
		interactive:       interactiveOpt.(bool),
		outputSplitChar:   outputSplitCharOpt.(string),
		outputNewlineChar: outputNewlineCharOpt.(string),
		inputNextLineChar: inputNextLineCharOpt.(string),
		inputDivisionChar: inputDivisionCharOpt.(string),
		inputEndChar:      inputEndCharOpt.(string),
		outputEndChar:     outputEndCharOpt.(string),
		args:              args,
	}, nil

}

func (l *localGenerator) Gen(ctx context.Context, l2 log.LoggerDebugExtension[any]) (types.Conn, error) {
	return connection.NewLocalShConn(
		l.interactive, l.info.Name, l.outputSplitChar, l.outputNewlineChar, l.inputNextLineChar, l.inputDivisionChar, l.inputEndChar, l.outputEndChar, l.args...)
}
