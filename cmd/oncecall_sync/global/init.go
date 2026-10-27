package global

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"oncecall/cmd/oncecall_sync/cfg"
	"oncecall/cmd/oncecall_sync/global/config"
	log2 "oncecall/cmd/oncecall_sync/global/log"
	"oncecall/cmd/oncecall_sync/global/manage"
	"oncecall/cmd/oncecall_sync/global/metric"
	"oncecall/extension/log"
	"oncecall/extension/parser"
	"oncecall/pool"
	"oncecall/utils"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"

	_ "net/http/pprof"
)

var (
	cfgFile   = flag.String("cfgPath", "../cfg/oncecall.query.toml", "config file")
	cpu       = flag.Int("cpu", runtime.NumCPU()/2, "cpu core")
	syncIdent = flag.String("syncIdent", "", "sync conn ident")
)


var (
	_closer []io.Closer
)

func GlobalClose() {
	for _, closer := range _closer {
		_ = closer.Close()
		fmt.Println("close closer : %s", reflect.TypeOf(closer).Name())
	}
}

func init() {
	flag.Parse()
	runtime.GOMAXPROCS(*cpu)

	configData, configErr := os.ReadFile(*cfgFile)
	if configErr != nil {
		panic("Init :" + configErr.Error())
	}

	var reader parser.ParserExtension[cfg.ProcessConfig] = parser.TomlParserExtension[cfg.ProcessConfig]{}
	if readerErr := reader.Parse(configData, &config.Config); readerErr != nil {
		panic("Init :" + readerErr.Error())
	}

	if config.Config.Perf.Standard != nil {
		go func() {
			if err := http.ListenAndServe(config.Config.Perf.Standard.Address, nil); err != nil {
				panic("Init : " + err.Error())
			}
		}()
	}

	if config.Config.Log.File != nil {
		size, sizeParsingErr := utils.ParseMemorySize(config.Config.Log.File.MaxSize)
		if sizeParsingErr != nil {
			panic("Init : " + sizeParsingErr.Error())
		}

		procExec, execErr := os.Executable()
		if execErr != nil {
			panic("Init : " + execErr.Error())
		}

		logBaseName := ""
		if *syncIdent == "" {
			logBaseName = filepath.Base(procExec) + "." + strconv.Itoa(os.Getpid())
		} else {
			logBaseName = filepath.Base(procExec) + "." + *syncIdent + "." + strconv.Itoa(os.Getpid())
		}

		dir := config.Config.Log.File.Dir
		rotate := config.Config.Log.File.Rotate

		log2.Log.Script = &log.ZapSugaredFileLoggerExtension{
			Dir:     dir,
			Name:    logBaseName + ".script",
			Level:   config.Config.Log.Level,
			MaxSize: size,
			BackUp:  rotate,
		}

		log2.Log.Conn = &log.ZapSugaredFileLoggerExtension{
			Dir:     dir,
			Name:    logBaseName + ".conn",
			Level:   config.Config.Log.Level,
			MaxSize: size,
			BackUp:  rotate,
		}

		log2.Log.Manage = &log.ZapSugaredFileLoggerExtension{
			Dir:     dir,
			Name:    logBaseName + ".manage",
			Level:   config.Config.Log.Level,
			MaxSize: size,
			BackUp:  rotate,
		}

		log2.Log.Query = &log.ZapSugaredFileLoggerExtension{
			Dir:     dir,
			Name:    logBaseName + ".query",
			Level:   config.Config.Log.Level,
			MaxSize: size,
			BackUp:  rotate,
		}

		log2.Log.Gen = &log.ZapSugaredFileLoggerExtension{
			Dir:     dir,
			Name:    logBaseName + ".gen",
			Level:   config.Config.Log.Level,
			MaxSize: size,
			BackUp:  rotate,
		}
	}

	log2.Log.Manage.Info("init success")
	log2.Log.Conn.Info("init success")
	log2.Log.Script.Info("init success")
	log2.Log.Query.Info("init success")
	log2.Log.Gen.Info("init success")

	_closer = append(_closer, log2.Log.Conn)
	_closer = append(_closer, log2.Log.Script)
	_closer = append(_closer, log2.Log.Query)
	_closer = append(_closer, log2.Log.Gen)
	_closer = append(_closer, log2.Log.Manage)

	var managePErr error
	manage.ManageP, managePErr = pool.NewConnPool(
		context.Background(),
		"internalManage",
		config.Config.ManageDB,
		metric.Metric, log2.Log.Gen, log2.Log.Gen, log2.Log.Conn)

	if managePErr != nil {
		GlobalClose()
		panic("Init :" + managePErr.Error())
	}

	_closer = append(_closer, manage.ManageP)
}
