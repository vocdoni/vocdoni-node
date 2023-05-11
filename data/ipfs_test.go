package data

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	ipfscid "github.com/ipfs/go-cid"
	"go.vocdoni.io/dvote/ipfs"
)

func TestIPFSCIDv1(t *testing.T) {
	msg := []byte("{\"test\": { \"hello\": \"world\" }}")
	expectedCid := "bafybeiatc7haowqj7aq4lnnz4wnci6mhznuoykrme5mfwa3fvypxwv4gfu"

	i := MockIPFS(t)
	cidFromPublish, err := i.Publish(context.Background(), msg)
	qt.Assert(t, err, qt.IsNil)
	cid, err := ipfscid.Decode(cidFromPublish)
	qt.Assert(t, err, qt.IsNil)
	t.Log("Publish returned:")
	t.Logf("CID: %s, Hash: %s, Version: %d, MhType: %x, Codec: %x",
		cid.String(), cid.Hash(), cid.Version(), cid.Prefix().MhType, cid.Type())

	cidFromCalculateIPFSCIDv1json := ipfs.CalculateCIDv1json(msg)
	cid, err = ipfscid.Decode(cidFromCalculateIPFSCIDv1json)
	qt.Assert(t, err, qt.IsNil)
	t.Log("CalculateCIDv1json returned:")
	t.Logf("CID: %s, Hash: %s, Version: %d, MhType: %x, Codec: %x",
		cid.String(), cid.Hash(), cid.Version(), cid.Prefix().MhType, cid.Type())

	qt.Assert(t,
		ipfs.CIDequals(expectedCid, cidFromCalculateIPFSCIDv1json),
		qt.Equals, true)
	qt.Assert(t,
		ipfs.CIDequals(cidFromPublish, cidFromCalculateIPFSCIDv1json),
		qt.Equals, true)
}
