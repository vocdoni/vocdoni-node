package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"nhooyr.io/websocket"
)

// Client holds an API client.
type Client struct {
	Addr string
	WS   *websocket.Conn
	HTTP *http.Client
}

// New starts a connection with the given endpoint address.
// Supported protocols are ws(s):// and http(s)://
func New(addr string) (*Client, error) {
	cli := &Client{Addr: addr}
	var err error
	if strings.HasPrefix(addr, "ws") {
		cli.WS, _, err = websocket.Dial(context.Background(), addr, nil)
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(addr, "http") {
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    10 * time.Second,
			DisableCompression: false,
		}
		cli.HTTP = &http.Client{Transport: tr, Timeout: time.Second * 5}
	} else {
		return nil, fmt.Errorf("address is not websockets nor http: %s", addr)
	}
	return cli, nil
}

func (c *Client) Close() error {
	var err error
	if c.WS != nil {
		err = c.WS.Close(websocket.StatusNormalClosure, "")
	}
	if c.HTTP != nil {
		c.HTTP.CloseIdleConnections()
	}
	return err
}

func (c *Client) CheckClose(err *error) {
	if clerr := c.Close(); clerr != nil {
		*err = clerr
	}
}

// Request makes a request to the previously connected endpoint
func (c *Client) Request(req types.MetaRequest, signer *ethereum.SignKeys) (*types.MetaResponse, error) {
	method := req.Method
	req.Timestamp = int32(time.Now().Unix())
	reqInner, err := crypto.SortedMarshalJSON(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	var signature []byte
	if signer != nil {
		signature, err = signer.Sign(reqInner)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", method, err)
		}
	}

	reqOuter := types.RequestMessage{
		ID:          fmt.Sprintf("%d", rand.Intn(1000)),
		Signature:   signature,
		MetaRequest: reqInner,
	}
	reqBody, err := json.Marshal(reqOuter)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}

	log.Debugf("request: %s", reqBody)
	message := []byte{}
	if c.WS != nil {
		tctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		if err := c.WS.Write(tctx, websocket.MessageText, reqBody); err != nil {
			return nil, fmt.Errorf("%s: %v", method, err)
		}
		_, message, err = c.WS.Read(tctx)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", method, err)
		}
	}
	if c.HTTP != nil {
		resp, err := c.HTTP.Post(c.Addr, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, err
		}
		message, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
	}
	log.Debugf("response: %s", message)
	var respOuter types.ResponseMessage
	if err := json.Unmarshal(message, &respOuter); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	if respOuter.ID != reqOuter.ID {
		return nil, fmt.Errorf("%s: %v", method, "request ID doesn'tb match")
	}
	if len(respOuter.Signature) == 0 {
		return nil, fmt.Errorf("%s: empty signature in response: %s", method, message)
	}
	var respInner types.MetaResponse
	if err := json.Unmarshal(respOuter.MetaResponse, &respInner); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	return &respInner, nil
}

// Request makes a request to the previously connected endpoint
func (c *Client) ForTest(tb testing.TB, req *types.MetaRequest) func(
	method string, signer *ethereum.SignKeys) *types.MetaResponse {
	return func(method string, signer *ethereum.SignKeys) *types.MetaResponse {
		if req == nil {
			tb.Fatalf("request is nil")
		}
		req.Method = method
		req.Timestamp = int32(time.Now().Unix())
		resp, err := c.Request(*req, signer)
		if err != nil {
			tb.Fatal(err)
		}
		return resp
	}
}

type TestClient struct {
	tb     testing.TB
	client Client
}

func NewForTest(tb testing.TB, addr string) *TestClient {
	client, err := New(addr)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = client.Close() })

	return &TestClient{tb: tb, client: *client}
}

func (c *TestClient) Request(req types.MetaRequest, signer *ethereum.SignKeys) *types.MetaResponse {
	resp, err := c.client.Request(req, signer)
	if err != nil {
		c.tb.Fatal(err)
	}
	return resp
}
