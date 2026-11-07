package connection

import (
	"context"
	"io"
	"oncecall/connpool/internal/utils"
	"oncecall/connpool/types"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"os/exec"
	"sync"
	"sync/atomic"
)

type localShConn struct {
	connUtils     utils.ShUtils
	isCloseFlag   atomic.Bool
	isInteractive bool

	splitChar   string
	newlineChar string

	inputNextLineChar string
	inputDivisionChar string

	shMutex sync.Mutex

	interactive struct {
		proc   *exec.Cmd
		stdin  io.WriteCloser
		stdout io.ReadCloser

		outputEndChar string
		inputEndChar  string
	}

	nonInteractive struct {
		execPath string
		args     []string
	}
}

func NewLocalShConn(isInteractive bool, execPath, outputSplitChar, outputNewlineChar, inputNextLineChar, inputDivisionChar, inputEndChar, outputEndChar string, args ...string) (types.Conn, error) {
	c := &localShConn{
		isCloseFlag:       atomic.Bool{},
		splitChar:         outputSplitChar,
		newlineChar:       outputNewlineChar,
		inputNextLineChar: inputNextLineChar,
		inputDivisionChar: inputDivisionChar,
		isInteractive:     isInteractive,
		shMutex:           sync.Mutex{},
		connUtils:         utils.ShUtils{},
	}

	if isInteractive {
		cmd := exec.Command(execPath, args...)
		stdin, stdinErr := cmd.StdinPipe()
		if stdinErr != nil {
			return nil, errlist.ErrG.NewError(stdinErr, "exec stdin failed : %s", execPath)
		}

		stdout, stdoutErr := cmd.StdoutPipe()
		if stdoutErr != nil {
			return nil, errlist.ErrG.NewError(stdoutErr, "exec stdout failed : %s", execPath)
		}

		if startErr := cmd.Start(); startErr != nil {
			return nil, errlist.ErrG.NewError(startErr, "start failed : %s", execPath)
		}

		c.interactive.proc = cmd
		c.interactive.stdin = stdin
		c.interactive.stdout = stdout
		c.interactive.inputEndChar = inputEndChar
		c.interactive.outputEndChar = outputEndChar
	} else {
		c.nonInteractive.execPath = execPath
		c.nonInteractive.args = args
	}

	return c, nil

}

func (l *localShConn) RunExecute(ctx context.Context, arg *types.Args) error {
	if l.isCloseFlag.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "exec closed : %s", l.interactive.proc.Path)
	}
	l.shMutex.Lock()
	defer l.shMutex.Unlock()

	if l.isInteractive {
		_, writeErr := l.interactive.stdin.Write(l.connUtils.MakeParam(l.inputNextLineChar,
			l.inputDivisionChar, l.interactive.inputEndChar, arg))

		if writeErr != nil {
			return errlist.ErrG.NewError(writeErr, "process write error, %s", l.interactive.proc.Path)
		}
	} else {
		cmd := exec.Command(l.nonInteractive.execPath, l.nonInteractive.args...)
		stdoutErr := cmd.Start()
		if stdoutErr != nil {
			return errlist.ErrG.NewError(stdoutErr, "interactive exec failed : %s", l.nonInteractive.execPath)
		}
	}
	return nil
}

func (l *localShConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	if l.isCloseFlag.Load() {
		return nil, nil, errlist.ErrG.NewError(prefix.ClosedError, "exec closed : %s", l.interactive.proc.Path)
	}

	l.shMutex.Lock()
	defer l.shMutex.Unlock()

	if l.isInteractive {
		_, writeErr := l.interactive.stdin.Write(l.connUtils.MakeParam(l.inputNextLineChar,
			l.inputDivisionChar, l.interactive.inputEndChar, arg))
		if writeErr != nil {
			return nil, nil, errlist.ErrG.NewError(writeErr, "process write error, %s", l.interactive.proc.Path)
		}
		rows, name, err = l.connUtils.ParseResponseFromReader(l.interactive.stdout, l.newlineChar, l.splitChar, l.interactive.outputEndChar)
		if err != nil {
			err = errlist.ErrG.NewError(err, "parse failed response %s", l.interactive.proc.Path)
		}
	} else {
		cmd := exec.Command(l.nonInteractive.execPath, l.nonInteractive.args...)
		stdoutData, stdoutErr := cmd.Output()
		if stdoutErr != nil {
			return nil, nil, errlist.ErrG.NewError(stdoutErr, "non interactive exec failed : %s", l.nonInteractive.execPath)
		}
		rows, name = l.connUtils.ParseResponse(string(stdoutData), l.newlineChar, l.splitChar)
		err = nil
	}
	return
}

func (l *localShConn) Close() error {
	if l.isCloseFlag.Load() {
		return errlist.ErrG.NewError(prefix.ClosedError, "exec closed : %s", l.interactive.proc.Path)
	}

	l.shMutex.Lock()
	defer l.shMutex.Unlock()

	if l.isInteractive {
		_ = l.interactive.stdin.Close()
		_ = l.interactive.stdout.Close()
		_ = l.interactive.proc.Process.Kill()
		_, _ = l.interactive.proc.Process.Wait()
	}
	l.isCloseFlag.Store(true)
	return nil
}
