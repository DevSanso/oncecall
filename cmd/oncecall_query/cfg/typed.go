package cfg

import "oncecall/conn"

type ConnConfig = conn.ConnConfig

type ScriptQueryPlan struct {
	Bind map[int]struct {
		Dynamic *struct {
			PlanIdx string `toml:"plan_idx"`
			Col     string `toml:"col"`
		} `toml:"dynamic"`

		Static *string `toml:"static"`
	} `toml:"bind"`

	Vm *struct {
		Lang   string `toml:"lang"`
		Script string `toml:"script"`
	} `toml:"vm"`

	Tran  bool   `toml:"is_tran"`
	Query string `toml:"query"`

	IsNext *struct {
		Query struct {
			DB    string `toml:"db"`
			Query string `toml:"query"`
		}
	} `toml:"is_next"`
}

type ScriptConfig struct {
	Name string `toml:"name"`

	Interval struct {
		Sec int `toml:"sec"`
	} `toml:"interval"`

	//key : dbname
	Init map[string]struct {
		Query        []string `toml:"query"`
		TriggerQuery string   `toml:"trigger"`
	} `toml:"init"`

	Plan map[string]ScriptQueryPlan `toml:"plan"`
}

type ProcessConfig struct {
	Version  int        `toml:"version"`
	ManageDB ConnConfig `toml:"manage"`

	Cmd struct {
		Db struct {
			Query struct {
				DbList       string `toml:"db_list"`
				DbOption     string `toml:"db_option"`
				DbScriptLink string `toml:"db_script_link"`
				Script       string `toml:"script"`
			} `toml:"query"`
		} `toml:"db"`
	} `toml:"cmd"`
}
