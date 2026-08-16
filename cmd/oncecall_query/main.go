package main

import (
	"context"
	"flag"
	"fmt"
	_ "net/http/pprof"
	gcfg "oncecall/cfg"
	"oncecall/cmd/oncecall_query/cfg"
	"oncecall/cmd/oncecall_query/executor"
	"oncecall/utils"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"oncecall/initialize"

	"go.uber.org/zap"
)

var (
	cfgDir     = flag.String("cfgpath", "../cfg", "config Dir")
	jobDirPath = flag.String("jobdir", "../job", "script job dir")
)

func getConfig() (app *gcfg.Config, manageConf *cfg.ProcessConfig, self []cfg.ScriptSelfConfig, err error) {

	appConfFile := filepath.Join(*cfgDir, "common.toml")
	appConf, appConfErr := gcfg.GetConfigFromToml(appConfFile)
	if appConfErr != nil {
		return nil, nil, nil, appConfErr
	} else {
		app = appConf
	}

	procConfFile := filepath.Join(*cfgDir, "oncecall.query.toml")
	queryConf, queryConfErr := cfg.GetManageConfFromToml(procConfFile)
	if queryConfErr != nil {
		return nil, nil, nil, queryConfErr
	} else {
		manageConf = queryConf
	}

	entry, readDirErr := os.ReadDir(*jobDirPath)
	if readDirErr != nil {
		return nil, nil, nil, utils.ErrorfPc("%s", readDirErr.Error())
	}

	self = make([]cfg.ScriptSelfConfig, 0, 5)

	for _, f := range entry {
		if f.IsDir() {
			continue
		}

		if filepath.Ext(f.Name()) != ".toml" {
			continue
		}

		scriptConf, scriptConfErr := cfg.GetScriptFromToml(filepath.Join(*jobDirPath, f.Name()))
		if scriptConfErr != nil {
			return nil, nil, nil, scriptConfErr
		}
		self = append(self, scriptConf)
	}

	return
}

func main() {
	deferFn, err := initialize.InitProc()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer deferFn()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	execCtx, cancelFn := context.WithCancel(context.Background())

	if conf, procConf, selfScriptData, err := getConfig(); err != nil {
		zap.L().Error(err.Error())
		return
	} else {
		exec := executor.NewSelfExecutor(conf, procConf, selfScriptData)

		go func(e *executor.SelfExecutor, ctx context.Context) {
			if execErr := e.Run(ctx); execErr != nil {
				zap.L().Error(execErr.Error())
			}
			cancelFn()
		}(exec, execCtx)
	}

mainLoop:
	for {
		select {
		case signo := <-sigs:
			zap.L().Info("get signal", zap.String("signal", signo.String()))
			fmt.Println("get signal : ", signo.String())
			cancelFn()
			continue
		case <-execCtx.Done():
			zap.L().Info("stop main process, wait 3 second")
			fmt.Println("stop main process, wait 3 second")
			time.Sleep(3 * time.Second)
			break mainLoop
		default:
		}
		time.Sleep(3 * time.Second)
	}

	zap.L().Info("main function end")
}
