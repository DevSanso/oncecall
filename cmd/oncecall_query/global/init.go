package global

import (
	"flag"
	"net/http"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/extension/log"
	"oncecall/utils"
	"os"
	"path/filepath"
	"runtime"

	_ "net/http/pprof"
)

var (
	cfgDir          = flag.String("cfgPath", "../cfg", "config Dir")
	loopSec = flag.Int("loopSec", 5, "exec loop interval sec")
	updateMs = flag.Int("updateMs", 100, "exec update timeout ms")
	fetchMs = flag.Int("fetchMs", 100, "exec fetch timeout ms")
	cpu             = flag.Int("cpu", runtime.NumCPU()/2, "cpu core")
	logLevel        = flag.String("loglevel", "debug", "loglevel")
	logDir          = flag.String("logfile", "../log", "logdir")
	logSize         = flag.String("logsize", "20M", "logsize")
	logRotate       = flag.Int("logrotate", 3, "logrotate")
	perfAddress     = flag.String("perf", "", "perf address")
)

func Init() {
	flag.Parse()
	runtime.GOMAXPROCS(*cpu)
	if *perfAddress != "" {
		go func() {
			if err := http.ListenAndServe(*perfAddress, nil); err != nil {
				panic("Init : "+err.Error())
			}
		}()
	}

	Config.ProcParam.LoopSec = *loopSec
	Config.ProcParam.UpdateTimeoutMs = *updateMs
	Config.ProcParam.FetchTimeoutMs = *fetchMs

	var confErr error
	Config.File, confErr = cfg.GetManageConfTomlFromFile(filepath.Join(*cfgDir, "oncecall.query.toml"))
	if confErr != nil {
		panic("Init : " + confErr.Error())
	}

	logSizeNum, parsingErr := utils.ParseMemorySize(*logSize)
	if parsingErr != nil {
		panic("Init : " + parsingErr.Error())
	}

	procExec, execErr := os.Executable()
	if execErr != nil {
		panic("Init : " + execErr.Error())
	}

	procName := filepath.Base(procExec)

	Log.ScriptLogExtension = &log.ZapSugaredLoggerExtension{
		Dir:     *logDir,
		Name:    procName + ".script",
		Level:   *logLevel,
		MaxSize: logSizeNum,
		BackUp:  *logRotate,
	}

	Log.ConnLogExtension = &log.ZapSugaredLoggerExtension{
		Dir:     *logDir,
		Name:    procName + ".conn",
		Level:   *logLevel,
		MaxSize: logSizeNum,
		BackUp:  *logRotate,
	}

	Log.ExecuteLogExtension = &log.ZapSugaredLoggerExtension{
		Dir:     *logDir,
		Name:    procName + ".exec",
		Level:   *logLevel,
		MaxSize: logSizeNum,
		BackUp:  *logRotate,
	}

	Log.RawLogExtension = &log.ZapSugaredLoggerExtension{
		Dir:     *logDir,
		Name:    procName + ".script",
		Level:   *logLevel,
		MaxSize: logSizeNum,
		BackUp:  *logRotate,
	}

	Log.ScriptLogExtension.Debug("init success")
	Log.ScriptLogExtension.Debug("init success")
	Log.ScriptLogExtension.Info("init success")
	Log.RawLogExtension.Info("init success")
}
