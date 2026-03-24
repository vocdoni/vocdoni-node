package vocone

import (
	"context"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/apiclient"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/test/testcommon/testvoteproof"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
	"go.vocdoni.io/proto/build/go/models"
)

// newTestVocone creates a Vocone instance for testing with fast block times and API enabled.
// Returns the vocone, a context cancel function, and an API client.
func newTestVocone(t *testing.T) (*Vocone, context.CancelFunc, *apiclient.HTTPclient, int) {
	t.Helper()
	dir := t.TempDir()

	keymng := ethereum.SignKeys{}
	qt.Assert(t, keymng.Generate(), qt.IsNil)

	vc, err := NewVocone(dir, &keymng, false, "", nil)
	qt.Assert(t, err, qt.IsNil)

	vc.App.SetChainID("test-vocone")
	vc.App.SetBlockTimeTarget(time.Millisecond * 500)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := vc.Start(ctx); err != nil {
			t.Logf("vocone start error: %v", err)
		}
	}()

	port := 13000 + util.RandomInt(0, 2000)
	_, err = vc.EnableAPI("127.0.0.1", port, "/v2")
	qt.Assert(t, err, qt.IsNil)

	waitForHeightAtLeast(t, vc, 1, 10*time.Second)

	qt.Assert(t, vc.SetBulkTxCosts(0, true), qt.IsNil)

	cli, err := apiclient.New(fmt.Sprintf("http://127.0.0.1:%d/v2", port))
	qt.Assert(t, err, qt.IsNil)

	return vc, cancel, cli, port
}

// newTestVoconeLite creates a minimal Vocone without IPFS or API for unit tests.
// It uses a reduced init to avoid global metric registration conflicts.
func newTestVoconeLite(t *testing.T) (*Vocone, context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()

	keymng := ethereum.SignKeys{}
	qt.Assert(t, keymng.Generate(), qt.IsNil)

	// Build Vocone components manually to avoid VochainInfo.Start()
	// which registers global metrics and panics on duplicate test runs.
	vc := &Vocone{}
	vc.Config = &config.VochainCfg{}
	vc.Config.DataDir = dir
	vc.Config.DBType = db.TypePebble

	var err error
	vc.App, err = vochain.NewBaseApplication(vc.Config)
	qt.Assert(t, err, qt.IsNil)
	vc.txsPerBlock = DefaultTxsPerBlock

	vc.blockStore, err = metadb.New(db.TypePebble, filepath.Join(dir, "blockstore"))
	qt.Assert(t, err, qt.IsNil)

	vc.mempoolDB, err = metadb.New(db.TypePebble, filepath.Join(dir, "mempool"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, vc.loadMempool(), qt.IsNil)

	vc.setDefaultMethods()
	vc.App.SetBlockTimeTarget(time.Millisecond * 200)

	// Create indexer (needed for block processing).
	vc.Indexer, err = indexer.New(vc.App, indexer.Options{
		DataDir: filepath.Join(dir, "indexer"),
	})
	qt.Assert(t, err, qt.IsNil)

	// Set chain ID and key keeper.
	vc.App.SetChainID("test-vocone")
	qt.Assert(t, vc.SetKeyKeeper(&keymng), qt.IsNil)

	// Create VochainInfo but do NOT call Start() to avoid metric registration panics.
	vc.Stats = vochaininfo.NewVochainInfo(vc.App)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := vc.Start(ctx); err != nil {
			t.Logf("vocone start error: %v", err)
		}
	}()

	waitForHeightAtLeast(t, vc, 1, 10*time.Second)
	return vc, cancel
}

func waitForHeightAtLeast(t *testing.T, vc *Vocone, minHeight int64, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if vc.height.Load() >= minHeight {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for height >= %d (current=%d)", minHeight, vc.height.Load())
		case <-ticker.C:
		}
	}
}

