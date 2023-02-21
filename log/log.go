package log

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
	log      zerolog.Logger
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

// Logger provides access to the global logger (zerolog).
func Logger() *zerolog.Logger { return &log }

// Init initializes the logger. Output can be either "stdout/stderr/<filePath>".
// Log level can be "debug/info/warn/error".
func Init(logLevel string, output string) {
	var out io.Writer
	switch output {
	case "stdout":
		out = os.Stdout
	case "stderr":
		out = os.Stderr
	default:
		errorLog, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			panic(fmt.Sprintf("invalid log output: %v", err))
		}
		out = errorLog
	}

	// Init the global logger var, with millisecond timestamps
	log = zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = out
		w.TimeFormat = time.RFC3339Nano
	})).With().Timestamp().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	// Include caller, increasing SkipFrameCount to account for this log package wrapper
	log = log.With().Caller().Logger()
	zerolog.CallerSkipFrameCount = 3

	switch logLevel {
	case LogLevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LogLevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LogLevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LogLevelError:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		panic("invalid log level")
	}

	log.Info().Msgf("logger construction succeeded at level %s with output %s", logLevel, output)

	if s := os.Getenv("LOG_PANIC_ON_INVALIDCHARS"); s != "" {
		// ignore ParseBool errors, if anything fails panicOnInvalidChars will stay false which is good
		b, _ := strconv.ParseBool(s)
		panicOnInvalidChars = b
	}
}

// SetFileErrorLog if set writes the Warning and Error messages to a file.
func SetFileErrorLog(path string) error {
	Logger().Info().Msgf("using file %s for logging warning and errors", path)
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
	log.Debug().Msg(fmt.Sprint(args...))
	checkInvalidChars(args...)
}

// Info sends an info level log message
func Info(args ...interface{}) {
	log.Info().Msg(fmt.Sprint(args...))
	checkInvalidChars(args...)
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
	writeErrorToFile(fmt.Sprint(args...))
	checkInvalidChars(args...)
}

// Error sends an error level log message
func Error(args ...interface{}) {
	log.Error().Msg(fmt.Sprint(args...))
	writeErrorToFile(fmt.Sprint(args...))
	checkInvalidChars(args...)
}

// Fatal sends a fatal level log message
func Fatal(args ...interface{}) {
	log.Fatal().Msg(fmt.Sprint(args...))
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
	Logger().Debug().Msgf(template, args...)
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Infof sends a formatted info level log message
func Infof(template string, args ...interface{}) {
	Logger().Info().Msgf(template, args...)
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Warnf sends a formatted warn level log message
func Warnf(template string, args ...interface{}) {
	Logger().Warn().Msgf(template, args...)
	writeErrorToFile(fmt.Sprintf(template, args...))
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Errorf sends a formatted error level log message
func Errorf(template string, args ...interface{}) {
	Logger().Error().Msgf(template, args...)
	writeErrorToFile(fmt.Sprintf(template, args...))
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Fatalf sends a formatted fatal level log message
func Fatalf(template string, args ...interface{}) {
	Logger().Fatal().Msgf(template, args...)
	checkInvalidChars(fmt.Sprintf(template, args...))
}

// Debugw sends a debug level log message with key-value pairs.
func Debugw(msg string, keyvalues ...interface{}) {
	Logger().Debug().Fields(keyvalues).Msg(msg)
	checkInvalidChars(fmt.Sprintf("%s %+v", msg, keyvalues))
}

// Infow sends an info level log message with key-value pairs.
func Infow(msg string, keyvalues ...interface{}) {
	Logger().Info().Fields(keyvalues).Msg(msg)
	checkInvalidChars(fmt.Sprintf("%s %+v", msg, keyvalues))
}

// Warnw sends a warning level log message with key-value pairs.
func Warnw(msg string, keyvalues ...interface{}) {
	Logger().Warn().Fields(keyvalues).Msg(msg)
	checkInvalidChars(fmt.Sprintf("%s %+v", msg, keyvalues))
}

// Errorw sends an error level log message with a special format for errors.
func Errorw(err error, msg string) {
	Logger().Error().Err(err).Msg(msg)
}
