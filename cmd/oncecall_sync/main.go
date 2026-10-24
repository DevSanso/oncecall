package main

import (
	"context"
	_ "net/http/pprof"
	"oncecall/cmd/oncecall_sync/entry"
	"oncecall/cmd/oncecall_sync/global"
	"oncecall/cmd/oncecall_sync/global/config"
	"oncecall/cmd/oncecall_sync/global/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Log.Proc.Info("start: process")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	procCtx, cancel := context.WithCancel(context.Background())

	if config.SyncIdent == "" {
		go entry.MasterEntry(procCtx)
	} else {
		go entry.MasterEntry(procCtx)
	}

loop:
	for {
		select {
		case sig := <-sigs:
			log.Log.Proc.Info("signal: ", sig)
			break loop
		default:
			time.Sleep(1 * time.Second)
		}
	}

	log.Log.Proc.Info("Stop: process")
	cancel()
	global.GlobalClose()
}
