package httprouter

import (
	"bytes"
	stdlog "log"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5/middleware"
)

const testKey = "0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e5e0b1e"

func TestRedactRequestURI(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/v2/wallet/add/" + testKey, "/v2/wallet/add/<redacted>"},
		{"/wallet/add/0x" + testKey, "/wallet/add/<redacted>"},
		{"/v2/wallet/add/" + testKey + "?x=1", "/v2/wallet/add/<redacted>?x=1"},
		{"/v2/wallet/add/" + testKey + "/extra", "/v2/wallet/add/<redacted>/extra"},
		{"/v2/wallet/add/", "/v2/wallet/add/<redacted>"},
		{"/v2/wallet/bootstrap", "/v2/wallet/bootstrap"},
		{"/v2/censuses/weighted", "/v2/censuses/weighted"},
	} {
		qt.Check(t, RedactRequestURI(tc.in), qt.Equals, tc.want, qt.Commentf("in: %s", tc.in))
	}
}

// TestRequestLogRedactsPrivateKey checks that the request log line of
// POST /wallet/add/{privateKey} carries <redacted> instead of the key, and
// that the formatter leaves the request it was given untouched.
func TestRequestLogRedactsPrivateKey(t *testing.T) {
	var buf bytes.Buffer
	f := redactingLogFormatter{&middleware.DefaultLogFormatter{
		Logger:  stdlog.New(&buf, "", 0),
		NoColor: true,
	}}
	req := httptest.NewRequest("POST", "/v2/wallet/add/"+testKey, nil)
	f.NewLogEntry(req).Write(200, 2, nil, time.Millisecond, nil)

	qt.Assert(t, buf.String(), qt.Not(qt.Contains), testKey)
	qt.Assert(t, buf.String(), qt.Contains, "/v2/wallet/add/<redacted>")
	qt.Assert(t, req.RequestURI, qt.Equals, "/v2/wallet/add/"+testKey)
	qt.Assert(t, req.URL.Path, qt.Equals, "/v2/wallet/add/"+testKey)
}
