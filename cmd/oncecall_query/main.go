package main

import (
	"context"
	"flag"
	"fmt"
	_ "net/http/pprof"
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
	cfgDir          = flag.String("cfgPath", "../cfg", "config Dir")
	updateTimeoutMs = flag.Int("updateTimeoutMs", 10000, "exec Update timeout ms")
	fetchTimeoutMs  = flag.Int("fetchTimeoutMs", 10000, "exec fetch timeout ms")
	loopIntervalSec = flag.Int("loopIntervalSec", 5, "exec loop interval sec")
)

func readProcConfig() (*cfg.ProcessConfig, error) {
	return cfg.GetManageConfTomlFromFile(filepath.Join(*cfgDir, "oncecall.query.toml"))
}

func main() {
	deferFn, err := initialize.InitProc()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer deferFn()

	procCfg, cfgErr := readProcConfig()
	if cfgErr != nil {
		fmt.Println(cfgErr.Error())
		return
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	execCtx, cancelFn := context.WithCancel(context.Background())
	thExec := executor.NewExecutor(execCtx, procCfg)

	sleepTime := time.Duration(*loopIntervalSec) * time.Second
	sleepOffset := 10 * time.Millisecond

mainLoop:
	for {
		utils.IntervalSleep(sleepTime, sleepOffset)

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
			if err := thExec.Update(*updateTimeoutMs); err != nil {
				zap.L().Error("exec update error", zap.Error(err))
				continue
			}

			if err := thExec.Fetch(*fetchTimeoutMs); err != nil {
				zap.L().Error("exec fetch error", zap.Error(err))
				continue
			}

			if err := thExec.DisPatch(); err != nil {
				zap.L().Error("exec dispatch error", zap.Error(err))
				continue
			}
		}
	}

	zap.L().Info("main function end")
}
