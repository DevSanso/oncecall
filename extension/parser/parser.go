package parser

import (
	"oncecall/errset"

	"github.com/pelletier/go-toml"
)

type ParserExtension[T any] interface {
	Parse(data []byte, out *T) error
}

type TomlParserExtension[T any] struct {
}

func (TomlParserExtension[T]) Parse(data []byte, out *T) error {
	if err := toml.Unmarshal(data, out); err != nil {
		return errset.ErrG.NewError(err, "Toml Constructor failed")
	}
	return nil
}
