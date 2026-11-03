package httprouter

import (
	"bytes"
	stdlog "log"
	"net/http/httptest"
	"strings"
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
		// everything after the prefix (including extra path segments) is redacted
		{"/v2/wallet/add/" + testKey + "/extra", "/v2/wallet/add/<redacted>"},
		{"/v2/wallet/add/", "/v2/wallet/add/<redacted>"},
		{"/v2/wallet/bootstrap", "/v2/wallet/bootstrap"},
		{"/v2/censuses/weighted", "/v2/censuses/weighted"},
		// double slash: empty segment before the key
		{"/v2/wallet/add//" + testKey, "/v2/wallet/add/<redacted>"},
		// case-insensitive match
		{"/v2/Wallet/Add/" + testKey, "/v2/Wallet/Add/<redacted>"},
		// percent-encoded slash in path
		{"/v2/wallet%2Fadd/" + testKey, "/v2/wallet/add/<redacted>"},
		// percent-encoded slashes with query string
		{"/v2/wallet%2fadd%2F" + testKey + "?x=1", "/v2/wallet/add/<redacted>?x=1"},
		// double slash with leading segment
		{"/v2//wallet/add/" + testKey, "/v2//wallet/add/<redacted>"},
		// invalid percent-escape before key: falls back to raw path match
		{"/v2/wallet/add/%zz" + testKey, "/v2/wallet/add/<redacted>"},
		// U+0130 grows when lowercased: the whole path is redacted
		{"/v2/İ/WALLET/ADD/" + testKey, "<redacted>"},
		{"/v2/%C4%B0/wallet/add/" + testKey + "?x=1", "<redacted>?x=1"},
		// U+0130 without the prefix: unchanged
		{"/v2/İ/censuses", "/v2/İ/censuses"},
	} {
		got := RedactRequestURI(tc.in)
		qt.Check(t, got, qt.Equals, tc.want, qt.Commentf("in: %s", tc.in))
		if strings.Contains(tc.in, testKey) {
			qt.Check(t, got, qt.Not(qt.Contains), testKey, qt.Commentf("key leaked in: %s", tc.in))
		}
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

	// percent-encoded slash variant
	buf.Reset()
	req2 := httptest.NewRequest("POST", "/v2/wallet%2Fadd/"+testKey, nil)
	f.NewLogEntry(req2).Write(200, 2, nil, time.Millisecond, nil)
	qt.Assert(t, buf.String(), qt.Not(qt.Contains), testKey)

	// case variant with double slash
	buf.Reset()
	req3 := httptest.NewRequest("POST", "/v2/Wallet/add//"+testKey, nil)
	f.NewLogEntry(req3).Write(200, 2, nil, time.Millisecond, nil)
	qt.Assert(t, buf.String(), qt.Not(qt.Contains), testKey)
}
