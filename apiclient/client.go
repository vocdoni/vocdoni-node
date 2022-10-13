package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
	"go.vocdoni.io/dvote/log"
)

const (
	// HTTPGET is the method string used for calling Request()
	HTTPGET = "GET"
	// HTTPPOST is the method string used for calling Request()
	HTTPPOST = "POST"

	errCodeNot200 = "API server returned status code is not 200"
)

// HTTPclient is the Vocdoni API HTTP client.
type HTTPclient struct {
	c       *http.Client
	token   *uuid.UUID
	addr    *url.URL
	account *ethereum.SignKeys
	chainID string
}

// NewHTTPclient creates a new HTTP(s) API Vocdoni client.
func NewHTTPclient(addr *url.URL, bearerToken *uuid.UUID) (*HTTPclient, error) {
	tr := &http.Transport{
		IdleConnTimeout:    10 * time.Second,
		DisableCompression: false,
		WriteBufferSize:    1 * 1024 * 1024, // 1 MiB
		ReadBufferSize:     1 * 1024 * 1024, // 1 MiB
	}
	c := &HTTPclient{
		c:     &http.Client{Transport: tr, Timeout: time.Second * 8},
		token: bearerToken,
		addr:  addr,
	}
	data, status, err := c.Request(HTTPGET, nil, "chain", "info")
	if err != nil {
		return nil, err
	}
	if status != bearerstdapi.HTTPstatusCodeOK {
		return nil, fmt.Errorf("%s: %d (%s)", errCodeNot200, status, data)
	}
	info := &api.ChainInfo{}
	if err := json.Unmarshal(data, info); err != nil {
		return nil, fmt.Errorf("cannot get chain ID from API server")
	}
	c.chainID = info.ID
	return c, nil
}

// ChainID returns the chain identifier name in which the API backend is connected.
func (c *HTTPclient) ChainID() string {
	return c.chainID
}

// SetAccount sets the Vocdoni account used for signing transactions.
func (c *HTTPclient) SetAccount(accountPrivateKey string) error {
	c.account = new(ethereum.SignKeys)
	return c.account.AddHexKey(accountPrivateKey)
}

// SetAuthToken configures the bearer authentication token.
func (c *HTTPclient) SetAuthToken(token *uuid.UUID) {
	c.token = token
}

// SetHostAddr configures the host address of the API server.
func (c *HTTPclient) SetHostAddr(addr *url.URL) error {
	c.addr = addr
	data, status, err := c.Request(HTTPGET, nil, "chain", "info")
	if err != nil {
		return err
	}
	if status != bearerstdapi.HTTPstatusCodeOK {
		return fmt.Errorf("%s: %d (%s)", errCodeNot200, status, data)
	}
	info := &api.ChainInfo{}
	if err := json.Unmarshal(data, info); err != nil {
		return fmt.Errorf("cannot get chain ID from API server")
	}
	c.chainID = info.ID
	return nil
}

// Request performs a `method` type raw request to the endpoint specyfied in urlPath parameter.
// Method is either GET or POST. If POST, a JSON struct should be attached.  Returns the response,
// the status code and an error.
func (c *HTTPclient) Request(method string, jsonBody any, urlPath ...string) ([]byte, int, error) {
	body, err := json.Marshal(jsonBody)
	if err != nil {
		return nil, 0, err
	}
	u, err := url.Parse(c.addr.String())
	if err != nil {
		return nil, 0, err
	}
	u.Path = path.Join(u.Path, path.Join(urlPath...))
	headers := http.Header{}
	if c.token != nil {
		headers = http.Header{
			"Authorization": []string{"Bearer " + c.token.String()},
			"User-Agent":    []string{"Vocdoni API client / 1.0"},
		}
	}
	log.Debugf("%s %s", method, u)
	resp, err := c.c.Do(&http.Request{
		Method: method,
		URL:    u,
		Header: headers,
		Body:   io.NopCloser(bytes.NewBuffer(body)),
	})
	if err != nil {
		return nil, 0, err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}
