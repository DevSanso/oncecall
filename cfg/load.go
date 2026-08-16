package cfg

import (
	"oncecall/errlist"
	"os"

	"github.com/pelletier/go-toml"
)

func GetConfigFromToml(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errlist.ErrG.NewError(err, "read file:%s", path)
	}
	ret := new(Config)
	if err = toml.Unmarshal(data, ret); err != nil {
		return nil, errlist.ErrG.NewError(err, "unmarshal config:%s", path)
	}
	return ret, nil
}
