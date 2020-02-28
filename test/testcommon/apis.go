package testcommon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
	voclient "github.com/tendermint/tendermint/rpc/client"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	dnet "gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"

	"gitlab.com/vocdoni/go-dvote/log"
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
func (d *DvoteAPIServer) Start(logLevel string, apis []string) error {
	log.InitLogger(logLevel, "stdout")
	var err error
	rand.Seed(time.Now().UnixNano())
	rint := rand.Int()
	// create signer
	d.Signer = new(signature.SignKeys)
	d.Signer.Generate()

	// create the proxy to handle HTTP queries
	pxy := dnet.NewProxy()
	pxy.C.Address = "127.0.0.1"
	pxy.C.Port = 0
	err = pxy.Init()
	if err != nil {
		return err
	}
	d.PxyAddr = fmt.Sprintf("ws://%s/dvote", pxy.Addr)

	// Create WebSocket endpoint
	ws := new(dnet.WebsocketHandle)
	ws.Init(new(types.Connection))
	ws.SetProxy(pxy)

	// Create the listener for routing messages
	listenerOutput := make(chan types.Message)
	go ws.Listen(listenerOutput)

	// Create the API router
	d.IpfsDir, err = ioutil.TempDir("", fmt.Sprintf("ipfs%d", rint))
	if err != nil {
		return err
	}
	// defer os.RemoveAll(ipfsDir)
	ipfsStore := data.IPFSNewConfig(d.IpfsDir)
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		return err
	}
	routerAPI := router.InitRouter(listenerOutput, storage, ws, d.Signer)

	// Create the Census Manager and enable it trough the router
	var cm census.Manager
	d.CensusDir, err = ioutil.TempDir("", fmt.Sprintf("census%d", rint))
	if err != nil {
		return err
	}

	if err := cm.Init(d.CensusDir, ""); err != nil {
		return err
	}

	for _, a := range apis {
		switch a {
		case "file":
			routerAPI.EnableFileAPI()
		case "census":
			routerAPI.EnableCensusAPI(&cm)
		case "vote":
			vnode, err := NewMockVochainNode(d, logLevel)
			if err != nil {
				return err
			}
			sc, err := NewMockScrutinizer(d, vnode)
			if err != nil {
				return err
			}
			routerAPI.Scrutinizer = sc
			routerAPI.EnableVoteAPI(d.VochainRPCClient)
		}
	}

	go routerAPI.Route()
	ws.AddProxyHandler("/dvote")
	return nil
}

// APIConnection holds an API websocket connection
type APIConnection struct {
	Conn *websocket.Conn
}

// Connect starts a connection with the given endpoint
func (r *APIConnection) Connect(addr string) (err error) {
	r.Conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	return
}

// Request makes a request to the previously connected endpoint
func (r *APIConnection) Request(req types.MetaRequest, signer *signature.SignKeys) (*types.MetaResponse, error) {
	// t.Helper()
	method := req.Method

	var cmReq types.RequestMessage
	cmReq.MetaRequest = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Timestamp = int32(time.Now().Unix())
	if signer != nil {
		var err error
		cmReq.Signature, err = signer.SignJSON(cmReq.MetaRequest)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", method, err)
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	if err := r.Conn.WriteMessage(websocket.TextMessage, rawReq); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	_, message, err := r.Conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	var cmRes types.ResponseMessage
	if err := json.Unmarshal(message, &cmRes); err != nil {
		return nil, fmt.Errorf("%s: %v", method, err)
	}
	if cmRes.ID != cmReq.ID {
		return nil, fmt.Errorf("%s: %v", method, "request ID doesn't match")
	}
	if cmRes.Signature == "" {
		return nil, fmt.Errorf("%s: empty signature in response: %s", method, message)
	}
	return &cmRes.MetaResponse, nil
}
