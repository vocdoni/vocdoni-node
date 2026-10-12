package vocone

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
)

func TestParseImportGenesis(t *testing.T) {
	c := qt.New(t)
	// cometbft encodes initial_height as a string
	doc, err := ParseImportGenesis([]byte(`{"genesis_time":"2024-06-21T08:00:00Z","chain_id":"vocdoni/DEV/36",` +
		`"initial_height":"1","consensus_params":{},"app_state":{"max_election_size":100000}}`))
	c.Assert(err, qt.IsNil)
	c.Assert(doc.ChainID, qt.Equals, "vocdoni/DEV/36")
	c.Assert(doc.InitialHeight, qt.Equals, int64(1))
	c.Assert(doc.GenesisTime.Equal(time.Date(2024, 6, 21, 8, 0, 0, 0, time.UTC)), qt.IsTrue)
	c.Assert(string(doc.AppState), qt.Equals, `{"max_election_size":100000}`)

	doc, err = ParseImportGenesis([]byte(`{"chain_id":"vocone","initial_height":5}`))
	c.Assert(err, qt.IsNil)
	c.Assert(doc.InitialHeight, qt.Equals, int64(5))

	_, err = ParseImportGenesis([]byte(`{"initial_height":"1"}`))
	c.Assert(err, qt.IsNotNil)
	_, err = ParseImportGenesis([]byte(`{"chain_id":"x","initial_height":"one"}`))
	c.Assert(err, qt.IsNotNil)
}

// TestImportChain verifies that a state and indexer produced by another node are
// continued at the next height, keeping the chainID and balances, without applying
// a genesis.
func TestImportChain(t *testing.T) {
	c := qt.New(t)

	// Produce a source chain with a funded account, then stop it.
	src, cancel := newTestVoconeLite(t)
	account := ethereum.SignKeys{}
	c.Assert(account.Generate(), qt.IsNil)
	c.Assert(src.CreateAccount(account.Address(), &state.Account{
		Account: models.Account{Balance: 4242},
	}), qt.IsNil)
	waitForHeightAtLeast(t, src, 4, 10*time.Second)
	cancel()
	srcDir := src.Config.DataDir
	srcChainID := src.App.ChainID()
	c.Assert(src.Close(), qt.IsNil)

	// Copy only the state and the indexer, as from a node without a vocone block store.
	dir := t.TempDir()
	for _, sub := range []string{vochain.StateDataDir, "indexer"} {
		c.Assert(os.CopyFS(filepath.Join(dir, sub), os.DirFS(filepath.Join(srcDir, sub))), qt.IsNil)
	}

	doc := &genesis.Doc{ChainID: srcChainID, InitialHeight: 1, GenesisTime: time.Now().UTC()}
	res, err := ImportChain(dir, doc)
	c.Assert(err, qt.IsNil)
	c.Assert(res.ChainID, qt.Equals, srcChainID)
	c.Assert(res.StateHeight > 0, qt.IsTrue)
	c.Assert(res.FirstHeight, qt.Equals, res.StateHeight+1)
	c.Assert(res.IndexerHeight <= res.StateHeight, qt.IsTrue)

	// A second import into the same directory is rejected.
	_, err = ImportChain(dir, doc)
	c.Assert(err, qt.IsNotNil)

	keymng := ethereum.SignKeys{}
	c.Assert(keymng.Generate(), qt.IsNil)
	vc := openTestVocone(t, dir, &keymng)
	c.Assert(vc.Initialized(), qt.IsTrue)
	c.Assert(vc.App.ChainID(), qt.Equals, srcChainID)
	c.Assert(vc.App.Genesis().InitialHeight, qt.Equals, int64(1))
	c.Assert(vc.ImportBase(), qt.Equals, res.FirstHeight)
	c.Assert(vc.height.Load(), qt.Equals, res.FirstHeight)
	base, top := vc.blockStoreRange()
	c.Assert(base, qt.Equals, res.FirstHeight)
	c.Assert(top, qt.Equals, res.FirstHeight-1)

	acc, err := vc.App.State.GetAccount(account.Address(), true)
	c.Assert(err, qt.IsNil)
	c.Assert(acc, qt.IsNotNil)
	c.Assert(acc.Balance, qt.Equals, uint64(4242))

	// Explicit minting adds to an existing balance or creates the account.
	c.Assert(vc.MintBalance(account.Address(), 8), qt.IsNil)
	acc, err = vc.App.State.GetAccount(account.Address(), true)
	c.Assert(err, qt.IsNil)
	c.Assert(acc.Balance, qt.Equals, uint64(4250))
	other := ethereum.SignKeys{}
	c.Assert(other.Generate(), qt.IsNil)
	c.Assert(vc.MintBalance(other.Address(), 100), qt.IsNil)
	acc, err = vc.App.State.GetAccount(other.Address(), true)
	c.Assert(err, qt.IsNil)
	c.Assert(acc.Balance, qt.Equals, uint64(100))

	// The first produced block links to the last imported block, when it was indexed.
	c.Assert(vc.produceBlock(), qt.IsNil)
	blk := vc.getBlock(res.FirstHeight)
	c.Assert(blk, qt.IsNotNil)
	c.Assert(blk.ChainID, qt.Equals, srcChainID)
	if res.IndexerHeight == res.StateHeight {
		c.Assert(len(blk.LastBlockID.Hash) > 0, qt.IsTrue)
		c.Assert([]byte(blk.LastBlockID.Hash), qt.DeepEquals, []byte(vc.imported.LastBlockHash))
	}
	c.Assert(vc.Close(), qt.IsNil)

	// A restart resumes after the produced block, keeping the import base.
	vc = openTestVocone(t, dir, &keymng)
	defer func() { c.Assert(vc.Close(), qt.IsNil) }()
	c.Assert(vc.height.Load(), qt.Equals, res.FirstHeight+1)
	c.Assert(vc.ImportBase(), qt.Equals, res.FirstHeight)
	c.Assert(vc.App.ChainID(), qt.Equals, srcChainID)
}

// TestImportChainRejectsExistingChain verifies that a data directory that already
// holds a vocone chain, or no state at all, cannot be imported.
func TestImportChainRejectsExistingChain(t *testing.T) {
	c := qt.New(t)
	doc := &genesis.Doc{ChainID: "import-test"}

	_, err := ImportChain(t.TempDir(), doc)
	c.Assert(err, qt.IsNotNil)

	dir := t.TempDir()
	keymng := ethereum.SignKeys{}
	c.Assert(keymng.Generate(), qt.IsNil)
	vc := openTestVocone(t, dir, &keymng)
	c.Assert(vc.InitChain("existing", DefaultGenesisAppState(vc.txsPerBlock)), qt.IsNil)
	c.Assert(vc.produceBlock(), qt.IsNil)
	c.Assert(vc.Close(), qt.IsNil)
	_, err = ImportChain(dir, doc)
	c.Assert(err, qt.IsNotNil)
}
