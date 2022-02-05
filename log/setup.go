package log

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var config Config

type LogFormat int

const (
	ColorizedOutput LogFormat = iota
	PlaintextOutput
	JSONOutput
)

type Config struct {
	// Format overrides the format of the log output. Defaults to ColorizedOutput
	Format LogFormat

	// Level is the default minimum enabled logging level.
	Level zapcore.Level

	// SubsystemLevels are the default levels per-subsystem. When unspecified, defaults to Level.
	SubsystemLevels map[string]zapcore.Level

	// Output can be either "stdout", "stderr" or "/path/to/file".
	Output string

	// Labels is a set of key-values to apply to all loggers
	Labels map[string]string
}

// ErrNoSuchLogger is returned when the util pkg is asked for a non existant logger
var ErrNoSuchLogger = errors.New("error: No such logger")

var loggerMutex sync.RWMutex // guards access to global logger state

// loggers is the set of loggers in the system
var loggers = make(map[string]*zap.SugaredLogger)
var levels = make(map[string]zap.AtomicLevel)

// primaryCore is the primary logging core
var primaryCore zapcore.Core

// loggerCore is the base for all loggers created by this package
var loggerCore = &lockedMultiCore{}

// SetupLogging will initialize the logger backend and set the flags.
// TODO calling this in `init` pushes all configuration to env variables
// - move it out of `init`? then we need to change all the code (js-ipfs, go-ipfs) to call this explicitly
// - have it look for a config file? need to define what that is
func SetupLogging(cfg Config) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	config = cfg

	ws, _, err := zap.Open(cfg.Output)
	if err != nil {
		panic(fmt.Sprintf("unable to open logging output: %v", err))
	}

	newPrimaryCore := newCore(cfg.Format, ws, zapcore.DebugLevel) // the main core needs to log everything.

	for k, v := range cfg.Labels {
		newPrimaryCore = newPrimaryCore.With([]zap.Field{zap.String(k, v)})
	}

	setPrimaryCore(newPrimaryCore)
	setAllLoggers(cfg.Level)

	for name, level := range cfg.SubsystemLevels {
		if leveler, ok := levels[name]; ok {
			leveler.SetLevel(zapcore.Level(level))
		} else {
			levels[name] = zap.NewAtomicLevelAt(zapcore.Level(level))
		}
	}
}

// SetPrimaryCore changes the primary logging core. If the SetupLogging was
// called then the previously configured core will be replaced.
func SetPrimaryCore(core zapcore.Core) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	setPrimaryCore(core)
}

func setPrimaryCore(core zapcore.Core) {
	if primaryCore != nil {
		loggerCore.ReplaceCore(primaryCore, core)
	} else {
		loggerCore.AddCore(core)
	}
	primaryCore = core
}

// SetAllLoggers changes the logging level of all loggers to lvl
func SetAllLoggers(lvl zapcore.Level) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	setAllLoggers(lvl)
}

func setAllLoggers(lvl zapcore.Level) {
	for _, l := range levels {
		l.SetLevel(lvl)
	}
}

// levelFromString accepts a lowercase "debug", "info", "warn", "error", "fatal"
// and returns the corresponding zapcore.Level
func levelFromString(logLevel string) (zapcore.Level, error) {
	switch logLevel {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("couldn't parse log level '%s'", logLevel)
	}
}

// SetLogLevel changes the log level of a specific subsystem
// name=="*" changes all subsystems
func SetLogLevel(name, level string) error {
	lvl, err := levelFromString(level)
	if err != nil {
		return err
	}

	// wildcard, change all
	if name == "*" {
		SetAllLoggers(zapcore.Level(lvl))
		return nil
	}

	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	// Check if we have a logger by that name
	if _, ok := levels[name]; !ok {
		return ErrNoSuchLogger
	}

	levels[name].SetLevel(zapcore.Level(lvl))
	return nil
}

// SetLogLevelRegex sets all loggers to level `l` that match expression `e`.
// An error is returned if `e` fails to compile.
func SetLogLevelRegex(e, l string) error {
	lvl, err := levelFromString(l)
	if err != nil {
		return err
	}

	rem, err := regexp.Compile(e)
	if err != nil {
		return err
	}

	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	for name := range loggers {
		if rem.MatchString(name) {
			levels[name].SetLevel(zapcore.Level(lvl))
		}
	}
	return nil
}

// GetSubsystems returns a sorted slice containing the
// names of the current loggers
func GetSubsystems() []string {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	subs := make([]string, 0, len(loggers))

	for k := range loggers {
		subs = append(subs, k)
	}
	sort.Strings(subs)
	return subs
}

// Named returns a named logger.
func Named(name string) *zap.SugaredLogger {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	log, ok := loggers[name]
	if !ok {
		level, ok := levels[name]
		if !ok {
			level = zap.NewAtomicLevelAt(config.Level)
			levels[name] = level
		}
		log = zap.New(loggerCore).
			WithOptions(
				zap.IncreaseLevel(level),
				zap.AddCaller(),
			).
			Named(name).
			Sugar()

		loggers[name] = log
	}

	return log
}

// GetLogLevel returns the configured log level of a specific subsystem
func GetLogLevel(name string) (level string, found bool) {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()

	l, found := config.SubsystemLevels[name]
	return l.String(), found
}
