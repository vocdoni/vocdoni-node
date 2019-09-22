package log

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.SugaredLogger

//InitLogger initializes the logger. Output can be either "stdout/stderr/filePath"
func InitLogger(logLevel string, output string) {
	newLogger(logLevel, output)
}

//GetLogger returns a logger object
func getCurrentLogger() *zap.SugaredLogger {
	return log
}

func getLevelFromString(logLevel string) zapcore.Level {
	switch logLevel {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "dpanic":
		return zap.DPanicLevel
	case "panic":
		return zap.PanicLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

func newConfig(logLevel, output string) zap.Config {
	var encoderCfg = zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:  "ts",
		LevelKey: "level",
		//	NameKey:        "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
		EncodeTime: func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(ts.Local().Format(time.RFC3339))
		},
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	var cfg = zap.Config{
		Level:       zap.NewAtomicLevelAt(getLevelFromString(logLevel)),
		Development: false,
		Encoding:    "console",
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		EncoderConfig:    encoderCfg,
		OutputPaths:      []string{output},
		ErrorOutputPaths: []string{output},
	}
	return cfg
}

//Creates a new logger object with debug option
func newLogger(logLevel, output string) {
	cfg := newConfig(logLevel, output)

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	withOptions := logger.WithOptions(zap.AddCallerSkip(1))
	log = withOptions.Sugar()
	log.Infof("logger construction succeeded at level %s and output %s", logLevel, output)
}

//Debug sends a debug level log message
func Debug(args ...interface{}) {
	getCurrentLogger().Debug(args...)
}

//Info sends an info level log message
func Info(args ...interface{}) {
	getCurrentLogger().Info(args...)
}

//Warn sends a warn level log message
func Warn(args ...interface{}) {
	getCurrentLogger().Warn(args...)
}

//Error sends an error level log message
func Error(args ...interface{}) {
	getCurrentLogger().Error(args...)
}

//DPanic sends a dpanic level log message
func DPanic(args ...interface{}) {
	getCurrentLogger().DPanic(args...)
}

//Panic sends a panic level log message
func Panic(args ...interface{}) {
	getCurrentLogger().Panic(args...)
}

//Fatal sends a fatal level log message
func Fatal(args ...interface{}) {
	getCurrentLogger().Fatal(args...)
}

//Debugf sends a formatted debug level log message
func Debugf(template string, args ...interface{}) {
	getCurrentLogger().Debugf(template, args...)
}

//Infof sends a formatted info level log message
func Infof(template string, args ...interface{}) {
	getCurrentLogger().Infof(template, args...)
}

//Warnf sends a formatted warn level log message
func Warnf(template string, args ...interface{}) {
	getCurrentLogger().Warnf(template, args...)
}

//Errorf sends a formatted error level log message
func Errorf(template string, args ...interface{}) {
	getCurrentLogger().Errorf(template, args...)
}

//DPanicf sends a formatted dpanic level log message
func DPanicf(template string, args ...interface{}) {
	getCurrentLogger().DPanicf(template, args...)
}

//Panicf sends a formatted panic level log message
func Panicf(template string, args ...interface{}) {
	getCurrentLogger().Panicf(template, args...)
}

//Fatalf sends a formatted fatal level log message
func Fatalf(template string, args ...interface{}) {
	getCurrentLogger().Fatalf(template, args...)
}

//Debugw sends a key-value formatted debug level log message
func Debugw(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().Debugw(msg, keysAndValues...)
}

//Infow sends a key-value formatted info level log message
func Infow(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().Infow(msg, keysAndValues...)
}

//Warnw sends a key-value formatted warn level log message
func Warnw(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().Warnw(msg, keysAndValues...)
}

//Errorw sends a key-value formatted error level log message
func Errorw(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().Errorw(msg, keysAndValues...)
}

//DPanicw sends a key-value formatted dpanic level log message
func DPanicw(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().DPanicw(msg, keysAndValues...)
}

//Panicw sends a key-value formatted panic level log message
func Panicw(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().Panicw(msg, keysAndValues...)
}

//Fatalw sends a key-value formatted fatal level log message
func Fatalw(msg string, keysAndValues ...interface{}) {
	getCurrentLogger().Fatalw(msg, keysAndValues...)
}