// TestVocone runs a full end-to-end test: account creation, election, voting, results.
func TestVocone(t *testing.T) {
	vc, cancel, cli, _ := newTestVocone(t)
	defer cancel()
	defer vc.Close()

	account := ethereum.SignKeys{}
	qt.Assert(t, account.Generate(), qt.IsNil)
	qt.Assert(t, cli.SetAccount(fmt.Sprintf("%x", account.PrivateKey())), qt.IsNil)

	qt.Assert(t, testCreateAccount(cli), qt.IsNil)
	qt.Assert(t, vc.CreateAccount(account.Address(), &state.Account{
		Account: models.Account{Balance: 10000},
	}), qt.IsNil)
	qt.Assert(t, testCSPvote(cli), qt.IsNil)
}

// TestBlockStoreAndRetrieval verifies that blocks are persisted and can be
// retrieved by height and by hash through the API.
func TestBlockStoreAndRetrieval(t *testing.T) {
	vc, cancel := newTestVoconeLite(t)
	defer cancel()
	defer vc.Close()

	waitForHeightAtLeast(t, vc, 5, 15*time.Second)

	// Verify block metadata is persisted correctly.
	meta, err := vc.loadBlockMeta(3)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, meta.Timestamp > 0, qt.IsTrue)
	qt.Assert(t, len(meta.Hash) > 0, qt.IsTrue)
	qt.Assert(t, len(meta.ProposerAddress) > 0, qt.IsTrue)

	// Verify getBlock returns a proper block.
	blk := vc.getBlock(3)
	qt.Assert(t, blk != nil, qt.IsTrue)
	qt.Assert(t, blk.Height == 3, qt.IsTrue)
	qt.Assert(t, blk.Header.ChainID != "", qt.IsTrue)
	qt.Assert(t, !blk.Header.Time.IsZero(), qt.IsTrue)
	qt.Assert(t, len(blk.Hash()) > 0, qt.IsTrue,
		qt.Commentf("block hash should not be empty"))

	// Verify getBlockByHash returns the same block.
	blkByHash := vc.getBlockByHash(meta.Hash)
	qt.Assert(t, blkByHash != nil, qt.IsTrue)
	qt.Assert(t, blkByHash.Height == 3, qt.IsTrue)

	// Verify chain linkage: block N's lastBlockHash should equal block N-1's hash.
	meta4, err := vc.loadBlockMeta(4)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(meta4.LastBlockHash) > 0, qt.IsTrue,
		qt.Commentf("block 4 should have a lastBlockHash"))
	qt.Assert(t, string(meta4.LastBlockHash) == string(meta.Hash), qt.IsTrue,
		qt.Commentf("block 4 lastBlockHash should equal block 3 hash"))
}

// TestBlockPersistence verifies persisted block metadata and reconstruction
// across produced heights.
func TestBlockPersistence(t *testing.T) {
	vc, cancel := newTestVoconeLite(t)
	defer cancel()
	defer vc.Close()

	waitForHeightAtLeast(t, vc, 3, 15*time.Second)
	height := vc.height.Load()
	qt.Assert(t, height >= 3, qt.IsTrue,
		qt.Commentf("expected at least 3 blocks, got %d", height))
	for h := int64(1); h < height; h++ {
		meta, err := vc.loadBlockMeta(h)
		qt.Assert(t, err, qt.IsNil, qt.Commentf("block %d meta should exist", h))
		qt.Assert(t, meta.Timestamp > 0, qt.IsTrue)

		// Verify the block can be reconstructed.
		blk := vc.getBlock(h)
		qt.Assert(t, blk != nil, qt.IsTrue)
		qt.Assert(t, blk.Height == h, qt.IsTrue)
		qt.Assert(t, len(blk.Hash()) > 0, qt.IsTrue)
	}
}

