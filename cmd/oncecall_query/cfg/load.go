package cfg

import (
	"oncecall/errlist"
	"os"
	"unsafe"

	"github.com/pelletier/go-toml"
)

func GetManageConfTomlFromFile(path string) (*ProcessConfig, error) {
	data, err := os.ReadFile(path)
	ret := &ProcessConfig{}
	if err != nil {
		return nil, errlist.ErrG.NewError(err, "read config file:%s", path)
	}

	if err = toml.Unmarshal(data, ret); err != nil {
		return nil, errlist.ErrG.NewError(err, "unmarshal config:%s", path)
	}
	return ret, nil
}

func GetScriptConfigTomlFromData(data string) (*ScriptConfig, error) {
	ptr := unsafe.Slice(unsafe.StringData(data), len(data))
	ret := &ScriptConfig{}
	if err := toml.Unmarshal(ptr, ret); err != nil {
		return nil, errlist.ErrG.NewError(err, "unmarshal %s", data)
	}
	return ret, nil
}
