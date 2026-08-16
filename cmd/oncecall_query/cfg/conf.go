package cfg

import (
	"oncecall/utils"
	"os"

	"github.com/pelletier/go-toml"
)

type ScriptSelfConfig struct {
	Name     string `toml:"name"`
	IsEanble bool   `toml:"is_enable"`

	Interval struct {
		Sec int `toml:"sec"`
	} `toml:"interval"`

	Init *struct {
		Realtime *struct {
			Query        []string `toml:"query"`
			TriggerQuery string   `toml:"trigger"`
		} `toml:"realtime"`
		Collect *struct {
			Query        []string `toml:"query"`
			TriggerQuery string   `toml:"trigger"`
		} `toml:"collect"`
	} `toml:"init"`

	IsTranscation struct {
		Set bool `toml:"set"`
		Get bool `toml:"get"`
	} `toml:"is_tran"`

	GetTrigger *struct {
		Query       string   `toml:"query"`
		DynamicBind []string `toml:"dynamic_bind"`
	} `toml:"trigger"`

	Get struct {
		Query       string   `toml:"query"`
		DynamicBind []string `toml:"dynamic_bind"`
		Script      *string  `toml:"script"`
	} `toml:"get"`

	Set struct {
		RealTime *struct {
			Query string `toml:"query"`
		} `toml:"realtime"`
		Collect *struct {
			Query string `toml:"query"`
		} `toml:"collect"`
	} `toml:"set"`
}

type ProcessConfig struct {
	Cmd struct {
		Db struct {
			Query struct {
				DbList   string `toml:"dblist"`
				DbOption string `toml:"db_option"`
			} `toml:"query"`
		} `toml:"db"`
	} `toml:"cmd"`
}

func GetScriptFromToml(path string) (ScriptSelfConfig, error) {
	data, err := os.ReadFile(path)
	ret := ScriptSelfConfig{}
	if err != nil {
		return ret, utils.ErrorfPc("%s", err)
	}

	if err = toml.Unmarshal(data, &ret); err != nil {
		return ret, utils.ErrorfPc("%s", err)
	}
	return ret, nil
}

func GetManageConfFromToml(path string) (*ProcessConfig, error) {
	data, err := os.ReadFile(path)
	ret := &ProcessConfig{}
	if err != nil {
		return nil, utils.ErrorfPc("%s", err)
	}

	if err = toml.Unmarshal(data, ret); err != nil {
		return nil, utils.ErrorfPc("%s", err)
	}
	return ret, nil
}
