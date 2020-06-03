package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	dnet "gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/types"
	"nhooyr.io/websocket"
)

type jsonrpcRequestWrapper struct {
	ID      int
	Jsonrpc string
	Method  string
}

var testRequests = []struct {
	name    string
	request jsonrpcRequestWrapper
	result  interface{}
}{
	{
		name:    "net_listening",
		request: jsonrpcRequestWrapper{ID: 67, Method: "net_listening"},
		result:  true,
	},
	{
		name:    "net_version",
		request: jsonrpcRequestWrapper{ID: 67, Method: "net_version"},
		result:  "5",
	},
}

func TestWeb3WSEndpoint(t *testing.T) {
	t.Parallel()

	// create the proxy
	pxy := testcommon.NewMockProxy(t)
	// create ethereum node
	node, err := NewMockEthereum(testcommon.TempDir(t, "ethereum"), pxy)
	if err != nil {
		t.Fatalf("cannot create ethereum node: %s", err)
	}
	// start node
	node.Start()
	// TODO(mvdan): re-enable Node.Stop once
	// https://github.com/ethereum/go-ethereum/issues/20420 is fixed
	// defer node.Node.Stop()
	// proxy websocket handle
	pxyAddr := fmt.Sprintf("ws://%s/web3ws", pxy.Addr)
	// Create WebSocket endpoint
	ws := new(dnet.WebsocketHandle)
	ws.Init(new(types.Connection))
	ws.SetProxy(pxy)
	// Create the listener for routing messages
	listenerOutput := make(chan types.Message)
	go ws.Listen(listenerOutput)
	// create ws client
	c, _, err := websocket.Dial(context.TODO(), pxyAddr, nil)
	if err != nil {
		t.Fatalf("cannot dial web3ws: %s", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	// send requests
	for _, tt := range testRequests {
		t.Run(tt.name, func(t *testing.T) {
			// write message
			tt.request.Jsonrpc = "2.0"
			reqBytes, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("cannot marshal request: %s", err)
			}
			t.Logf("sending request: %v", tt.request)
			err = c.Write(context.TODO(), websocket.MessageText, reqBytes)
			if err != nil {
				t.Fatalf("cannot write to ws: %s", err)
			}
			// read message
			_, message, err := c.Read(context.TODO())
			if err != nil {
				t.Fatalf("cannot read message: %s", err)
			}
			t.Logf("response: %s", message)
			// check if response == expected
			var resp map[string]interface{}
			err = json.Unmarshal(message, &resp)
			if err != nil {
				t.Fatalf("cannot unmarshal response: %s", err)
			}

			if diff := cmp.Diff(resp["result"], tt.result); diff != "" {
				t.Fatalf("result not expected, diff is: %s", diff)
			}
		})
	}
}

// NewMockEthereum creates an ethereum node, attaches a signing key and adds a http or ws endpoint to a given proxy
func NewMockEthereum(dataDir string, pxy *dnet.Proxy) (*chain.EthChainContext, error) {
	// create base config
	ethConfig := &config.EthCfg{
		LogLevel:  "error",
		DataDir:   dataDir,
		ChainType: "goerli",
	}
	w3Config := &config.W3Cfg{
		RPCHost: "127.0.0.1",
		Route:   "/web3",
		Enabled: true,
		// TODO(mvdan): use 0 to grab a random unused port instead.
		RPCPort: 1025 + rand.Intn(50000),
	}
	// init node
	w3cfg, err := chain.NewConfig(ethConfig, w3Config)
	if err != nil {
		return nil, err
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		return nil, err
	}
	// register node endpoint
	pxy.AddHandler(w3Config.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3Config.RPCHost, w3Config.RPCPort)))
	pxy.AddWsHandler(w3Config.Route+"ws", pxy.AddWsHTTPBridge(fmt.Sprintf("http://%s:%d", w3Config.RPCHost, w3Config.RPCPort)))
	return node, nil
}
