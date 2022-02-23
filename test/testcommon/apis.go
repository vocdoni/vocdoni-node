package testcommon

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/rpcapi"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// DvoteAPIServer contains all the required pieces for running a go-dvote api server
type DvoteAPIServer struct {
	Signer         *ethereum.SignKeys
	VochainCfg     *config.VochainCfg
	CensusDir      string
	CensusBackend  string // graviton or asmt // NOTE: CensusBackend is not used
	IpfsDir        string
	ScrutinizerDir string
	ListenAddr     string
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

	// Create the API router
	d.IpfsDir = tb.TempDir()
	ipfsStore := data.IPFSNewConfig(d.IpfsDir)
	ipfs := data.IPFSHandle{}
	d.IpfsPort = 14000 + rand.Intn(2048)
	if err := ipfs.SetMultiAddress(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", d.IpfsPort)); err != nil {
		tb.Fatal(err)
	}
	if err := ipfs.Init(ipfsStore); err != nil {
		tb.Fatal(err)
	}
	d.Storage = &ipfs
	tb.Cleanup(func() {
		if err := d.Storage.Stop(); err != nil {
			tb.Error(err)
		}
	})

	httpRouter := httprouter.HTTProuter{}
	if err := httpRouter.Init("127.0.0.1", 0); err != nil {
		log.Fatal(err)
	}
	d.ListenAddr = fmt.Sprintf("http://%s/dvote", httpRouter.Address().String())

	// Initialize the RPC API
	rpc, err := rpcapi.NewAPI(d.Signer, &httpRouter, "/dvote", nil, true)
	if err != nil {
		log.Fatal(err)
	}

	// Create the Census Manager and enable it trough the router
	var cm census.Manager
	d.CensusDir = tb.TempDir()
	if err := cm.Start(db.TypePebble, d.CensusDir, ""); err != nil {
		tb.Fatal(err)
	}

	for _, api := range apis {
		switch api {
		case "file":
			rpc.EnableFileAPI(d.Storage)
		case "census":
			rpc.EnableCensusAPI(&cm)
		case "vote":
			d.VochainAPP = NewMockVochainNode(tb, d)
			vi := vochaininfo.NewVochainInfo(d.VochainAPP)
			go vi.Start(10 * time.Second)
			d.Scrutinizer = NewMockScrutinizer(tb, d, d.VochainAPP)
			rpc.EnableVoteAPI(d.VochainAPP, vi)
			rpc.EnableResultsAPI(d.VochainAPP, d.Scrutinizer)
			rpc.EnableIndexerAPI(d.VochainAPP, vi, d.Scrutinizer)
		default:
			tb.Fatalf("unknown api: %q", api)
		}
	}
}
