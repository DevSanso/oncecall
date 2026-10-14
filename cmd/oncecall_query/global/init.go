package global

import (
	"flag"
	"net/http"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/extension/log"
	"oncecall/extension/parser"
	"oncecall/utils"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	_ "net/http/pprof"
)

var (
	cfgFile      = flag.String("cfgPath", "../cfg/oncecall.query.toml", "config file")
	cpu         = flag.Int("cpu", runtime.NumCPU()/2, "cpu core")

)

var (
	_once sync.Once
)

func Init() {
	_once.Do(func() {
		flag.Parse()
		runtime.GOMAXPROCS(*cpu)

		configData, configErr := os.ReadFile(*cfgFile)
		if configErr != nil {
			panic("Init :" + configErr.Error())
		}

		var reader parser.ParserExtension[cfg.ProcessConfig] = parser.TomlParserExtension[cfg.ProcessConfig]{}
		if readerErr := reader.Parse(configData, &Config); readerErr != nil {
			panic("Init :" + readerErr.Error())
		}

		if Config.Perf.Standard != nil {
			go func() {
				if err := http.ListenAndServe(Config.Perf.Standard.Address, nil); err != nil {
					panic("Init : " + err.Error())
				}
			}()
		}

		if Config.Log.File != nil {
			size,sizeParsingErr := utils.ParseMemorySize(Config.Log.File.MaxSize)
			if sizeParsingErr != nil {
				panic("Init : " + sizeParsingErr.Error())
			}

			procExec, execErr := os.Executable()
			if execErr != nil {
				panic("Init : " + execErr.Error())
			}

			logBaseName := filepath.Base(procExec) + "." + strconv.Itoa(os.Getpid())

			dir := Config.Log.File.Dir
			rotate := Config.Log.File.Rotate

			Log.ScriptLogExtension = &log.ZapSugaredFileLoggerExtension{
				Dir:     dir,
				Name:    logBaseName + ".script",
				Level:   Config.Log.Level,
				MaxSize: size,
				BackUp:  rotate,
			}

			Log.ConnLogExtension = &log.ZapSugaredFileLoggerExtension{
				Dir:     dir,
				Name:    logBaseName + ".conn",
				Level:   Config.Log.Level,
				MaxSize: size,
				BackUp:  rotate,
			}

			Log.ManageLogExtension = &log.ZapSugaredFileLoggerExtension{
				Dir:     dir,
				Name:    logBaseName + ".manage",
				Level:   Config.Log.Level,
				MaxSize: size,
				BackUp:  rotate,
			}

			Log.QueryLogExtension = &log.ZapSugaredFileLoggerExtension{
				Dir:     dir,
				Name:    logBaseName + ".query",
				Level:   Config.Log.Level,
				MaxSize: size,
				BackUp:  rotate,
			}

			Log.GenLogExtension = &log.ZapSugaredFileLoggerExtension{
				Dir:     dir,
				Name:    logBaseName + ".gen",
				Level:   Config.Log.Level,
				MaxSize: size,
				BackUp:  rotate,
			}
		}

		Log.ScriptLogExtension.Info("init success")
		Log.ConnLogExtension.Info("init success")
		Log.ScriptLogExtension.Info("init success")
		Log.QueryLogExtension.Info("init success")
		Log.GenLogExtension.Info("init success")
	})
}