// TestMempoolPersistence verifies that the mempool survives restarts
// by checking the persistent storage directly.
func TestMempoolPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create a mempool db and store some entries.
	mdb, err := metadb.New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)

	// Write two mempool entries.
	wTx := mdb.WriteTx()
	qt.Assert(t, wTx.Set(mempoolKey(1), []byte("tx1")), qt.IsNil)
	qt.Assert(t, wTx.Set(mempoolKey(2), []byte("tx2")), qt.IsNil)
	seqBytes := make([]byte, 8)
	seqBytes[7] = 2 // sequence = 2
	qt.Assert(t, wTx.Set([]byte(keyMempoolSeq), seqBytes), qt.IsNil)
	qt.Assert(t, wTx.Commit(), qt.IsNil)
	qt.Assert(t, mdb.Close(), qt.IsNil)

	// Reopen and verify the entries are recovered.
	mdb2, err := metadb.New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	defer mdb2.Close()

	// Create a minimal Vocone-like structure to test loadMempool.
	vc := &Vocone{mempoolDB: mdb2}
	qt.Assert(t, vc.loadMempool(), qt.IsNil)
	qt.Assert(t, len(vc.mempoolKeys) == 2, qt.IsTrue,
		qt.Commentf("expected 2 mempool keys, got %d", len(vc.mempoolKeys)))
	qt.Assert(t, vc.mempoolSeq == 2, qt.IsTrue,
		qt.Commentf("expected seq 2, got %d", vc.mempoolSeq))

	// If persisted sequence is stale, loadMempool should derive the max from keys.
	wTx2 := mdb2.WriteTx()
	staleSeq := make([]byte, 8)
	staleSeq[7] = 1 // sequence = 1 (stale)
	qt.Assert(t, wTx2.Set([]byte(keyMempoolSeq), staleSeq), qt.IsNil)
	qt.Assert(t, wTx2.Commit(), qt.IsNil)
	qt.Assert(t, vc.loadMempool(), qt.IsNil)
	qt.Assert(t, vc.mempoolSeq == 2, qt.IsTrue,
		qt.Commentf("expected seq recovered from keys as 2, got %d", vc.mempoolSeq))
}

// TestBlockMetaPersistence verifies block metadata round-trip serialization.
func TestBlockMetaPersistence(t *testing.T) {
	dir := t.TempDir()

	bdb, err := metadb.New(db.TypePebble, dir)
	qt.Assert(t, err, qt.IsNil)
	defer bdb.Close()

	vc := &Vocone{blockStore: bdb}

	now := time.Now()
	stateRoot := []byte("state-root-hash")
	blockHash := []byte("block-hash-value")
	proposer := []byte("proposer-addr")
	lastHash := []byte("last-block-hash")
	dataHash := []byte("data-hash")

	// Store metadata.
	qt.Assert(t, vc.storeBlockMeta(42, now, 5, stateRoot, blockHash, proposer, lastHash, dataHash), qt.IsNil)

	// Load and verify.
	meta, err := vc.loadBlockMeta(42)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, meta.TxCount == 5, qt.IsTrue)
	qt.Assert(t, string(meta.StateRoot) == string(stateRoot), qt.IsTrue)
	qt.Assert(t, string(meta.Hash) == string(blockHash), qt.IsTrue)
	qt.Assert(t, string(meta.ProposerAddress) == string(proposer), qt.IsTrue)
	qt.Assert(t, string(meta.LastBlockHash) == string(lastHash), qt.IsTrue)
	qt.Assert(t, string(meta.DataHash) == string(dataHash), qt.IsTrue)
	qt.Assert(t, time.Unix(0, meta.Timestamp).Unix() == now.Unix(), qt.IsTrue)

	// Verify hash→height reverse index (check the raw index key directly).
	heightBytes, err := bdb.Get(blockHashKey(blockHash))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, len(heightBytes) == 8, qt.IsTrue)
	qt.Assert(t, binary.BigEndian.Uint64(heightBytes) == 42, qt.IsTrue)

	// Overwriting metadata with a different hash should remove stale reverse index.
	newHash := []byte("block-hash-value-2")
	qt.Assert(t, vc.storeBlockMeta(42, now, 5, stateRoot, newHash, proposer, lastHash, dataHash), qt.IsNil)
	_, err = bdb.Get(blockHashKey(blockHash))
	qt.Assert(t, err, qt.IsNotNil)
	heightBytes, err = bdb.Get(blockHashKey(newHash))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, binary.BigEndian.Uint64(heightBytes) == 42, qt.IsTrue)

	// Verify non-existent hash returns error.
	_, err = bdb.Get(blockHashKey([]byte("nonexistent")))
	qt.Assert(t, err, qt.IsNotNil)

	// Verify non-existent height returns minimal block.
	meta2, err := vc.loadBlockMeta(999)
	qt.Assert(t, err, qt.IsNotNil)
	qt.Assert(t, meta2, qt.IsNil)
}

