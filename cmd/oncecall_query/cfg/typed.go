package cfg

import "oncecall/conn"

type ConnConfig = conn.ConnConfig

type ScriptQuerySyncPlan struct {
	Bind *struct {
		Size struct {
			Col int `toml:"col"`
			Row int `toml:"row"`
		} `toml:"size"`

		//key: cols pos
		Pos map[int]struct {
			Dynamic *struct {
				PlanIdx         int    `toml:"plan_idx"`
				Col             string `toml:"col"`
				StartSyncOffset int    `toml:"start_sync_offset"`
			} `toml:"dynamic"`

			Static *string `toml:"static"`
		}
	} `toml:"bind"`

	Vm *struct {
		Lang   string `toml:"lang"`
		Script string `toml:"script"`
	} `toml:"vm"`

	Tran  bool   `toml:"is_tran"`
	Query string `toml:"query"`
}

type ScriptQueryReadPlan struct {
	//key: cols pos
	Bind map[int]struct {
		Dynamic *struct {
			PlanIdx int    `toml:"plan_idx"`
			Col     string `toml:"col"`
			RowIdx  int    `toml:"row"`
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
		Query string `toml:"query"`
	} `toml:"is_next"`
}

type ScriptConfig struct {
	Ident string `toml:"identifier"`

	Interval struct {
		Sec int `toml:"sec"`
	} `toml:"interval"`

	//key : identifier
	Init map[string]struct {
		Query        []string `toml:"query"`
		TriggerQuery string   `toml:"trigger"`
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
