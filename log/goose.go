package log

import "github.com/pressly/goose/v3"

// gooseLogger is a logger that implements goose.Logger interface,
// hardcoded to log messages as Debug level
type gooseLogger struct{}

var _ goose.Logger = (*gooseLogger)(nil)

func (*gooseLogger) Fatal(v ...any)                 { Fatal(v...) }
func (*gooseLogger) Fatalf(format string, v ...any) { Fatalf(format, v...) }
func (*gooseLogger) Print(v ...any)                 { Debug(v...) }
func (*gooseLogger) Println(v ...any)               { Debug(v...) }
func (*gooseLogger) Printf(format string, v ...any) { Debugf(format, v...) }

// TODO: goose passes a trailing newline to Printf.
// We should trim format (or v...?) before passing it to Debugf, to avoid printing an empty line

// GooseLogger provides access to a goose compatible logger,
// hardcoded to log messages as Debug level
func GooseLogger() *gooseLogger { return &gooseLogger{} }
