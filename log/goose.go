package log

// gooseLogger is a logger that implements goose.Logger interface,
// hardcoded to log messages as Debug level
type gooseLogger struct{}

func (*gooseLogger) Fatal(v ...interface{})                 { Fatal(v...) }
func (*gooseLogger) Fatalf(format string, v ...interface{}) { Fatalf(format, v...) }
func (*gooseLogger) Print(v ...interface{})                 { Debug(v...) }
func (*gooseLogger) Println(v ...interface{})               { Debug(v...) }
func (*gooseLogger) Printf(format string, v ...interface{}) { Debugf(format, v...) }

// TODO: goose passes a trailing newline to Printf.
// We should trim format (or v...?) before passing it to Debugf, to avoid printing an empty line

// GooseLogger provides access to a goose compatible logger,
// hardcoded to log messages as Debug level
func GooseLogger() *gooseLogger { return &gooseLogger{} }