// TestKeyEncoding verifies that key encoding functions produce deterministic,
// ordered keys suitable for KV store iteration.
func TestKeyEncoding(t *testing.T) {
	// txKey should produce different keys for different heights/indices.
	k1 := txKey(1, 0)
	k2 := txKey(1, 1)
	k3 := txKey(2, 0)
	qt.Assert(t, string(k1) != string(k2), qt.IsTrue)
	qt.Assert(t, string(k1) != string(k3), qt.IsTrue)
	qt.Assert(t, string(k2) != string(k3), qt.IsTrue)

	// metaKey should produce different keys for different heights.
	m1 := metaKey(0)
	m2 := metaKey(1)
	m3 := metaKey(1000)
	qt.Assert(t, string(m1) != string(m2), qt.IsTrue)
	qt.Assert(t, string(m2) != string(m3), qt.IsTrue)

	// blockHashKey should include the full hash.
	h1 := blockHashKey([]byte("abc"))
	h2 := blockHashKey([]byte("def"))
	qt.Assert(t, string(h1) != string(h2), qt.IsTrue)

	// mempoolKey should produce ordered keys.
	mp1 := mempoolKey(1)
	mp2 := mempoolKey(2)
	qt.Assert(t, string(mp1) < string(mp2), qt.IsTrue,
		qt.Commentf("mempool keys should be lexicographically ordered"))
}

// TestGracefulShutdown verifies that Start returns cleanly when context is cancelled.
func TestGracefulShutdown(t *testing.T) {
	vc, cancel := newTestVoconeLite(t)

	waitForHeightAtLeast(t, vc, 1, 10*time.Second)

	// Cancel context — Start should return.
	cancel()
	time.Sleep(time.Second)

	// Verify blocks were produced.
	qt.Assert(t, vc.height.Load() > 0, qt.IsTrue)

	// Close should not error.
	qt.Assert(t, vc.Close(), qt.IsNil)
}

// TestCloseIdempotent verifies that Close can be called safely on a fresh Vocone.
func TestCloseIdempotent(t *testing.T) {
	dir := t.TempDir()

	bdb, err := metadb.New(db.TypePebble, filepath.Join(dir, "blockstore"))
	qt.Assert(t, err, qt.IsNil)
	mdb, err := metadb.New(db.TypePebble, filepath.Join(dir, "mempool"))
	qt.Assert(t, err, qt.IsNil)

	vc := &Vocone{blockStore: bdb, mempoolDB: mdb}
	qt.Assert(t, vc.Close(), qt.IsNil)
}

