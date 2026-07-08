package api

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.vocdoni.io/dvote/types"
)

// TestClassifyTransactionBatch checks the fail-fast grouping: submission stops at
// the first failure, every input item lands in exactly one group, and each item
// carries its predicted processId (including the unsent pending ones).
func TestClassifyTransactionBatch(t *testing.T) {
	c := qt.New(t)

	txs := []Transaction{
		{Payload: []byte("a")},
		{Payload: []byte("b")},
		{Payload: []byte("bad")}, // fails to submit
		{Payload: []byte("d")},   // must never be submitted (fail-fast)
	}

	// predicted processId = "pid-" + payload, for every item.
	processID := func(p []byte) []byte { return append([]byte("pid-"), p...) }

	var sent [][]byte
	send := func(p []byte) (hash, response []byte, code uint32, err error) {
		sent = append(sent, p)
		if string(p) == "bad" {
			return nil, nil, 0, fmt.Errorf("boom")
		}
		return append([]byte("hash-"), p...), nil, 0, nil
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
