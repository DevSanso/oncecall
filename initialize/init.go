package initialize

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"oncecall/errlist"
	"oncecall/utils"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	cpu         = flag.Int("cpu", runtime.NumCPU()/2, "cpu core")
	logLevel    = flag.String("loglevel", "debug", "loglevel")
	logDir      = flag.String("logfile", "../log", "logdir")
	logMaxSize  = flag.String("logmaxsize", "20M", "logmaxsize")
	logRotate   = flag.Int("logrotate", 3, "logrotate")
	perfAddress = flag.String("perf", "", "perf address")
)

func getLevelFromArgs() zapcore.Level {
	switch *logLevel {
	case "info":
		return zap.InfoLevel
	case "debug":
		return zap.DebugLevel
	case "wran":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.FatalLevel
	}
}

func setLogger() (deferFn func(), err error) {
	if logDir == nil {
		var logConfig = zap.NewProductionConfig()
		logConfig.Level.SetLevel(getLevelFromArgs())
		logConfig.EncoderConfig.TimeKey = "timestamp"
		logConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, logErr := logConfig.Build()
		if logErr != nil {
			return nil, errlist.ErrG.NewError(logErr, "config load failed", "")
		}
		zap.ReplaceGlobals(logger)
		return nil, nil
	}
	logDirAbs, pathErr := filepath.Abs(*logDir)

	if pathErr != nil {
		return nil, errlist.ErrG.NewError(pathErr, "absolute path failed : %s", *logDir)
	}

	stat, statErr := os.Stat(logDirAbs)
	if statErr != nil {
		return nil, errlist.ErrG.NewError(statErr, "logdir stat failed : %s", logDirAbs)
	}

	if !stat.IsDir() {
		return nil, errlist.ErrG.NewError(nil, "is not directory %s", stat.Name())
	}

	maxSize, maxSizeErr := utils.ParseMemorySize(*logMaxSize)
	if maxSizeErr != nil {
		return nil, errlist.ErrG.NewError(maxSizeErr, "parse memory size failed : %s", *logMaxSize)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	processName := strings.ReplaceAll(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]), "")

	errorFile, errfileErr := utils.NewRotateWriter(filepath.Join(logDirAbs, processName+".error.log"), maxSize, *logRotate)
	if errfileErr != nil {
		return nil, errlist.ErrG.NewError(errfileErr, "err rotate log file failed : %s", processName)
	}

	infoFile, infoFileErr := utils.NewRotateWriter(filepath.Join(logDirAbs, processName+".info.log"), maxSize, *logRotate)
	if infoFileErr != nil {
		return nil, errlist.ErrG.NewError(infoFileErr, "info rotate log file failed : %s", processName)
	}

	debugFile, debugFileErr := utils.NewRotateWriter(filepath.Join(logDirAbs, processName+".debug.log"), maxSize, *logRotate)
	if debugFileErr != nil {
		return nil, errlist.ErrG.NewError(debugFileErr, "debug rotate log file failed : %s", processName)
	}

	allFile, allFileErr := utils.NewRotateWriter(filepath.Join(logDirAbs, processName+".all.log"), maxSize, *logRotate)
	if allFileErr != nil {
		return nil, errlist.ErrG.NewError(allFileErr, "all rotate log file failed : %s", processName)
	}

	deferFn = func() {
		_ = infoFile.Close()
		_ = debugFile.Close()
		_ = allFile.Close()
		_ = errorFile.Close()
		_ = zap.L().Sync()
	}
	err = nil

	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.InfoLevel &&
			lvl < zapcore.ErrorLevel
	})

	debugLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.DebugLevel &&
			lvl < zapcore.InfoLevel
	})

	allLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return true
	})

	core := zapcore.NewTee(
		zapcore.NewCore(
			encoder,
			zapcore.AddSync(allFile),
			allLevel,
		),
		zapcore.NewCore(
			encoder,
			zapcore.AddSync(errorFile),
			errorLevel,
		),
		zapcore.NewCore(
			encoder,
			zapcore.AddSync(infoFile),
			infoLevel,
		),
		zapcore.NewCore(
			encoder,
			zapcore.AddSync(debugFile),
			debugLevel,
		),
	)

	logger := zap.New(
		core,
		zap.AddCaller(),
	)
	zap.ReplaceGlobals(logger)

	return
}

func setPerf() {
	if *perfAddress == "" {
		return
	}

	go func() {
		if err := http.ListenAndServe(*perfAddress, nil); err != nil {
			zap.L().Error(err.Error())
			zap.L().Sync()
			os.Exit(2)
		}
	}()
}

func InitProc() (deferFn func(), err error) {
	flag.Parse()
	deferFn, err = setLogger()
	setPerf()
	runtime.GOMAXPROCS(*cpu)
	return
}
