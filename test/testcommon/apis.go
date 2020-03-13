package testcommon

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	voclient "github.com/tendermint/tendermint/rpc/client"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	dnet "gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"

	"gitlab.com/vocdoni/go-dvote/types"
)

// DvoteAPIServer contains all the required pieces for running a go-dvote api server
type DvoteAPIServer struct {
	Signer           *signature.SignKeys
	VochainCfg       *config.VochainCfg
	VochainRPCClient *voclient.HTTP
	CensusDir        string
	IpfsDir          string
	ScrutinizerDir   string
	PxyAddr          string
}

/*
Start starts a basic dvote server
1. Create signing key
2. Starts the Proxy
3. Starts the IPFS storage
4. Starts the Census Manager
5. Starts the Vochain miner if vote api enabled
6. Starts the Dvote API router if enabled
7. Starts the scrutinizer service and API if enabled
*/
func (d *DvoteAPIServer) Start(tb testing.TB, apis ...string) {
	// create signer
	d.Signer = new(signature.SignKeys)
	d.Signer.Generate()

	// create the proxy to handle HTTP queries
	pxy := NewMockProxy(tb)
	d.PxyAddr = fmt.Sprintf("ws://%s/dvote", pxy.Addr)

	// Create WebSocket endpoint
	ws := new(dnet.WebsocketHandle)
	ws.Init(new(types.Connection))
	ws.SetProxy(pxy)

	// Create the listener for routing messages
	listenerOutput := make(chan types.Message)
	go ws.Listen(listenerOutput)

	// Create the API router
	d.IpfsDir = TempDir(tb, "dvote-ipfs")
	ipfsStore := data.IPFSNewConfig(d.IpfsDir)
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		tb.Fatal(err)
	}
	routerAPI := router.InitRouter(listenerOutput, storage, ws, d.Signer)

	// Create the Census Manager and enable it trough the router
	var cm census.Manager
	d.CensusDir = TempDir(tb, "census")

	if err := cm.Init(d.CensusDir, ""); err != nil {
		tb.Fatal(err)
	}

	for _, api := range apis {
		switch api {
		case "file":
			routerAPI.EnableFileAPI()
		case "census":
			routerAPI.EnableCensusAPI(&cm)
		case "vote":
			vnode := NewMockVochainNode(tb, d)
			sc := NewMockScrutinizer(tb, d, vnode)
			routerAPI.Scrutinizer = sc
			routerAPI.EnableVoteAPI(d.VochainRPCClient)
		default:
			tb.Fatalf("unknown api: %q", api)
		}
	}

	go routerAPI.Route()
	ws.AddProxyHandler("/dvote")
}

// APIConnection holds an API websocket connection
type APIConnection struct {
	tb   testing.TB
	Conn *websocket.Conn
}

// Connect starts a connection with the given endpoint
func (r *APIConnection) Connect(tb testing.TB, addr string) {
	tb.Helper()
	r.tb = tb
	var err error
	r.Conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		tb.Fatal(err)
	}
	r.tb.Cleanup(func() { r.Conn.Close() })
}

// Request makes a request to the previously connected endpoint
func (r *APIConnection) Request(req types.MetaRequest, signer *signature.SignKeys) *types.MetaResponse {
	r.tb.Helper()
	method := req.Method

	var cmReq types.RequestMessage
	cmReq.MetaRequest = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Timestamp = int32(time.Now().Unix())
	if signer != nil {
		var err error
		cmReq.Signature, err = signer.SignJSON(cmReq.MetaRequest)
		if err != nil {
			r.tb.Fatalf("%s: %v", method, err)
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		r.tb.Fatalf("%s: %v", method, err)
	}
	if err := r.Conn.WriteMessage(websocket.TextMessage, rawReq); err != nil {
		r.tb.Fatalf("%s: %v", method, err)
	}
	_, message, err := r.Conn.ReadMessage()
	if err != nil {
		r.tb.Fatalf("%s: %v", method, err)
	}
	var cmRes types.ResponseMessage
	if err := json.Unmarshal(message, &cmRes); err != nil {
		r.tb.Fatalf("%s: %v", method, err)
	}
	if cmRes.ID != cmReq.ID {
		r.tb.Fatalf("%s: %v", method, "request ID doesn'tb match")
	}
	if cmRes.Signature == "" {
		r.tb.Fatalf("%s: empty signature in response: %s", method, message)
	}
	return &cmRes.MetaResponse
}

// NewMockProxy creates a new testing proxy with predefined valudes
func NewMockProxy(tb testing.TB) *dnet.Proxy {
	pxy := dnet.NewProxy()
	pxy.C.Address = "127.0.0.1"
	pxy.C.Port = 0
	err := pxy.Init()
	if err != nil {
		tb.Fatal(err)
	}
	return pxy
}
