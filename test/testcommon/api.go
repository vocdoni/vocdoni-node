package testcommon

import (
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// APIserver contains all the required pieces for running a mock API server.
// It is used for testing purposes only. The server starts a Vochain instance,
// the Indexer, the IPFS storage and the API router.
// The blockchain is advanced by calling the APIserver.VochainAPP.AdvanceTestBlock() method.
type APIserver struct {
	Account     *ethereum.SignKeys
	ListenAddr  *url.URL
	Storage     data.Storage
	VochainAPP  *vochain.BaseApplication
	Indexer     *indexer.Indexer
	VochainInfo *vochaininfo.VochainInfo
}

// Start starts a basic URL API server for testing
func (d *APIserver) Start(t testing.TB, apis ...string) {
	// create the account signer
	d.Account = ethereum.NewSignKeys()
	if err := d.Account.Generate(); err != nil {
		t.Fatal(err)
	}

	// create the IPFS storage
	d.Storage = &data.DataMockTest{}
	d.Storage.Init(&types.DataStore{Datadir: t.TempDir()})

	// create the API router
	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + router.Address().String() + "/")
	qt.Assert(t, err, qt.IsNil)
	d.ListenAddr = addr
	t.Logf("address: %s", addr.String())
	api, err := api.NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	qt.Assert(t, err, qt.IsNil)

	// create vochain application
	d.VochainAPP = vochain.TestBaseApplication(t)

	// create and add balance for the pre-created Account
	err = d.VochainAPP.State.CreateAccount(d.Account.Address(), "", nil, 1000000)
	qt.Assert(t, err, qt.IsNil)
	d.VochainAPP.CommitState()

	// create vochain info (we do not start since it is not required)
	d.VochainInfo = vochaininfo.NewVochainInfo(d.VochainAPP)

	// create indexer
	d.Indexer = NewMockIndexer(t, d.VochainAPP)

	// create census database
	db, err := metadb.New(db.TypePebble, t.TempDir())
	qt.Assert(t, err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(db)

	// attach all the pieces to the API
	api.Attach(d.VochainAPP, d.VochainInfo, d.Indexer, d.Storage, censusDB)

	// enable the required handlers
	err = api.EnableHandlers(apis...)
	qt.Assert(t, err, qt.IsNil)
}
