package log

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

var (
	sampleInt      = 3
	sampleBytes    = []byte("123")
	sampleList     = []int64{10, 0, -10}
	sampleDuration = time.Second
	sampleTime     = time.Unix(12345678, 0)

	errSample = errors.New("some error")
)

func doLogs() {
	// Some sample logs from existing code.
	Infof("added %d keys to census %x", sampleInt, sampleBytes)
	Debugw("importing census", "root", "abc123", "type", "type1")
	Errorf("cannot commit to blockstore: %v", errSample)
	Warnw("various types",
		"list", sampleList,
		"duration", sampleDuration,
		"time", sampleTime,
	)
	Error(errSample)
}

func TestCheckInvalidChars(t *testing.T) {
	v := []byte{'h', 'e', 'l', 'l', 'o', 0xff, 'w', 'o', 'r', 'l', 'd'}
	_ = os.Setenv("LOG_PANIC_ON_INVALIDCHARS", "false")
	Init("debug", "stderr")
	Debugf("%s", v)
	// should not panic since env var is false. if it panics, test will fail

	// now enable panic and try again: should recover() and never reach t.Errorf()
	_ = os.Setenv("LOG_PANIC_ON_INVALIDCHARS", "true")
	Init("debug", "stderr")
	defer func() { recover() }()
	Debugf("%s", v)
	t.Errorf("Debugf(%s) should have panicked because of invalid char", v)
}

var update = flag.Bool("update", false, "update expected output file")

func TestLoggerOutput(t *testing.T) {
	wantPath := "log_test_out.txt"
	wantBytes, err := os.ReadFile(wantPath)
	want := string(wantBytes)
	qt.Assert(t, err, qt.IsNil)

	var buf bytes.Buffer
	logTestWriter = &buf
	Init("debug", logTestWriterName)

	doLogs()

	got := buf.String()

	if *update {
		err := os.WriteFile(wantPath, []byte(got), 0o666)
		qt.Assert(t, err, qt.IsNil)
	} else {
		qt.Assert(t, got, qt.Equals, want)
	}
}

func BenchmarkLogger(b *testing.B) {
	logTestWriter = io.Discard // to not grow a buffer
	Init("debug", logTestWriterName)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doLogs()
	}
}
