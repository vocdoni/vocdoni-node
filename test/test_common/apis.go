package test_common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	dnet "gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"

	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type DvoteApiServer struct {
	Signer    *signature.SignKeys
	CensusDir string
	IpfsDir   string
	PxyAddr   string
}

func (d *DvoteApiServer) Start(level string, t *testing.T) error {
	log.InitLogger(level, "stdout")
	d.Signer = new(signature.SignKeys)
	d.Signer.Generate()

	// create the proxy to handle HTTP queries
	pxy := dnet.NewProxy()
	pxy.C.Address = "127.0.0.1"
	pxy.C.Port = 0
	err := pxy.Init()
	if err != nil {
		t.Fatal(err)
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
	d.IpfsDir, err = ioutil.TempDir("", "ipfs")
	if err != nil {
		t.Fatal(err)
		return err
	}
	//defer os.RemoveAll(ipfsDir)
	ipfsStore := data.IPFSNewConfig(d.IpfsDir)
	storage, err := data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
	if err != nil {
		t.Fatalf("cannot start IPFS %s", err)
		return err
	}
	routerAPI := router.InitRouter(listenerOutput, storage, ws, d.Signer)

	// Create the Census Manager and enable it trough the router
	var cm census.Manager
	d.CensusDir, err = ioutil.TempDir("", "census")
	if err != nil {
		t.Fatal(err)
		return err
	}

	if err := cm.Init(d.CensusDir, ""); err != nil {
		t.Fatal(err)
		return err
	}
	routerAPI.EnableCensusAPI(&cm)

	go routerAPI.Route()
	ws.AddProxyHandler("/dvote")
	return nil
}

type ApiConnection struct {
	Conn *websocket.Conn
}

func (r *ApiConnection) Connect(addr string) (err error) {
	r.Conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	return
}

func (r *ApiConnection) Request(t *testing.T, req types.MetaRequest, signer *signature.SignKeys) types.MetaResponse {
	t.Helper()
	method := req.Method

	var cmReq types.RequestMessage
	cmReq.MetaRequest = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Timestamp = int32(time.Now().Unix())
	if signer != nil {
		var err error
		cmReq.Signature, err = signer.SignJSON(cmReq.MetaRequest)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if err := r.Conn.WriteMessage(websocket.TextMessage, rawReq); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	_, message, err := r.Conn.ReadMessage()
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	var cmRes types.ResponseMessage
	if err := json.Unmarshal(message, &cmRes); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	if cmRes.ID != cmReq.ID {
		t.Fatalf("%s: %v", method, "request ID doesn't match")
	}
	return cmRes.MetaResponse
}
