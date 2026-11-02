package apirest

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
)

const redactTestKey = "0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e"

// captureLog sends the global log, at debug level, to a file for the rest of
// the test and returns a function that reads what was logged so far.
func captureLog(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log.txt")
	log.Init(log.LogLevelDebug, path, nil)
	t.Cleanup(func() { log.Init(cmp.Or(os.Getenv("LOG_LEVEL"), log.LogLevelError), "stderr", nil) })
	return func() string {
		data, err := os.ReadFile(path)
		qt.Assert(t, err, qt.IsNil)
		return string(data)
	}
}

// waitForLog polls the captured log until it contains want, since the request
// log line is written after the response reaches the client.
func waitForLog(t *testing.T, logged func() string, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := logged()
		if strings.Contains(got, want) {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("log never contained %q:\n%s", want, got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRequestLogRedactsWalletKey checks that the request log of a real
// POST /wallet/add/{privateKey}, with and without a query string, never
// records the key.
func TestRequestLogRedactsWalletKey(t *testing.T) {
	logged := captureLog(t)
	r := httprouter.HTTProuter{}
	qt.Assert(t, r.Init("127.0.0.1", 0), qt.IsNil)
	api, err := NewAPI(&r, "/v2")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, api.RegisterMethod("/wallet/add/{privateKey}", "POST", MethodAccessTypePublic,
		func(_ *APIdata, ctx *httprouter.HTTPContext) error {
			return ctx.Send([]byte("ok"), HTTPstatusOK)
		}), qt.IsNil)

	for _, suffix := range []string{"", "?x=1"} {
		url := fmt.Sprintf("http://%s/v2/wallet/add/%s%s", r.Address(), redactTestKey, suffix)
		resp, err := http.Post(url, "", nil)
		qt.Assert(t, err, qt.IsNil)
		_, err = io.Copy(io.Discard, resp.Body)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, resp.Body.Close(), qt.IsNil)
		qt.Assert(t, resp.StatusCode, qt.Equals, HTTPstatusOK)
		got := waitForLog(t, logged, "/v2/wallet/add/<redacted>"+suffix)
		qt.Assert(t, got, qt.Not(qt.Contains), redactTestKey)
	}
}

// TestPanicLogRedactsWalletKey checks that the log of a handler panic while
// serving POST /wallet/add/{privateKey} does not record the key.
func TestPanicLogRedactsWalletKey(t *testing.T) {
	logged := captureLog(t)
	r := httprouter.HTTProuter{}
	qt.Assert(t, r.Init("127.0.0.1", 0), qt.IsNil)
	api, err := NewAPI(&r, "/v2")
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, api.RegisterMethod("/wallet/add/{privateKey}", "POST", MethodAccessTypePublic,
		func(*APIdata, *httprouter.HTTPContext) error {
			panic("handler bug")
		}), qt.IsNil)

	resp, err := http.Post(fmt.Sprintf("http://%s/v2/wallet/add/%s", r.Address(), redactTestKey), "", nil)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, resp.Body.Close(), qt.IsNil)
	qt.Assert(t, resp.StatusCode, qt.Equals, HTTPstatusInternalErr)
	got := waitForLog(t, logged, "panic serving POST /v2/wallet/add/<redacted>")
	qt.Assert(t, got, qt.Not(qt.Contains), redactTestKey)
}
