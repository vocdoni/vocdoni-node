package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"gitlab.com/vocdoni/go-dvote/crypto"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"nhooyr.io/websocket"
)

// Client holds an API websocket client.
type Client struct {
	Addr string
	Conn *websocket.Conn
}

// New starts a connection with the given endpoint address.
func New(addr string) (*Client, error) {
	conn, _, err := websocket.Dial(context.TODO(), addr, nil)
	if err != nil {
		return nil, err
	}
	return &Client{Addr: addr, Conn: conn}, nil
}

func (c *Client) Close() error {
	return c.Conn.Close(websocket.StatusNormalClosure, "")
}

// Request makes a request to the previously connected endpoint
func (c *Client) Request(req types.MetaRequest, signer *ethereum.SignKeys) (*types.MetaResponse, error) {
	method := req.Method
	req.Timestamp = int32(time.Now().Unix())
	reqInner, err := crypto.SortedMarshalJSON(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	var signature string
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

	log.Infof("request: %s", reqBody)
	if err := c.Conn.Write(context.TODO(), websocket.MessageText, reqBody); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	_, message, err := c.Conn.Read(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	log.Infof("response: %s", message)
	var respOuter types.ResponseMessage
	if err := json.Unmarshal(message, &respOuter); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	if respOuter.ID != reqOuter.ID {
		return nil, fmt.Errorf("%s: %v", method, "request ID doesn'tb match")
	}
	if respOuter.Signature == "" {
		return nil, fmt.Errorf("%s: empty signature in response: %s", method, message)
	}
	var respInner types.MetaResponse
	if err := json.Unmarshal(respOuter.MetaResponse, &respInner); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	return &respInner, nil
}

// Request makes a request to the previously connected endpoint
func (c *Client) ForTest(tb testing.TB, req *types.MetaRequest) func(method string, signer *ethereum.SignKeys) *types.MetaResponse {
	return func(method string, signer *ethereum.SignKeys) *types.MetaResponse {
		req.Method = method
		req.Timestamp = int32(time.Now().Unix())
		resp, err := c.Request(*req, signer)
		if err != nil {
			tb.Fatal(err)
		}
		if !resp.Ok {
			tb.Fatalf("%s failed: %s", req.Method, resp.Message)
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
	tb.Cleanup(func() { client.Conn.Close(websocket.StatusNormalClosure, "") })

	return &TestClient{tb: tb, client: *client}
}

func (c *TestClient) Request(req types.MetaRequest, signer *ethereum.SignKeys) *types.MetaResponse {
	resp, err := c.client.Request(req, signer)
	if err != nil {
		c.tb.Fatal(err)
	}
	return resp
}
