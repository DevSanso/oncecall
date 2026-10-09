package main

import (
	"context"
	"fmt"
	_ "net/http/pprof"
	"oncecall/cmd/oncecall_query/executor"
	"oncecall/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
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
