package cfg

type ConnConfig struct {
	//stateless
	DBType string `toml:"dbtype"`
	Name   string `toml:"name"`

	Server   string `toml:"server"`
	Id       string `toml:"id"`
	Password string `toml:"password"`

	MaxConn   int            `toml:"max_conn"`
	OptionMap map[string]any `toml:"option"`

	//stateful
	DataMap map[string]any
}

type Config struct {
	Version  int        `toml:"version"`
	ManageDB ConnConfig `toml:"manage"`

	CollectDB   *ConnConfig `toml:"collect"`
	RealTimetDB *ConnConfig `toml:"realtime"`
}
