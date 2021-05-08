package testcommon

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/vocdoni/multirpc/transports"
	"github.com/vocdoni/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/census"
	graviton "go.vocdoni.io/dvote/censustree/gravitontree"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/router"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
)

// DvoteAPIServer contains all the required pieces for running a go-dvote api server
type DvoteAPIServer struct {
	Signer         *ethereum.SignKeys
	VochainCfg     *config.VochainCfg
	CensusDir      string
	CensusBackend  string // graviton or asmt
	IpfsDir        string
	ScrutinizerDir string
	PxyAddr        string
	Storage        data.Storage
	IpfsPort       int

	VochainAPP  *vochain.BaseApplication
	Scrutinizer *scrutinizer.Scrutinizer
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
	d.Signer = ethereum.NewSignKeys()
	if err := d.Signer.Generate(); err != nil {
		tb.Fatal(err)
	}

	// create the proxy to handle HTTP queries
	pxy := NewMockProxy(tb)
	d.PxyAddr = fmt.Sprintf("http://%s/dvote", pxy.Addr)

	// Create WebSocket endpoint
	httpws := new(mhttp.HttpWsHandler)
	if err := httpws.Init(new(transports.Connection)); err != nil {
		tb.Fatal(err)
	}
	httpws.SetProxy(pxy)

	// Create the listener for routing messages
	listenerOutput := make(chan transports.Message)
	httpws.Listen(listenerOutput)

	// Create the API router
	var err error

	d.IpfsDir = tb.TempDir()
	ipfsStore := data.IPFSNewConfig(d.IpfsDir)
	ipfs := data.IPFSHandle{}
	d.IpfsPort = 14000 + rand.Intn(2048)
	if err = ipfs.SetMultiAddress(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", d.IpfsPort)); err != nil {
		tb.Fatal(err)
	}
	if err = ipfs.Init(ipfsStore); err != nil {
		tb.Fatal(err)
	}
	d.Storage = &ipfs
	tb.Cleanup(func() {
		if err := d.Storage.Stop(); err != nil {
			tb.Error(err)
		}
	})

	routerAPI := router.InitRouter(listenerOutput, d.Storage, d.Signer, nil, true)

	// Create the Census Manager and enable it trough the router
	var cm census.Manager
	d.CensusDir = tb.TempDir()
	if d.CensusBackend == "" || d.CensusBackend == "graviton" {
		if err := cm.Init(d.CensusDir, "", graviton.NewTree); err != nil {
			tb.Fatal(err)
		}
	} else {
		tb.Fatalf("census backend %s is unknown", d.CensusBackend)
	}

	for _, api := range apis {
		switch api {
		case "file":
			routerAPI.EnableFileAPI()
		case "census":
			routerAPI.EnableCensusAPI(&cm)
		case "vote":
			d.VochainAPP = NewMockVochainNode(tb, d)
			d.Scrutinizer = NewMockScrutinizer(tb, d, d.VochainAPP)
			routerAPI.Scrutinizer = d.Scrutinizer
			routerAPI.EnableVoteAPI(d.VochainAPP, nil)
			routerAPI.EnableResultsAPI(d.VochainAPP, nil)
			routerAPI.EnableIndexerAPI(d.VochainAPP, nil)
		default:
			tb.Fatalf("unknown api: %q", api)
		}
	}

	go routerAPI.Route()
	httpws.AddProxyHandler("/dvote")
}

// NewMockProxy creates a new testing proxy with predefined valudes
func NewMockProxy(tb testing.TB) *mhttp.Proxy {
	pxy := mhttp.NewProxy()
	pxy.Conn.Address = "127.0.0.1"
	pxy.Conn.Port = 0
	err := pxy.Init()
	if err != nil {
		tb.Fatal(err)
	}
	return pxy
}
