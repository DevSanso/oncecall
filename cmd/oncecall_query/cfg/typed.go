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

	Perf struct {
		Standard *struct {
			Address string
		}
	}

	Log struct {
		Level string

		File *struct {
			Dir     string
			MaxSize string
			Rotate  int
		}
	}

	Exec struct {
		LoopIntervalSec int
		UpdateTimeoutMs int
		FetchTimeoutMs  int
	}

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

var (
	DefaultProcessConfig = ProcessConfig{
		Version: 1,
		ManageDB: ConnConfig{
			Name:     "postgres",
			Id:       "postgres",
			Password: "postgres",
			Server:   "localhost:5432",
			DBType:   "postgres",
			MaxConn:  5,
		},

		Perf: struct{ Standard *struct{ Address string } }{
			Standard: &struct{ Address string }{
				Address: "localhost:8001",
			},
		},
		Log: struct {
			Level string
			File  *struct {
				Dir     string
				MaxSize string
				Rotate  int
			}
		}{
			Level: "debug",
			File: &struct {
				Dir     string
				MaxSize string
				Rotate  int
			}{
				Dir:     "../log",
				MaxSize: "100M",
				Rotate:  3,
			},
		},
		Exec: struct {
			LoopIntervalSec int
			UpdateTimeoutMs int
			FetchTimeoutMs  int
		}{
			LoopIntervalSec: 5,
			UpdateTimeoutMs: 1000,
			FetchTimeoutMs:  1000,
		},

		Cmd: struct {
			Db struct {
				Query struct {
					DbList       string "toml:\"db_list\""
					DbOption     string "toml:\"db_option\""
					DbScriptLink string "toml:\"db_script_link\""
					Script       string "toml:\"script\""
				} "toml:\"query\""
			} "toml:\"db\""
		}{
			Db: struct {
				Query struct {
					DbList       string "toml:\"db_list\""
					DbOption     string "toml:\"db_option\""
					DbScriptLink string "toml:\"db_script_link\""
					Script       string "toml:\"script\""
				} "toml:\"query\""
			}{
				Query: struct {
					DbList       string "toml:\"db_list\""
					DbOption     string "toml:\"db_option\""
					DbScriptLink string "toml:\"db_script_link\""
					Script       string "toml:\"script\""
				}{
					DbList:   "SELECT * FROM db_list where is_enable = true",
					DbOption: `SELECT opt.db_uuid, opt."key", opt.value FROM db_option opt where exists ( select 1 from db_list l where l.s_enable = true and l.uuid = opt.db_uuid)`,
					DbScriptLink: `SELECT link.script_name, link.db_uuid 
						FROM db_script_link link 
						WHERE link.is_use = true and exists ( select 1 from db_list l where l.s_enable = true and l.uuid = link.db_uuid)`,
					Script: "SELECT script_name, script_data from scripts",
				},
			},
		},
	}
)
