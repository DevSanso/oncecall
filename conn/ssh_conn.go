package conn

import (
	"bytes"
	"context"
	"fmt"
	"oncecall/cfg"
	"oncecall/define"
	"oncecall/errlist"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/ssh"
)

type addressKey string

type sshClientPool struct {
	isCloseFlag atomic.Bool
	client      *ssh.Client

	address     string
	config      *ssh.ClientConfig
	splitChar   string
	newlineChar string
	sh          string

	initClientMutex sync.Mutex

	conf *cfg.ConnConfig
}

func newSSHConnPool(info *cfg.ConnConfig) (ConnPoolInterface, error) {
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

	if _, exists := info.OptionMap["split"]; !exists {
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

	return &sshClientPool{
		address:     ip,
		config:      config,
		splitChar:   info.OptionMap["split"].(string),
		newlineChar: info.OptionMap["newline"].(string),
		conf:        info,
		sh:          sh,
	}, nil

}

func (s *sshClientPool) GetConfig() cfg.ConnConfig {
	return *s.conf
}

func (s *sshClientPool) getSession() (sess *ssh.Session, err error) {
	s.initClientMutex.Lock()
	defer s.initClientMutex.Unlock()
	if s.client == nil {
		if s.client, err = ssh.Dial("tcp", s.address, s.config); err != nil {
			return nil, err
		}
	}

	return s.client.NewSession()
}

func (s *sshClientPool) RunExecute(ctx context.Context, arg *Args) error {
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

func (s *sshClientPool) countFields(line string, sep string) int {
	count := 1
	inQuote := false

	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			continue
		}

		if !inQuote && strings.HasPrefix(line[i:], sep) {
			count++
			i += len(sep) - 1
		}
	}

	return count
}

func (s *sshClientPool) splitLine(line string, sep string) []string {
	var result []string
	var field strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			continue
		}

		if !inQuote && strings.HasPrefix(line[i:], sep) {
			result = append(result, field.String())
			field.Reset()
			i += len(sep) - 1
			continue
		}

		field.WriteByte(line[i])
	}

	result = append(result, field.String())

	return result
}

func (s *sshClientPool) splitLineKeepQuote(line string, sep string) []string {
	var result []string
	var field strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			field.WriteByte(line[i])
			continue
		}

		if !inQuote && strings.HasPrefix(line[i:], sep) {
			result = append(result, field.String())
			field.Reset()
			i += len(sep) - 1
			continue
		}

		field.WriteByte(line[i])
	}

	result = append(result, field.String())

	return result
}

func (s *sshClientPool) RunQuery(ctx context.Context, arg *Args) ([][]any, error) {
	if s.isCloseFlag.Load() {
		return nil, errlist.ErrG.NewError(nil, "already close ssh conn")
	}

	if sess, err := s.getSession(); err != nil {
		return nil, err
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
				return nil, errlist.ErrG.NewError(err, "cmd err:%s", errB.String())
			}
			return nil, errlist.ErrG.NewError(err, "ssh err")
		} else if errB.Len() > 0 {
			return nil, errlist.ErrG.NewError(nil, "cmd err:%s", errB.String())
		}

		lines := s.splitLineKeepQuote(b.String(), s.newlineChar)
		var res = make([][]any, len(lines))
		var max = 0

		if s.splitChar != "" {
			for _, line := range lines {
				if cnt := s.countFields(line, s.splitChar); max < cnt {
					max = cnt
				}
			}

			if max <= 0 {
				max = 1
			}

			for idx, line := range lines {
				res[idx] = make([]any, max)
				for didx, data := range s.splitLine(line, s.splitChar) {
					res[idx][didx] = data
				}
			}
		} else {
			for idx, line := range lines {
				res[idx] = make([]any, 1)
				res[idx][0] = line
			}
		}

		return res, nil
	}
}
func (s *sshClientPool) Close() error {
	if s.isCloseFlag.Swap(true) {
		return errlist.ErrG.NewError(nil, "already close ssh conn")
	}

	return nil
}
