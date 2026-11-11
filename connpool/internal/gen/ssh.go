package gen

import (
	"context"
	"oncecall/connpool/internal/connection"
	"oncecall/connpool/types"
	"oncecall/errset"
	"oncecall/errset/prefix"
	"oncecall/extension/log"

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
		return nil, errset.ErrG.NewError(prefix.NotExistsError, "output.splitchar not exists")
	}
	outputNewlineCharOpt, exists = info.OptionMap["output.newline"]
	if !exists {
		return nil, errset.ErrG.NewError(prefix.NotExistsError, "output.newline not exists")
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
		return nil, errset.ErrG.NewError(sessErr, "ssh get failed")
	}

	return connection.NewSshConn(sess, s.newlineChar, s.splitChar), nil
}
