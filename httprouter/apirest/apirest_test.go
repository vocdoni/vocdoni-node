package apirest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
)

func TestRouterWithAPI(t *testing.T) {
	r := httprouter.HTTProuter{}
	rng := testutil.NewRandom(124)
	port := 23000 + rng.RandomIntn(1024)
	url := fmt.Sprintf("http://127.0.0.1:%d/api", port)
	err := r.Init("127.0.0.1", port)
	qt.Check(t, err, qt.IsNil)

	// Create a standard API handler
	stdAPI, err := NewAPI(&r, "/api")
	qt.Check(t, err, qt.IsNil)

	// Add a public handler to serve requests on std namespace
	stdAPI.RegisterMethod("/hello/*", "POST", MethodAccessTypePublic,
		func(msg *APIdata, ctx *httprouter.HTTPContext) error {
			return ctx.Send([]byte("hello public!"), 200)
		})

	// Add an admin handler to serve requests on std namespace
	stdAPI.RegisterMethod("/admin/*", "POST", MethodAccessTypeAdmin,
		func(msg *APIdata, ctx *httprouter.HTTPContext) error {
			return ctx.Send([]byte("hello admin!"), 200)
		})

	// Add a private handler
	stdAPI.RegisterMethod("/private/{name}", "POST", MethodAccessTypePrivate,
		func(msg *APIdata, ctx *httprouter.HTTPContext) error {
			return ctx.Send([]byte(fmt.Sprintf("hello %s!", ctx.URLParam("name"))), 200)
		})

	// Add a quota handler
	stdAPI.RegisterMethod("/quota/{name}", "POST", MethodAccessTypeQuota,
		func(msg *APIdata, ctx *httprouter.HTTPContext) error {
			return ctx.Send([]byte(fmt.Sprintf("hello %s!", ctx.URLParam("name"))), 200)
		})

	// Set the bearer admin token
	stdAPI.SetAdminToken("abcd")

	// Create a token with access to 2 requests
	stdAPI.AddAuthToken("1234", 2)

	// Test public
	resp := doRequest(t, url+"/hello/1234", "", "POST", []byte{})
	qt.Check(t, resp, qt.DeepEquals, []byte("hello public!\n"))

	// Test private and Path vars
	resp = doRequest(t, url+"/private/john", "1234", "POST", []byte{})
	qt.Check(t, resp, qt.DeepEquals, []byte("hello john!\n"))
	resp = doRequest(t, url+"/private/martin", "1234", "POST", []byte{})
	qt.Check(t, resp, qt.DeepEquals, []byte("hello martin!\n"))

	// Test quota
	resp = doRequest(t, url+"/quota/john", "1234", "POST", []byte{})
	qt.Check(t, resp, qt.DeepEquals, []byte("hello john!\n"))
	resp = doRequest(t, url+"/quota/martin", "1234", "POST", []byte{})
	qt.Check(t, resp, qt.DeepEquals, []byte("hello martin!\n"))

	// Token should be out of authorized requests now
	resp = doRequest(t, url+"/quota/ping", "1234", "POST", []byte("hello"))
	qt.Check(t, string(resp), qt.Contains, "no more requests available")

	// Private requests should still be valid, though
	resp = doRequest(t, url+"/private/john", "1234", "POST", []byte{})
	qt.Check(t, resp, qt.DeepEquals, []byte("hello john!\n"))

	// After revoking token, private method should not be valid
	stdAPI.DelAuthToken("1234")
	resp = doRequest(t, url+"/private/ping", "1234", "POST", []byte("hello"))
	qt.Check(t, string(resp), qt.Contains, "auth token not valid")

	// Test admin
	resp = doRequest(t, url+"/admin/do", "abcd", "POST", []byte("hello"))
	qt.Check(t, resp, qt.DeepEquals, []byte("hello admin!\n"))
	resp = doRequest(t, url+"/admin/do", "abcde", "POST", []byte("hello"))
	qt.Check(t, string(resp), qt.Contains, "admin token not valid\n")
}

func doRequest(t *testing.T, url, authToken, method string, body []byte) []byte {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	qt.Check(t, err, qt.IsNil)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	qt.Check(t, err, qt.IsNil)
	respBody, err := io.ReadAll(resp.Body)
	qt.Check(t, err, qt.IsNil)
	return respBody
}
