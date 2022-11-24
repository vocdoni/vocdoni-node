package testcommon

import (
	"fmt"
	"math/rand"
	"testing"

	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/rpcapi"
	"go.vocdoni.io/dvote/rpccensus"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// DvoteAPIServer contains all the required pieces for running a go-dvote api server
type DvoteAPIServer struct {
	Signer        *ethereum.SignKeys
	VochainCfg    *config.VochainCfg
	CensusDir     string
	CensusBackend string // graviton or asmt // NOTE: CensusBackend is not used
	IpfsDir       string
	IndexerDir    string
	ListenAddr    string
	Storage       data.Storage
	IpfsPort      int

	VochainAPP *vochain.BaseApplication
	Indexer    *indexer.Indexer
}

/*
Start starts a basic dvote server
1. Create signing key
2. Starts the Proxy
3. Starts the IPFS storage
4. Starts the Census Manager
5. Starts the Vochain miner if vote api enabled
6. Starts the Dvote API router if enabled
7. Starts the indexer service and API if enabled
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
	d.CensusDir = tb.TempDir()
	cDB, err := metadb.New(db.TypePebble, d.CensusDir)
	if err != nil {
		log.Fatal(err)
	}
	censusDB := censusdb.NewCensusDB(cDB)
	cm := rpccensus.NewCensusManager(censusDB, d.Storage)

	for _, api := range apis {
		switch api {
		case "file":
			rpc.EnableFileAPI(d.Storage)
		case "census":
			rpc.EnableCensusAPI(cm)
		case "vote":
			d.VochainAPP = NewMockVochainNode(tb, d.VochainCfg, d.Signer)
			vi := vochaininfo.NewVochainInfo(d.VochainAPP)
			go vi.Start(10)
			d.Indexer = NewMockIndexer(tb, d.VochainAPP)
			rpc.EnableVoteAPI(d.VochainAPP, vi)
			rpc.EnableResultsAPI(d.VochainAPP, d.Indexer)
			rpc.EnableIndexerAPI(d.VochainAPP, vi, d.Indexer)
		default:
			tb.Fatalf("unknown api: %q", api)
		}
	}
}
