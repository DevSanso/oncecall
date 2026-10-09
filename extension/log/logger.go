package log

import (
	"io"
	"oncecall/utils"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggerDebugExtension[PARAM any] interface {
	Debug(msg string, param ...PARAM)
}

type LoggerLogExtension[PARAM any] interface {
	LoggerDebugExtension[PARAM]
	Info(msg string, param ...PARAM)
	Warn(msg string, param ...PARAM)
	Error(msg string, param ...PARAM)
}

type LoggerExtension[PARAM any] interface {
	LoggerLogExtension[PARAM]
	io.Closer
}

type ZapSugaredLoggerExtension struct {
	Dir     string
	Name    string
	Level   string
	MaxSize int64
	BackUp  int

	l       *zap.SugaredLogger
	lock    sync.Mutex
	once    sync.Once
	closeFn func()
}

func (*ZapSugaredLoggerExtension) getLevelFromArgs(level string) zapcore.Level {
	switch level {
	case "info":
		return zap.InfoLevel
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	default:
		return zap.ErrorLevel
	}
}
func (z *ZapSugaredLoggerExtension) initGetFile(logDir, name, level string, maxsize int64, backup int) (*utils.RotateWriter, zap.LevelEnablerFunc, error) {
	f, fErr := utils.NewRotateWriter(filepath.Join(logDir, name+"."+level+".log"), maxsize*1024, backup)
	if fErr != nil {
		return nil, nil, fErr
	}

	return f, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl == z.getLevelFromArgs(level)
	}), nil

}
func (z *ZapSugaredLoggerExtension) init() {
	z.once.Do(func() {
		z.lock.Lock()
		defer z.lock.Unlock()

		if z.Dir == "" {
			var logConfig = zap.NewProductionConfig()
			logConfig.Level.SetLevel(z.getLevelFromArgs(z.Level))
			logConfig.EncoderConfig.TimeKey = "timestamp"
			logConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			logger, logErr := logConfig.Build()
			if logErr != nil {
				panic("ZapLogExtension Init Panic: " + logErr.Error())
			}

			z.l = logger.Sugar()
		} else {
			logDirAbs, pathErr := filepath.Abs(z.Dir)

			if pathErr != nil {
				panic("ZapLogExtension Init Panic: " + pathErr.Error())
			}

			stat, statErr := os.Stat(logDirAbs)
			if statErr != nil {
				panic("ZapLogExtension Init Panic: " + statErr.Error())
			}

			if !stat.IsDir() {
				panic("ZapLogExtension Init Panic: " + "not dir=" + logDirAbs)
			}

			debugF, debugCond, debugErr := z.initGetFile(logDirAbs, z.Name, "debug", z.MaxSize, z.BackUp)
			if debugErr != nil {
				panic("ZapLogExtension Init Panic: " + debugErr.Error())
			}
			infoF, infoCond, infoErr := z.initGetFile(logDirAbs, z.Name, "info", z.MaxSize, z.BackUp)
			if infoErr != nil {
				_ = infoF.Close()
				panic("ZapLogExtension Init Panic: " + infoErr.Error())
			}
			warnF, warnCond, warnErr := z.initGetFile(logDirAbs, z.Name, "warn", z.MaxSize, z.BackUp)
			if warnErr != nil {
				_ = infoF.Close()
				_ = warnF.Close()
				panic("ZapLogExtension Init Panic: " + warnErr.Error())
			}
			errorF, errorCond, errorErr := z.initGetFile(logDirAbs, z.Name, "error", z.MaxSize, z.BackUp)
			if errorErr != nil {
				_ = infoF.Close()
				_ = warnF.Close()
				_ = errorF.Close()
				panic("ZapLogExtension Init Panic: " + errorErr.Error())
			}
			z.closeFn = func() {
				_ = debugF.Close()
				_ = infoF.Close()
				_ = warnF.Close()
				_ = errorF.Close()
			}
			encoderConfig := zap.NewProductionEncoderConfig()
			encoderConfig.TimeKey = "timestamp"
			encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

			encoder := zapcore.NewJSONEncoder(encoderConfig)

			core := zapcore.NewTee(
				zapcore.NewCore(
					encoder,
					zapcore.AddSync(debugF),
					debugCond,
				),
				zapcore.NewCore(
					encoder,
					zapcore.AddSync(infoF),
					infoCond,
				),
				zapcore.NewCore(
					encoder,
					zapcore.AddSync(warnF),
					warnCond,
				),
				zapcore.NewCore(
					encoder,
					zapcore.AddSync(errorF),
					errorCond,
				),
			)
			l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
			z.l = l.Sugar()
		}
	})
}
func (z *ZapSugaredLoggerExtension) Debug(msg string, param ...any) {
	z.init()
	z.l.Debug(msg, param)
}

func (z *ZapSugaredLoggerExtension) Error(msg string, param ...any) {
	z.init()
	z.l.Error(msg, param)
}

func (z *ZapSugaredLoggerExtension) Info(msg string, param ...any) {
	z.init()
	z.l.Info(msg, param)
}

func (z *ZapSugaredLoggerExtension) Warn(msg string, param ...any) {
	z.init()
	z.l.Warn(msg, param)
}

func (z *ZapSugaredLoggerExtension) Close() error {
	_ = z.l.Sync()
	z.closeFn()
	return nil
}
