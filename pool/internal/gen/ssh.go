package gen

import (
	"context"
	"oncecall/errlist"
	"oncecall/errlist/prefix"
	"oncecall/extension/log"
	"oncecall/pool/internal/connection"
	"oncecall/pool/types"

	"golang.org/x/crypto/ssh"
)

type sshGenerator struct {
	info   types.ConnConfig
	client *ssh.Client

	newlineChar string
	splitChar   string
}

func NewSSHGenerator(info types.ConnConfig, lExtension log.LoggerLogExtension[any]) (Generator, error) {
	var exists bool
	var outputSplitCharOpt, outputNewlineCharOpt any

	outputSplitCharOpt, exists = info.OptionMap["output.splitchar"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "output.splitchar not exists")
	}
	outputNewlineCharOpt, exists = info.OptionMap["output.newline"]
	if !exists {
		return nil, errlist.ErrG.NewError(prefix.NotExistsError, "output.newline not exists")
	}

	return &sshGenerator{
		info:        info,
		newlineChar: outputSplitCharOpt.(string),
		splitChar:   outputNewlineCharOpt.(string),
	}, nil

}

func (s *sshGenerator) Gen(ctx context.Context, l log.LoggerDebugExtension[any]) (types.Conn, error) {
	sess, sessErr := s.client.NewSession()
	if sessErr != nil {
		return nil, errlist.ErrG.NewError(sessErr, "ssh get failed")
	}

	return connection.NewSshConn(sess, s.newlineChar, s.splitChar), nil
}
