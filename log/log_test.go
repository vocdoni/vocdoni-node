package log

import (
	"os"
	"testing"
)

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
