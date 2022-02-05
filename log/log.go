package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	log      *zap.SugaredLogger
	errorLog *os.File
)

func init() {
	// Allow overriding the default log level via $LOG_LEVEL, so that the
	// environment variable can be set globally even when running tests.
	// Always initializing the logger is also useful to avoid panics when
	// logging if the logger is nil.
	level := "error"
	if env, found := os.LookupEnv("LOG_LEVEL"); found {
		level = env
	}

	Init(level, "stderr")
}

// Init initializes the logger.
//
// logLevels is a comma-separated string like "somename:debug,*:info"
// (for backwards-compatibility, the "*:" is optional, so "warn" is interpreted as "*:warn")
//
// output can be either "stdout", "stderr" or "/path/to/file" and it's a global setting.
//
// It's safe to use concurrently.
func Init(logLevels string, output string) {
	cfg := Config{
		Output:          output,
		SubsystemLevels: map[string]zapcore.Level{},
	}

	for _, ll := range strings.Split(logLevels, ",") {
		kv := strings.Split(ll, ":")

		var s, l string
		switch len(kv) {
		case 1: // backwards compatibility: --logLevel=error is equivalent to --logLevel="*:error"
			s, l = "*", kv[0]
		case 2: // kv[0] is subsystem name, kv[1] is level
			s, l = kv[0], kv[1]
		default:
			fmt.Println("warning: couldn't split log level: ", ll)
			continue
		}

		level, err := levelFromString(l)
		if err != nil {
			fmt.Println("warning: ", err)
			continue
		}

		if s == "*" {
			cfg.Level = level
		} else {
			cfg.SubsystemLevels[s] = level
		}
	}

	SetupLogging(cfg)

	log = Named("main").Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar()

	log.Infof("logger construction succeeded at default level %s with output %s", config.Level, output)
	log.Infof("logger subsystems: %v", levels)
}

// SetFileErrorLog if set writes the Warning and Error messages to a file.
func SetFileErrorLog(path string) error {
	log.Infof("using file %s for logging warning and errors", path)
	var err error
	errorLog, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	return err
}

func writeErrorToFile(msg string) {
	if errorLog == nil {
		return
	}
	// Use a separate goroutine, to ensure we don't block.
	// Ignore the error, as we're logging errors anyway.
	go errorLog.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("2006/0102/150405"), msg))
}

// Debug sends a debug level log message
func Debug(args ...interface{}) {
	log.Debug(args...)
}

// Info sends an info level log message
func Info(args ...interface{}) {
	log.Info(args...)
}

// Warn sends a warn level log message
func Warn(args ...interface{}) {
	log.Warn(args...)
	writeErrorToFile(fmt.Sprint(args...))
}

// Error sends an error level log message
func Error(args ...interface{}) {
	log.Error(args...)
	writeErrorToFile(fmt.Sprint(args...))
}

// Fatal sends a fatal level log message
func Fatal(args ...interface{}) {
	log.Fatal(args...)
	// We don't support log levels lower than "fatal". Help analyzers like
	// staticcheck see that, in this package, Fatal will always exit the
	// entire program.
	panic("unreachable")
}

// Debugf sends a formatted debug level log message
func Debugf(template string, args ...interface{}) {
	log.Debugf(template, args...)
}

// Infof sends a formatted info level log message
func Infof(template string, args ...interface{}) {
	log.Infof(template, args...)
}

// Warnf sends a formatted warn level log message
func Warnf(template string, args ...interface{}) {
	log.Warnf(template, args...)
	writeErrorToFile(fmt.Sprintf(template, args...))
}

// Errorf sends a formatted error level log message
func Errorf(template string, args ...interface{}) {
	log.Errorf(template, args...)
	writeErrorToFile(fmt.Sprintf(template, args...))
}

// Fatalf sends a formatted fatal level log message
func Fatalf(template string, args ...interface{}) {
	log.Fatalf(template, args...)
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
