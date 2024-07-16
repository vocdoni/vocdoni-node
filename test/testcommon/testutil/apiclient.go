package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
)

type TestHTTPclient struct {
	c     *http.Client
	token *uuid.UUID
	addr  *url.URL
	t     testing.TB
}

func (c *TestHTTPclient) RequestWithQuery(method string, jsonBody any, query string, urlPath ...string) ([]byte, int) {
	u, err := url.Parse(c.addr.String())
	qt.Assert(c.t, err, qt.IsNil)
	u.RawQuery = query
	return c.request(method, u, jsonBody, urlPath...)
}

func (c *TestHTTPclient) Request(method string, jsonBody any, urlPath ...string) ([]byte, int) {
	u, err := url.Parse(c.addr.String())
	qt.Assert(c.t, err, qt.IsNil)
	return c.request(method, u, jsonBody, urlPath...)
}

func (c *TestHTTPclient) request(method string, u *url.URL, jsonBody any, urlPath ...string) ([]byte, int) {
	u.Path = path.Join(u.Path, path.Join(urlPath...))
	headers := http.Header{}
	if c.token != nil {
		headers = http.Header{"Authorization": []string{"Bearer " + c.token.String()}}
	}
	body, err := json.Marshal(jsonBody)
	qt.Assert(c.t, err, qt.IsNil)
	c.t.Logf("querying %s", u)
	resp, err := c.c.Do(&http.Request{
		Method: method,
		URL:    u,
		Header: headers,
		Body:   io.NopCloser(bytes.NewBuffer(body)),
	})
	qt.Assert(c.t, err, qt.IsNil)
	data, err := io.ReadAll(resp.Body)
	qt.Assert(c.t, err, qt.IsNil)
	return data, resp.StatusCode
}

func NewTestHTTPclient(t testing.TB, addr *url.URL, bearerToken *uuid.UUID) *TestHTTPclient {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    5 * time.Second,
		DisableCompression: false,
	}
	return &TestHTTPclient{
		c:     &http.Client{Transport: tr, Timeout: time.Second * 8},
		token: bearerToken,
		addr:  addr,
		t:     t,
	}
}
