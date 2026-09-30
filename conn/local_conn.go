package conn

import (
	"bytes"
	"context"
	"io"
	"oncecall/conn/internal/utils"
	"oncecall/conn/types"
	"oncecall/errlist"
	"os/exec"
	"sync"
	"sync/atomic"
)

type localNonInteractiveConnPool struct {
	isCloseFlag atomic.Bool

	splitChar   string
	newlineChar string

	inputNextLineChar string
	inputDivisionChar string

	conf *types.ConnConfig
}

func (l *localNonInteractiveConnPool) RunExecute(ctx context.Context, arg *types.Args) error {
	panic("implement me")
}

func (l *localNonInteractiveConnPool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (l *localNonInteractiveConnPool) GetConfig() types.ConnConfig {
	return *l.conf
}

func (l *localNonInteractiveConnPool) Close() error {
	//TODO implement me
	panic("implement me")
}

var _ types.ConnPoolInterface = (*localNonInteractiveConnPool)(nil)

type localInteractiveConnPool struct {
	utils.ShConnUtils
	isCloseFlag atomic.Bool

	splitChar   string
	newlineChar string

	inputNextLineChar string
	inputDivisionChar string

	conf *types.ConnConfig

	interactiveProc   *exec.Cmd
	interactiveStdin  io.WriteCloser
	interactiveStdout io.ReadCloser
	interactiveMutex  sync.Mutex
}

func (l *localInteractiveConnPool) RunExecute(ctx context.Context, arg *types.Args) error {
	l.interactiveMutex.Lock()
	defer l.interactiveMutex.Unlock()

	_, writeErr := l.interactiveStdin.Write(l.MakeParam(l.inputNextLineChar, l.inputDivisionChar, arg))
	if writeErr != nil {
		return errlist.ErrG.NewError(writeErr, "process write error, %s", l.interactiveProc.Path)
	}
	var readBuf bytes.Buffer
	readBuf.Grow(1)
	_, copyErr := io.Copy(&readBuf, l.interactiveStdout)

	if copyErr != nil {
		return errlist.ErrG.NewError(copyErr, "process read error, %s", l.interactiveProc.Path)
	}
	return nil
}

func (l *localInteractiveConnPool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	l.interactiveMutex.Lock()
	defer l.interactiveMutex.Unlock()

	_, writeErr := l.interactiveStdin.Write(l.MakeParam(l.inputNextLineChar, l.inputDivisionChar, arg))
	if writeErr != nil {
		return nil, nil, errlist.ErrG.NewError(writeErr, "process write error, %s", l.interactiveProc.Path)
	}
	var readBuf bytes.Buffer
	readBuf.Grow(1024)
	_, copyErr := io.Copy(&readBuf, l.interactiveStdout)
	if copyErr != nil {
		return nil, nil, errlist.ErrG.NewError(copyErr, "process read error, %s", l.interactiveProc.Path)
	}

	rows, name = l.ParseResponse(readBuf.String(), l.newlineChar, l.splitChar)
	err = nil
	return
}

func (l *localInteractiveConnPool) GetConfig() types.ConnConfig {
	return *l.conf
}

func (l *localInteractiveConnPool) Close() error {
	//TODO implement me
	panic("implement me")
}

var _ types.ConnPoolInterface = (*localInteractiveConnPool)(nil)
