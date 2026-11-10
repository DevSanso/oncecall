package connection

import (
	"bytes"
	"context"
	"oncecall/connpool/internal/utils"
	"oncecall/connpool/types"
	"oncecall/errlist"

	"golang.org/x/crypto/ssh"
)

type sshConn struct {
	shUtils     utils.ShUtils
	sess        *ssh.Session
	newlineChar string
	splitChar   string
}

func NewSshConn(sess *ssh.Session, newlineChar string, splitChar string) types.Conn {
	return &sshConn{sess: sess, newlineChar: newlineChar, splitChar: splitChar}
}

func (s *sshConn) RunExecute(ctx context.Context, arg *types.Args) error {
	if err := s.sess.Run(arg.Query); err != nil {
		return errlist.ErrG.NewError(err, "")
	}
	return nil
}

func (s *sshConn) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, ty []types.ConnPoolColType, err error) {
	var b bytes.Buffer
	var errB bytes.Buffer

	sess := s.sess
	sess.Stderr = &errB
	sess.Stdout = &b
	defer func() {
		sess.Stdin = nil
		sess.Stderr = nil
	}()

	err = sess.Run(arg.Query)

	if err != nil {
		if errB.Len() > 0 {
			return nil, nil, nil, errlist.ErrG.NewError(err, "cmd err:%s", errB.String())
		}
		return nil, nil, nil, errlist.ErrG.NewError(err, "ssh err")
	} else if errB.Len() > 0 {
		return nil, nil, nil, errlist.ErrG.NewError(nil, "cmd err:%s", errB.String())
	}

	rows, name = s.shUtils.ParseResponse(b.String(), s.newlineChar, s.splitChar)
	err = nil

	if len(rows) <=0 {
		return nil, nil, nil, errlist.ErrG.NewError(nil, "not exists")
	}

	ty = make([]types.ConnPoolColType, len(rows[0]))
	for idx := range len(rows[0]) {
		ty[idx] = types.ConnPoolColTypeText
	}

	return
}

func (s *sshConn) Close() error {
	return s.sess.Close()
}
