package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

var (
	log zerolog.Logger
	// panicOnInvalidChars is set based on env LOG_PANIC_ON_INVALIDCHARS (parsed as bool)
	panicOnInvalidChars = os.Getenv("LOG_PANIC_ON_INVALIDCHARS") == "true"
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
	Init(level, "stderr", nil)
}

// Logger provides access to the global logger (zerolog).
func Logger() *zerolog.Logger { return &log }

var logTestWriter io.Writer // for TestLogger
const logTestWriterName = "log_test_writer"

var logTestTime, _ = time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")

type testHook struct{}

// To ensure that the log output in the test is deterministic.
func (h *testHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	e.Stringer("time", logTestTime)
}

type errorLevelWriter struct {
	io.Writer
}

var _ zerolog.LevelWriter = &errorLevelWriter{}

func (w *errorLevelWriter) Write(p []byte) (int, error) {
	panic("should be calling WriteLevel")
}

func (w *errorLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if level < zerolog.WarnLevel {
		return len(p), nil
	}
	return w.Writer.Write(p)
}

// invalidCharChecker checks if the formatted string contains the Unicode replacement char (U+FFFD)
// and panics if env LOG_PANIC_ON_INVALIDCHARS bool is true.
//
// In production (LOG_PANIC_ON_INVALIDCHARS != true), this function returns immediately,
// i.e. no performance hit
//
// If the log string contains the "replacement char"
// https://en.wikipedia.org/wiki/Specials_(Unicode_block)#Replacement_character
// this most likely means a bug in the caller (a format mismatch in fmt.Sprintf())
type invalidCharChecker struct{}

func (*invalidCharChecker) Write(p []byte) (int, error) {
	if bytes.ContainsRune(p, '\uFFFD') {
		panic(fmt.Sprintf("log line with invalid chars: %q", string(p)))
	}
	return len(p), nil
}

// Init initializes the logger. Output can be either "stdout/stderr/<filePath>".
// Log level can be "debug/info/warn/error".
// errorOutput is an optional filename which only receives Warning and Error messages.
func Init(level, output string, errorOutput io.Writer) {
	var out io.Writer
	switch output {
	case "stdout":
		out = os.Stdout
	case "stderr":
		out = os.Stderr
	case logTestWriterName:
		out = logTestWriter
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			panic(fmt.Sprintf("cannot create log output: %v", err))
		}
		out = f
	}
	out = zerolog.ConsoleWriter{
		Out:        out,
		TimeFormat: time.RFC3339Nano,
	}
	outputs := []io.Writer{out}

	if errorOutput != nil {
		outputs = append(outputs, &errorLevelWriter{zerolog.ConsoleWriter{
			Out:        errorOutput,
			TimeFormat: time.RFC3339Nano,
			NoColor:    true, // error log files should not be colored
		}})
	}
	if panicOnInvalidChars {
		outputs = append(outputs, zerolog.ConsoleWriter{Out: &invalidCharChecker{}})
	}
	if len(outputs) > 1 {
		out = zerolog.MultiLevelWriter(outputs...)
	}

	// Init the global logger var, with millisecond timestamps
	log = zerolog.New(out).With().Timestamp().Logger()
	if output == logTestWriterName {
		log = log.Hook(&testHook{})
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	// Include caller, increasing SkipFrameCount to account for this log package wrapper
	log = log.With().Caller().Logger()
	zerolog.CallerSkipFrameCount = 3
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return fmt.Sprintf("%s/%s:%d", path.Base(path.Dir(file)), path.Base(file), line)
	}

	switch level {
	case LogLevelDebug:
		log = log.Level(zerolog.DebugLevel)
	case LogLevelInfo:
		log = log.Level(zerolog.InfoLevel)
	case LogLevelWarn:
		log = log.Level(zerolog.WarnLevel)
	case LogLevelError:
		log = log.Level(zerolog.ErrorLevel)
	default:
		panic(fmt.Sprintf("invalid log level: %q", level))
	}

	log.Info().Msgf("logger construction succeeded at level %s with output %s", level, output)
}

// Level returns the current log level
func Level() string {
	switch level := log.GetLevel(); level {
	case zerolog.DebugLevel:
		return LogLevelDebug
	case zerolog.InfoLevel:
		return LogLevelInfo
	case zerolog.WarnLevel:
		return LogLevelWarn
	case zerolog.ErrorLevel:
		return LogLevelError
	default:
		panic(fmt.Sprintf("invalid log level: %q", level))
	}
}

// Debug sends a debug level log message
func Debug(args ...interface{}) {
	if log.GetLevel() > zerolog.DebugLevel {
		return
	}
	log.Debug().Msg(fmt.Sprint(args...))
}

// Info sends an info level log message
func Info(args ...interface{}) {
	log.Info().Msg(fmt.Sprint(args...))
}

// Monitor is a wrapper around Info that allows passing a map of key-value pairs.
// This is useful for structured logging and monitoring.
// The caller information is skipped.
func Monitor(msg string, args map[string]interface{}) {
	log.Info().CallerSkipFrame(100).Fields(args).Msg(msg)
}

// Warn sends a warn level log message
func Warn(args ...interface{}) {
	log.Warn().Msg(fmt.Sprint(args...))
}

// Error sends an error level log message
func Error(args ...interface{}) {
	log.Error().Msg(fmt.Sprint(args...))
}

// Fatal sends a fatal level log message
func Fatal(args ...interface{}) {
	log.Fatal().Msg(fmt.Sprint(args...) + "\n" + string(debug.Stack()))
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
	Logger().Debug().Msgf(template, args...)
}

// Infof sends a formatted info level log message
func Infof(template string, args ...interface{}) {
	Logger().Info().Msgf(template, args...)
}

// Warnf sends a formatted warn level log message
func Warnf(template string, args ...interface{}) {
	Logger().Warn().Msgf(template, args...)
}

// Errorf sends a formatted error level log message
func Errorf(template string, args ...interface{}) {
	Logger().Error().Msgf(template, args...)
}

// Fatalf sends a formatted fatal level log message
func Fatalf(template string, args ...interface{}) {
	Logger().Fatal().Msgf(template+"\n"+string(debug.Stack()), args...)
}

// Debugw sends a debug level log message with key-value pairs.
func Debugw(msg string, keyvalues ...interface{}) {
	Logger().Debug().Fields(keyvalues).Msg(msg)
}

// Infow sends an info level log message with key-value pairs.
func Infow(msg string, keyvalues ...interface{}) {
	Logger().Info().Fields(keyvalues).Msg(msg)
}

// Warnw sends a warning level log message with key-value pairs.
func Warnw(msg string, keyvalues ...interface{}) {
	Logger().Warn().Fields(keyvalues).Msg(msg)
}

// Errorw sends an error level log message with a special format for errors.
func Errorw(err error, msg string) {
	Logger().Error().Err(err).Msg(msg)
}
