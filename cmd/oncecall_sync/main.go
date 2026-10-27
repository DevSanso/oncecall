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
		go func() {
			entry.MasterEntry(procCtx)
			cancel()
		}()
	} else {
		go func() {
			entry.SyncdEntry(procCtx)
			cancel()
		}()
	}

loop:
	for {
		select {
		case sig := <-sigs:
			log.Log.Proc.Info("signal: ", sig)
			break loop
		case <-procCtx.Done():
			log.Log.Proc.Info("process entry terminated")
			break loop
		default:
			time.Sleep(1 * time.Second)
		}
	}

	log.Log.Proc.Info("Stop: process")
	cancel()
	global.GlobalClose()
}
