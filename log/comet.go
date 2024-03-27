package log

import (
	"sync/atomic"

	cometlog "github.com/cometbft/cometbft/libs/log"
)

// CometLogger implements cometbft's Logger interface, with a couple of
// modifications.
//
// First, it routes the logs to go-dvote's logger, so that we don't end up with
// two loggers writing directly to stdout or stderr.
type CometLogger struct {
	keyvals  []any
	Artifact string
}

var _ cometlog.Logger = (*CometLogger)(nil)

var cometLogLevel atomic.Int32 // 0:debug 1:info 2:error 3:disabled

const (
	cometLogLevelDebug = int32(iota)
	cometLogLevelInfo
	cometLogLevelError
	cometLogLevelDisabled
)

// Debug logs a message at level debug.
func (l *CometLogger) Debug(msg string, keyvals ...any) {
	if cometLogLevel.Load() == cometLogLevelDebug {
		Logger().Debug().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

// Info logs a message at level info.
func (l *CometLogger) Info(msg string, keyvals ...any) {
	if cometLogLevel.Load() <= cometLogLevelInfo {
		Logger().Info().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

// Error logs a message at level error.
func (l *CometLogger) Error(msg string, keyvals ...any) {
	if cometLogLevel.Load() <= cometLogLevelError {
		Logger().Error().CallerSkipFrame(100).Fields(keyvals).Msg(l.Artifact + ": " + msg)
	}
}

// With implements Logger by constructing a new filter with a keyvals appended
// to the logger.
func (l *CometLogger) With(keyvals ...any) cometlog.Logger {
	// Make sure we copy the values, to avoid modifying the parent.
	// TODO(mvdan): use zap's With method directly.
	l2 := &CometLogger{Artifact: l.Artifact}
	l2.keyvals = append(l2.keyvals, l.keyvals...)
	l2.keyvals = append(l2.keyvals, keyvals...)
	return l2
}

// NewCometLogger creates a CometBFT compatible logger for specified artifact
func NewCometLogger(artifact string, logLevel string) *CometLogger {
	tl := &CometLogger{Artifact: artifact}
	SetCometLogLevel(logLevel)
	return tl
}

// SetCometLogLevel limits which messages from cometbft will be surfaced
// Valid keywords: "debug", "info", "error", "disabled" ("none" also accepted)
func SetCometLogLevel(logLevel string) {
	switch logLevel {
	case "debug":
		cometLogLevel.Store(cometLogLevelDebug)
	case "info":
		cometLogLevel.Store(cometLogLevelInfo)
	case "error":
		cometLogLevel.Store(cometLogLevelError)
	case "disabled", "none":
		cometLogLevel.Store(cometLogLevelDisabled)
	}
}

// CometLogLevel returns the current logLevel ("debug", "info", "error" or "disabled")
func CometLogLevel() string {
	switch cometLogLevel.Load() {
	case cometLogLevelDebug:
		return "debug"
	case cometLogLevelInfo:
		return "info"
	case cometLogLevelError:
		return "error"
	default:
		return "disabled"
	}
}
