package testcommon

import (
	"fmt"
	"math/rand"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

// APIserver contains all the required pieces for running an URL api server
type APIserver struct {
	Signer         *ethereum.SignKeys
	VochainCfg     *config.VochainCfg
	CensusDir      string
	IpfsDir        string
	ScrutinizerDir string
	ListenAddr     *url.URL
	Storage        data.Storage
	IpfsPort       int

	VochainAPP  *vochain.BaseApplication
	Scrutinizer *scrutinizer.Scrutinizer
	VochainInfo *vochaininfo.VochainInfo
}

// Start starts a basic URL API server for testing
func (d *APIserver) Start(t testing.TB, apis ...string) {
	// create signer
	d.Signer = ethereum.NewSignKeys()
	if err := d.Signer.Generate(); err != nil {
		t.Fatal(err)
	}

	// Create the API router
	d.IpfsDir = t.TempDir()
	ipfsStore := data.IPFSNewConfig(d.IpfsDir)
	ipfs := data.IPFSHandle{}
	d.IpfsPort = 14000 + rand.Intn(2048)
	if err := ipfs.SetMultiAddress(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", d.IpfsPort)); err != nil {
		t.Fatal(err)
	}
	if err := ipfs.Init(ipfsStore); err != nil {
		t.Fatal(err)
	}
	d.Storage = &ipfs
	t.Cleanup(func() {
		if err := d.Storage.Stop(); err != nil {
			t.Error(err)
		}
	})

	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + router.Address().String() + "/")
	qt.Assert(t, err, qt.IsNil)
	d.ListenAddr = addr
	t.Logf("address: %s", addr.String())

	api, err := api.NewAPI(&router, "/", t.TempDir())
	qt.Assert(t, err, qt.IsNil)

	d.VochainCfg = new(config.VochainCfg)
	d.VochainAPP = NewMockVochainNode(t, d.VochainCfg, d.Signer)
	d.VochainInfo = vochaininfo.NewVochainInfo(d.VochainAPP)
	go d.VochainInfo.Start(10)
	d.Scrutinizer = NewMockScrutinizer(t, d.VochainAPP)
	qt.Assert(t, d.VochainAPP.Service.Start(), qt.IsNil)

	api.Attach(d.VochainAPP, d.VochainInfo, d.Scrutinizer, d.Storage)

	err = api.EnableHandlers(apis...)
	qt.Assert(t, err, qt.IsNil)
}
