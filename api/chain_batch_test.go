package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"testing"

	cmtbytes "github.com/cometbft/cometbft/libs/bytes"
	cometcoretypes "github.com/cometbft/cometbft/rpc/core/types"
	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/data/ipfs"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/apirest"
	"go.vocdoni.io/dvote/test/testcommon/testutil"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
)

// TestClassifyTransactionBatch checks the fail-fast grouping: submission stops at
// the first failure, every input item lands in exactly one group, and each item
// carries its predicted processId (including the unsent pending ones).
func TestClassifyTransactionBatch(t *testing.T) {
	c := qt.New(t)

	txs := []TransactionPayload{
		{Payload: []byte("a")},
		{Payload: []byte("b")},
		{Payload: []byte("bad")}, // fails to submit
		{Payload: []byte("d")},   // must never be submitted (fail-fast)
	}

	// predicted processId = "pid-" + payload, for every item.
	processID := func(p []byte) []byte { return append([]byte("pid-"), p...) }

	var sent [][]byte
	send := func(p []byte) (hash []byte, code uint32, err error) {
		sent = append(sent, p)
		if string(p) == "bad" {
			return nil, 0, fmt.Errorf("boom")
		}
		return append([]byte("hash-"), p...), 0, nil
	}

	res := classifyTransactionBatch(txs, processID, send)

	// grouping
	c.Assert(res.Submitted, qt.HasLen, 2)
	c.Assert(res.Failed, qt.HasLen, 1)
	c.Assert(res.Pending, qt.HasLen, 1)
	// every input item appears exactly once
	c.Assert(len(res.Submitted)+len(res.Failed)+len(res.Pending), qt.Equals, len(txs))

	// fail-fast: only "a", "b", "bad" were ever submitted; "d" was not.
	c.Assert(sent, qt.HasLen, 3)
	c.Assert(sent[2], qt.DeepEquals, []byte("bad"))

	// the failed item carries its error and predicted id
	c.Assert(res.Failed[0].Error, qt.Equals, "boom")
	c.Assert(res.Failed[0].ProcessID, qt.DeepEquals, types.HexBytes("pid-bad"))
	// the pending (unsent) item still carries its predicted id and no hash
	c.Assert(res.Pending[0].ProcessID, qt.DeepEquals, types.HexBytes("pid-d"))
	c.Assert(res.Pending[0].Hash, qt.IsNil)
	// submitted items carry their hash
	c.Assert(res.Submitted[0].Hash, qt.DeepEquals, types.HexBytes("hash-a"))
}

// TestChainSendTxBatchHandler exercises the HTTP handler end to end against a test
// app whose mempool is stubbed: a payload equal to "bad" fails to submit.
func TestChainSendTxBatchHandler(t *testing.T) {
	c := qt.New(t)

	router := httprouter.HTTProuter{}
	router.Init("127.0.0.1", 0)
	addr, err := url.Parse("http://" + path.Join(router.Address().String(), "chain"))
	c.Assert(err, qt.IsNil)

	api, err := NewAPI(&router, "/", t.TempDir(), db.TypePebble)
	c.Assert(err, qt.IsNil)
	kv, err := metadb.New(db.TypePebble, t.TempDir())
	c.Assert(err, qt.IsNil)
	censusDB := censusdb.NewCensusDB(kv)
	storage := ipfs.MockIPFS(t)
	app := vochain.TestBaseApplication(t)
	// stub the mempool: a tx whose raw payload is "bad" is rejected at submit.
	app.SetFnSendTx(func(tx []byte) (*cometcoretypes.ResultBroadcastTx, error) {
		if string(tx) == "bad" {
			return nil, fmt.Errorf("rejected")
		}
		return &cometcoretypes.ResultBroadcastTx{Hash: cmtbytes.HexBytes("hash"), Code: 0}, nil
	})
	idx, err := indexer.New(app, indexer.Options{DataDir: t.TempDir()})
	c.Assert(err, qt.IsNil)
	api.Attach(app, nil, idx, storage, censusDB)
	c.Assert(api.EnableHandlers(ChainHandler), qt.IsNil)

	token := uuid.New()
	cl := testutil.NewTestHTTPclient(t, addr, &token)

	// ok, bad, ok -> submitted:1, failed:1 (fail-fast), pending:1
	body := &TransactionBatch{Transactions: []TransactionPayload{
		{Payload: []byte("ok1")},
		{Payload: []byte("bad")},
		{Payload: []byte("ok2")},
	}}
	resp, code := cl.Request("POST", body, "transactions", "batch")
	c.Assert(code, qt.Equals, apirest.HTTPstatusOK)
	result := &TransactionBatchResult{}
	c.Assert(json.Unmarshal(resp, result), qt.IsNil)
	c.Assert(result.Submitted, qt.HasLen, 1)
	c.Assert(result.Failed, qt.HasLen, 1)
	c.Assert(result.Pending, qt.HasLen, 1)
	c.Assert(result.Failed[0].Error, qt.Not(qt.Equals), "")
	c.Assert(result.Submitted[0].Hash, qt.Not(qt.IsNil))

	// empty batch -> dedicated 400 error
	_, code = cl.Request("POST", &TransactionBatch{Transactions: []TransactionPayload{}}, "transactions", "batch")
	c.Assert(code, qt.Equals, ErrTransactionBatchEmpty.HTTPstatus)
}
