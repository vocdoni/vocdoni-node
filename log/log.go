package log

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	log      *zap.SugaredLogger
	errorLog *os.File
	// panicOnInvalidChars is set based on env LOG_PANIC_ON_INVALIDCHARS (parsed as bool)
	panicOnInvalidChars bool
)

func init() {
	// Allow overriding the default log level via $LOG_LEVEL, so that the
	// environment variable can be set globally even when running tests.
	// Always initializing the logger is also useful to avoid panics when
	// logging if the logger is nil.
	level := "error"
	if s := os.Getenv("LOG_LEVEL"); s != "" {
		level = s
	}
	Init(level, "stderr")
}

func Logger() *zap.SugaredLogger { return log }

// Init initializes the logger. Output can be either "stdout/stderr/filePath"
func Init(logLevel string, output string) {
	cfg := newConfig(logLevel, output)

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	withOptions := logger.WithOptions(zap.AddCallerSkip(1))
	log = withOptions.Sugar()
	log.Infof("logger construction succeeded at level %s with output %s", logLevel, output)

	if s := os.Getenv("LOG_PANIC_ON_INVALIDCHARS"); s != "" {
		// ignore ParseBool errors, if anything fails panicOnInvalidChars will stay false which is good
		b, _ := strconv.ParseBool(s)
		panicOnInvalidChars = b
	}
}

// SetFileErrorLog if set writes the Warning and Error messages to a file.
func SetFileErrorLog(path string) error {
	log.Infof("using file %s for logging warning and errors", path)
	var err error
	errorLog, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	return err
}

func levelFromString(logLevel string) zapcore.Level {
	switch logLevel {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

func newConfig(logLevel, output string) zap.Config {
	encoderCfg := zapcore.EncoderConfig{
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
	cfg := zap.Config{
		Level:    zap.NewAtomicLevelAt(levelFromString(logLevel)),
		Encoding: "console",
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

func writeErrorToFile(msg string) {
	if errorLog == nil {
		return
	}
	// Use a separate goroutine, to ensure we don't block.
	// Ignore the error, as we're logging errors anyway.
	go errorLog.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006/0102/150405"), msg))
}

// checkInvalidChars checks if the formatted string contains the Unicode replacement char (U+FFFD)
// and panics if env LOG_PANIC_ON_INVALIDCHARS bool is true.
//
// In production (LOG_PANIC_ON_INVALIDCHARS != true), this function returns immediately,
// i.e. no performance hit
//
// If the log string contains the "replacement char"
// https://en.wikipedia.org/wiki/Specials_(Unicode_block)#Replacement_character
// this most likely means a bug in the caller (a format mismatch in fmt.Sprintf())
func checkInvalidChars(args ...interface{}) {
	if panicOnInvalidChars {
		s := fmt.Sprint(args...)
		if strings.ContainsRune(s, '\uFFFD') {
			panic(fmt.Sprintf("log line with invalid chars: %s", s))
		}
	}
}

// Debug sends a debug level log message
func Debug(args ...interface{}) {
	log.Debug(args...)
	checkInvalidChars(args...)
}

// Info sends an info level log message
func Info(args ...interface{}) {
	log.Info(args...)
	checkInvalidChars(args...)
}

// Warn sends a warn level log message
func Warn(args ...interface{}) {
	log.Warn(args...)
	writeErrorToFile(fmt.Sprint(args...))
	checkInvalidChars(args...)
}

// Error sends an error level log message
func Error(args ...interface{}) {
	log.Error(args...)
	writeErrorToFile(fmt.Sprint(args...))
	checkInvalidChars(args...)
}

// Fatal sends a fatal level log message
func Fatal(args ...interface{}) {
	log.Fatal(args...)
	checkInvalidChars(args...)
	// We don't support log levels lower than "fatal". Help analyzers like
	// staticcheck see that, in this package, Fatal will always exit the
	// entire program.
	panic("unreachable")
}

func FormatProto(arg protoreflect.ProtoMessage) string {
	pj := protojson.MarshalOptions{
		AllowPartial:    true,
		Multiline:       false,
		EmitUnpopulated: true,
	}
	return pj.Format(arg)
}

// Debugf sends a formatted debug level log message
func Debugf(template string, args ...interface{}) {
	log.Debugf(template, args...)
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Infof sends a formatted info level log message
func Infof(template string, args ...interface{}) {
	log.Infof(template, args...)
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Warnf sends a formatted warn level log message
func Warnf(template string, args ...interface{}) {
	log.Warnf(template, args...)
	writeErrorToFile(fmt.Sprintf(template, args...))
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Errorf sends a formatted error level log message
func Errorf(template string, args ...interface{}) {
	log.Errorf(template, args...)
	writeErrorToFile(fmt.Sprintf(template, args...))
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Fatalf sends a formatted fatal level log message
func Fatalf(template string, args ...interface{}) {
	log.Fatalf(template, args...)
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Debugw sends a key-value formatted debug level log message
func Debugw(msg string, keysAndValues ...interface{}) {
	log.Debugw(msg, keysAndValues...)
}

// Infow sends a key-value formatted info level log message
func Infow(msg string, keysAndValues ...interface{}) {
	log.Infow(msg, keysAndValues...)
}

// Warnw sends a key-value formatted warn level log message
func Warnw(msg string, keysAndValues ...interface{}) {
	log.Warnw(msg, keysAndValues...)
}

// Errorw sends a key-value formatted error level log message
func Errorw(msg string, keysAndValues ...interface{}) {
	log.Errorw(msg, keysAndValues...)
}

// Fatalw sends a key-value formatted fatal level log message
func Fatalw(msg string, keysAndValues ...interface{}) {
	log.Fatalw(msg, keysAndValues...)
}
