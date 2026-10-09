package cfg

import (
	"oncecall/pool/types"
)

type ConnConfig = types.ConnConfig

type ScriptQuerySyncPlan struct {
	Bind struct {
		Fix *struct {
			//key: cols pos
			Pos map[string]struct {
				Dynamic *struct {
					PlanIdx         int    `toml:"plan_idx"`
					Col             string `toml:"col"`
					StartSyncOffset int    `toml:"start_sync_offset"`
				} `toml:"dynamic"`

				Static *string `toml:"static"`
			}
		} `toml:"fix"`

		Vm *struct {
			Lang   string `toml:"lang"`
			Script string `toml:"script"`
		} `toml:"vm"`
	} `toml:"bind"`

	Tran  bool   `toml:"is_tran"`
	Query string `toml:"query"`
}

type ScriptQueryReadPlan struct {
	Bind struct {
		//key: cols pos
		Fix map[string]struct {
			Dynamic *struct {
				PlanIdx int    `toml:"plan_idx"`
				Col     string `toml:"col"`
				RowIdx  int    `toml:"row"`
			} `toml:"dynamic"`
			Static *string `toml:"static"`
		} `toml:"fix"`

		Vm *struct {
			Lang   string `toml:"lang"`
			Script string `toml:"script"`
		} `toml:"vm"`
	} `toml:"bind"`

	Tran  bool   `toml:"is_tran"`
	Query string `toml:"query"`

	IsNext *struct {
		Query string `toml:"query"`
	} `toml:"is_next"`
}

type ScriptConfig struct {
	Interval struct {
		Sec int `toml:"sec"`
	} `toml:"interval"`

	Init *struct {
		Self []struct {
			Query        string `toml:"query"`
			TriggerQuery string `toml:"trigger"`
		} `toml:"self"`

		//key : identifier
		Other map[string]struct {
			Query        []string `toml:"query"`
			TriggerQuery string   `toml:"trigger"`
		} `toml:"other"`
	} `toml:"init"`

	Option *struct {
		PlanRowBufCap      int `toml:"plan_buf_cap"`
		PlanColsBufAlloc   int `toml:"plan_cols_buf_alloc"`
		PlanRowBufIdleTime int `toml:"plan_buf_idle_time_sec"`
	} `toml:"option"`

	Plans struct {
		Read []struct {
			ReadIdent string              `toml:"identifier"`
			ReadPlan  ScriptQueryReadPlan `toml:"read_plan"`
		} `toml:"read"`
		Sync ScriptQuerySyncPlan `toml:"sync"`
	} `toml:"plan"`
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