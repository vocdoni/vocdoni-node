package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/multirpc/transports"
	"github.com/vocdoni/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/chain"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/test/testcommon"
	"go.vocdoni.io/dvote/types"
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
	node := mockEthereum(t, pxy)
	// start node
	node.Start()
	defer node.Node.Close()
	// proxy websocket handle
	pxyAddr := fmt.Sprintf("ws://%s/web3ws", pxy.Addr)
	// Create WebSocket endpoint
	ws := new(mhttp.WebsocketHandle)
	ws.Init(new(transports.Connection))
	ws.SetProxy(pxy)
	// Create the listener for routing messages
	listenerOutput := make(chan transports.Message)
	ws.Listen(listenerOutput)
	// create ws client
	c, _, err := websocket.Dial(context.Background(), pxyAddr, nil)
	qt.Assert(t, err, qt.IsNil)
	defer c.Close(websocket.StatusNormalClosure, "")
	// send requests
	for _, tt := range testRequests {
		t.Run(tt.name, func(t *testing.T) {
			// write message
			tt.request.Jsonrpc = "2.0"
			reqBytes, err := json.Marshal(tt.request)
			qt.Assert(t, err, qt.IsNil)
			t.Logf("sending request: %v", tt.request)
			tctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			err = c.Write(tctx, websocket.MessageText, reqBytes)
			qt.Assert(t, err, qt.IsNil)
			// read message
			_, message, err := c.Read(tctx)
			qt.Assert(t, err, qt.IsNil)
			t.Logf("response: %s", message)
			// check if response == expected
			var resp map[string]interface{}
			err = json.Unmarshal(message, &resp)
			qt.Assert(t, err, qt.IsNil)

			qt.Assert(t, resp["result"], qt.Equals, tt.result)
		})
	}
}

// mockEthereum creates an ethereum node, attaches a signing key and adds a http or ws endpoint to a given proxy
func mockEthereum(t *testing.T, pxy *mhttp.Proxy) *chain.EthChainContext {
	// create base config
	ethConfig := &config.EthCfg{
		LogLevel:  "error",
		DataDir:   t.TempDir(),
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
	qt.Assert(t, err, qt.IsNil)
	node, err := chain.Init(w3cfg)
	qt.Assert(t, err, qt.IsNil)
	// register node endpoint
	pxy.AddHandler(w3Config.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3Config.RPCHost, w3Config.RPCPort)))
	pxy.AddWsHandler(w3Config.Route+"ws", pxy.AddWsHTTPBridge(fmt.Sprintf("http://%s:%d", w3Config.RPCHost, w3Config.RPCPort)), types.Web3WsReadLimit)
	return node
}
