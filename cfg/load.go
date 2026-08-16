package cfg

import (
	"oncecall/utils"
	"os"

	"github.com/pelletier/go-toml"
)

func GetConfigFromToml(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, utils.ErrorfPc("%s", err)
	}
	ret := new(Config)
	if err = toml.Unmarshal(data, ret); err != nil {
		return nil, utils.ErrorfPc("%s", err)
	}
	return ret, nil
}
