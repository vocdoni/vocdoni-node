package test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/log"
	dnet "gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/test/testcommon"
	"gitlab.com/vocdoni/go-dvote/types"
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
	node, err := NewMockEthereum(*logLevel, testcommon.TempDir(t, "ethereum"), pxy)
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
	c, _, err := websocket.DefaultDialer.Dial(pxyAddr, nil)
	if err != nil {
		t.Fatalf("cannot dial web3ws: %s", err)
	}
	defer c.Close()
	// send requests
	for _, tt := range testRequests {
		t.Run(tt.name, func(t *testing.T) {
			// write message
			tt.request.Jsonrpc = "2.0"
			reqBytes, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("cannot marshal request: %s", err)
			}
			log.Infof("testing: %s, sending request: %v", tt.name, tt.request)
			err = c.WriteMessage(websocket.TextMessage, reqBytes)
			if err != nil {
				t.Fatalf("cannot write to ws: %s", err)
			}
			// read message
			_, message, err := c.ReadMessage()
			if err != nil {
				t.Fatalf("cannot read message: %s", err)
			}
			log.Infof("response: %s", message)
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
	// send close message
	err = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		t.Fatalf("write close: %s", err)
	}
}

// NewMockEthereum creates an ethereum node, attaches a signing key and adds a http or ws endpoint to a given proxy
func NewMockEthereum(logLevel, dataDir string, pxy *dnet.Proxy) (*chain.EthChainContext, error) {
	// create base config
	ethConfig := &config.EthCfg{
		LogLevel:  logLevel,
		DataDir:   dataDir,
		ChainType: "goerli",
	}
	w3Config := &config.W3Cfg{
		HTTPHost: "0.0.0.0",
		WsHost:   "0.0.0.0",
		Route:    "/web3",
		Enabled:  true,
		HTTPAPI:  true,
		WSAPI:    true,
		// TODO(mvdan): use 0 to grab a random unused port instead.
		HTTPPort: 1025 + rand.Intn(50000),
		WsPort:   1025 + rand.Intn(50000),
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
	pxy.AddHandler(w3Config.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
	pxy.AddWsHandler(w3Config.Route+"ws", pxy.AddWsHTTPBridge(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
	return node, nil
}
