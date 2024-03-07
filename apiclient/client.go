package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/log"
)

const (
	// HTTPGET is the method string used for calling Request()
	HTTPGET = http.MethodGet
	// HTTPPOST is the method string used for calling Request()
	HTTPPOST = http.MethodPost
	// HTTPDELETE is the method string used for calling
	HTTPDELETE = http.MethodDelete

	errCodeNot200 = "API error"

	// DefaultRetries this enables Request() to handle the situation where the server replies
	// "mempool is full", it will wait for next block and retry sending the tx
	DefaultRetries = 3
	// DefaultTimeout is the default timeout for the HTTP client
	DefaultTimeout = 10 * time.Second
)

// HTTPclient is the Vocdoni API HTTP client.
type HTTPclient struct {
	c       *http.Client
	token   *uuid.UUID
	addr    *url.URL
	account *ethereum.SignKeys
	chainID string
	circuit *circuit.ZkCircuit
	retries int
}

// NewHTTPclient creates a new HTTP(s) API Vocdoni client.
func NewHTTPclient(addr *url.URL, bearerToken *uuid.UUID) (*HTTPclient, error) {
	tr := &http.Transport{
		IdleConnTimeout:    DefaultTimeout,
		DisableCompression: false,
		WriteBufferSize:    1 * 1024 * 1024, // 1 MiB
		ReadBufferSize:     1 * 1024 * 1024, // 1 MiB
	}
	c := &HTTPclient{
		c:       &http.Client{Transport: tr, Timeout: DefaultTimeout},
		token:   bearerToken,
		addr:    addr,
		retries: DefaultRetries,
	}
	data, status, err := c.Request(HTTPGET, nil, "chain", "info")
	if err != nil {
		return nil, err
	}
	if status != apirest.HTTPstatusOK {
		log.Warnw("cannot get chain info from API server", "status", status, "data", data)
		return c, nil
	}
	info := &api.ChainInfo{}
	if err := json.Unmarshal(data, info); err != nil {
		return nil, fmt.Errorf("cannot get chain ID from API server")
	}
	c.chainID = info.ID

	c.circuit, err = circuit.LoadVersion(info.CircuitVersion)
	if err != nil {
		return nil, fmt.Errorf("error loading circuit: %w", err)
	}
	return c, nil
}

// ChainID returns the chain identifier name in which the API backend is connected.
func (c *HTTPclient) ChainID() string {
	return c.chainID
}

// SetAccount sets the Vocdoni account used for signing transactions and assign
// a new ZkAddress based on the provided private key account.
func (c *HTTPclient) SetAccount(accountPrivateKey string) error {
	c.account = new(ethereum.SignKeys)
	err := c.account.AddHexKey(accountPrivateKey)
	if err != nil {
		return err
	}
	return err
}

// Clone returns a copy of the HTTPclient with the accountPrivateKey set as the account key.
// Panics if the accountPrivateKey is not valid.
func (c *HTTPclient) Clone(accountPrivateKey string) *HTTPclient {
	clone := *c
	err := clone.SetAccount(accountPrivateKey)
	if err != nil {
		panic(err)
	}
	return &clone
}

// MyAddress returns the address of the account used for signing transactions.
func (c *HTTPclient) MyAddress() common.Address {
	return c.account.Address()
}

// SetAuthToken configures the bearer authentication token.
func (c *HTTPclient) SetAuthToken(token *uuid.UUID) {
	c.token = token
}

// BearerToken returns the current bearer authentication token.
func (c *HTTPclient) BearerToken() *uuid.UUID {
	return c.token
}

// SetHostAddr configures the host address of the API server.
func (c *HTTPclient) SetHostAddr(addr *url.URL) error {
	c.addr = addr
	data, status, err := c.Request(HTTPGET, nil, "chain", "info")
	if err != nil {
		return err
	}
	if status != apirest.HTTPstatusOK {
		return fmt.Errorf("%s: %d (%s)", errCodeNot200, status, data)
	}
	info := &api.ChainInfo{}
	if err := json.Unmarshal(data, info); err != nil {
		return fmt.Errorf("cannot get chain ID from API server")
	}
	c.chainID = info.ID
	return nil
}

func (c *HTTPclient) SetRetries(n int) {
	c.retries = n
}

func (c *HTTPclient) SetTimeout(d time.Duration) {
	c.c.Timeout = d
	c.c.Transport.(*http.Transport).ResponseHeaderTimeout = d
}

// Request performs a `method` type raw request to the endpoint specified in urlPath parameter.
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
			"Content-Type":  []string{"application/json"},
		}
	}

	log.Debugw("http request", "type", method, "path", u.Path, "body", jsonBody)
	var resp *http.Response
	for i := 1; i <= c.retries; i++ {
		resp, err = c.c.Do(&http.Request{
			Method: method,
			URL:    u,
			Header: headers,
			Body: func() io.ReadCloser {
				if jsonBody == nil {
					return nil
				}
				return io.NopCloser(bytes.NewBuffer(body))
			}(),
		})
		if resp != nil && resp.StatusCode == apirest.HTTPstatusServiceUnavailable { // mempool is full
			log.Warnf("mempool is full, will wait and retry (%d/%d)", i, c.retries)
			_ = c.WaitUntilNextBlock()
			continue
		}
		break
	}
	if err != nil {
		return nil, 0, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}
