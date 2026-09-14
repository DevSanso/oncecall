package main

import (
	"context"
	"flag"
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oncecall/initialize"

	"go.uber.org/zap"
)

var (
	cfgDir     = flag.String("cfgpath", "../cfg", "config Dir")
	jobDirPath = flag.String("jobdir", "../job", "script job dir")
)

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