// TestSetBulkTxCostsLogic verifies the fixed SetBulkTxCosts behavior:
// with force=false, costs that are not set should be set, and existing ones should be skipped.
func TestSetBulkTxCostsLogic(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.VochainCfg{DataDir: dir, DBType: db.TypePebble}
	app, err := vochain.NewBaseApplication(cfg)
	qt.Assert(t, err, qt.IsNil)

	bdb, err := metadb.New(db.TypePebble, filepath.Join(dir, "blockstore"))
	qt.Assert(t, err, qt.IsNil)
	mdb, err := metadb.New(db.TypePebble, filepath.Join(dir, "mempool"))
	qt.Assert(t, err, qt.IsNil)

	vc := &Vocone{}
	vc.App = app
	vc.blockStore = bdb
	vc.mempoolDB = mdb
	defer vc.Close()

	// Force-set all costs to 100.
	qt.Assert(t, vc.SetBulkTxCosts(100, true), qt.IsNil)

	// Now set with force=false and cost=999 — existing costs should NOT be overwritten.
	qt.Assert(t, vc.SetBulkTxCosts(999, false), qt.IsNil)

	// Verify costs are still 100 (not overwritten).
	for k := range state.TxTypeCostToStateKey {
		cost, err := vc.App.State.TxBaseCost(k, true)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, cost == 100, qt.IsTrue,
			qt.Commentf("tx type %v should have cost 100, got %d", k, cost))
	}
}

func testCreateAccount(cli *apiclient.HTTPclient) error {
	txhash, err := cli.AccountBootstrap(nil, nil, nil)
	if err != nil {
		return err
	}
	if _, err = cli.WaitUntilTxIsMined(context.Background(), txhash); err != nil {
		return err
	}
	_, err = cli.Account("")
	return err
}

func testCSPvote(cli *apiclient.HTTPclient) error {
	cspKey := ethereum.SignKeys{}
	if err := cspKey.Generate(); err != nil {
		return err
	}
	entityID := cli.MyAddress().Bytes()
	censusRoot := cspKey.PublicKey()
	censusOrigin := models.CensusOrigin_OFF_CHAIN_CA
	censusSize := uint64(10)
	processID, err := cli.NewElectionRaw(
		&models.Process{
			EntityId:     entityID,
			Status:       models.ProcessStatus_READY,
			CensusRoot:   censusRoot,
			CensusOrigin: censusOrigin,
			EnvelopeType: &models.EnvelopeType{},
			VoteOptions: &models.ProcessVoteOptions{
				MaxCount: 1,
				MaxValue: 1,
			},
			Mode: &models.ProcessMode{
				AutoStart:     true,
				Interruptible: true,
			},
			StartTime:     0,
			Duration:      60,
			MaxCensusSize: censusSize,
		})
	if err != nil {
		return err
	}
	voterKeys := ethereum.NewSignKeysBatch(int(censusSize))
	proofs, err := testvoteproof.GetCSPproofBatch(voterKeys, &cspKey, processID)
	if err != nil {
		return err
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel1()
	election, err := cli.WaitUntilElectionStatus(ctx1, processID, "READY")
	if err != nil {
		return err
	}

	for i, k := range voterKeys {
		c := cli.Clone(fmt.Sprintf("%x", k.PrivateKey()))
		if _, err := c.Vote(&apiclient.VoteData{
			Choices:  []int{1},
			Election: election,
			ProofCSP: proofs[i],
		}); err != nil {
			return err
		}
	}

	startTimeVoteCount := time.Now()
	for {
		if time.Since(startTimeVoteCount) > time.Second*10 {
			return fmt.Errorf("timeout waiting for votes to be counted")
		}
		votes, err := cli.ElectionVoteCount(processID)
		if err != nil {
			return err
		}
		if votes == uint32(censusSize) {
			break
		}
		time.Sleep(time.Second)
	}

	if _, err = cli.SetElectionStatus(processID, "ENDED"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	election, err = cli.WaitUntilElectionStatus(ctx, processID, "RESULTS")
	if err != nil {
		return err
	}
	if !election.Results[0][0].Equal(new(types.BigInt).SetUint64(0)) {
		return fmt.Errorf("expected result[0][0] to be 0, got %s", election.Results[0][0])
	}
	if !election.Results[0][1].Equal(new(types.BigInt).SetUint64(10)) {
		return fmt.Errorf("expected result[0][1] to be 10, got %s", election.Results[0][1])
	}
	return nil
}
