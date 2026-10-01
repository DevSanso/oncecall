package pool

import (
	"bytes"
	"context"
	"fmt"
	"oncecall/define"
	"oncecall/errlist"
	"oncecall/pool/internal/utils"
	"oncecall/pool/types"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/ssh"
)

type addressKey string

type sshNonInteractivePool struct {
	utils.ShUtils
	isCloseFlag atomic.Bool
	client      *ssh.Client

	address     string
	config      *ssh.ClientConfig
	splitChar   string
	newlineChar string
	sh          string

	initClientMutex sync.Mutex

	conf *types.ConnConfig
}

func newNonInteractiveSSHConnPool(info *types.ConnConfig) (types.ConnPoolInterface, error) {
	if info.DBType != string(define.SSH) {
		return nil, errlist.ErrG.NewError(nil, "not match db type: %s", info.DBType)
	}

	user := info.Id
	passwd := info.Password
	ip := info.Server

	if info.OptionMap == nil {
		return nil, errlist.ErrG.NewError(nil, "not exists option string")
	}

	if _, exists := info.OptionMap["split"]; !exists {
		return nil, errlist.ErrG.NewError(nil, "not exists split option string")
	}

	if _, exists := info.OptionMap["newline"]; !exists {
		return nil, errlist.ErrG.NewError(nil, "not exists newline option string")
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(passwd),
		},
	}

	if data, exists := info.OptionMap["hostkey"]; exists {
		switch data {
		case "publickey":
			keyData, keyExists := info.OptionMap["publickey"]
			if !keyExists {
				return nil, errlist.ErrG.NewError(nil, "not exists publickey, publickey hostkey")
			}

			if convertKey, convertOk := keyData.([]byte); convertOk {
				hostkey, _, _, _, authErr := ssh.ParseAuthorizedKey(convertKey)
				if authErr != nil {
					return nil, errlist.ErrG.NewError(authErr, "parse hostkey failed")
				}
				config.HostKeyCallback = ssh.FixedHostKey(hostkey)
			} else {
				return nil, errlist.ErrG.NewError(nil, "convert bytes failed hostkey")
			}
		default:
			config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	} else {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	var sh string = ""
	if optionSh, exists := info.OptionMap["sh"]; exists {
		var ok bool
		sh, ok = optionSh.(string)
		if !ok {
			return nil, errlist.ErrG.NewError(nil, "profileLoad not string")
		}
	}

	return &sshNonInteractivePool{
		address:     ip,
		config:      config,
		splitChar:   info.OptionMap["split"].(string),
		newlineChar: info.OptionMap["newline"].(string),
		conf:        info,
		sh:          sh,
	}, nil

}

func (s *sshNonInteractivePool) GetConfig() types.ConnConfig {
	return *s.conf
}

func (s *sshNonInteractivePool) getSession() (sess *ssh.Session, err error) {
	s.initClientMutex.Lock()
	defer s.initClientMutex.Unlock()
	if s.client == nil {
		if s.client, err = ssh.Dial("tcp", s.address, s.config); err != nil {
			return nil, err
		}
	}

	return s.client.NewSession()
}

func (s *sshNonInteractivePool) RunExecute(ctx context.Context, arg *types.Args) error {
	if s.isCloseFlag.Load() {
		return errlist.ErrG.NewError(nil, "already close ssh conn")
	}

	if sess, err := s.getSession(); err != nil {
		return err
	} else {
		cmd := ""

		if s.sh != "" {
			cmd = fmt.Sprintf(`%s "%s"`, s.sh, arg.Query)
		} else {
			cmd = arg.Query
		}

		err = sess.Run(cmd)

		sess.Close()
		return err
	}
}

func (s *sshNonInteractivePool) RunQuery(ctx context.Context, arg *types.Args) (rows [][]any, name []string, err error) {
	if s.isCloseFlag.Load() {
		return nil, nil, errlist.ErrG.NewError(nil, "already close ssh conn")
	}

	if sess, err := s.getSession(); err != nil {
		return nil, nil, err
	} else {
		defer sess.Close()

		var b bytes.Buffer
		var errB bytes.Buffer

		sess.Stderr = &errB
		sess.Stdout = &b

		cmd := ""

		if s.sh != "" {
			cmd = fmt.Sprintf(`%s '%s'`, s.sh, arg.Query)
		} else {
			cmd = arg.Query
		}

		err = sess.Run(cmd)

		if err != nil {
			if errB.Len() > 0 {
				return nil, nil, errlist.ErrG.NewError(err, "cmd err:%s", errB.String())
			}
			return nil, nil, errlist.ErrG.NewError(err, "ssh err")
		} else if errB.Len() > 0 {
			return nil, nil, errlist.ErrG.NewError(nil, "cmd err:%s", errB.String())
		}

		rows, name = s.ParseResponse(b.String(), s.newlineChar, s.splitChar)
		err = nil
		return
	}
}
func (s *sshNonInteractivePool) Close() error {
	if s.isCloseFlag.Swap(true) {
		return errlist.ErrG.NewError(nil, "already close ssh conn")
	}

	return nil
}
