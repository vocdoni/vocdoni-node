package data

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	ipfscid "github.com/ipfs/go-cid"
)

func TestIPFSCIDv1(t *testing.T) {
	msg := []byte("{\"test\": { \"hello\": \"world\" }}")
	expectedCid := "bagaaiera63prsqlm2mptglagfk6ywr4cv3ibcxbpapi2rjzzufnilp52crvq"

	i := MockIPFS(t)
	cidFromPublish, err := i.Publish(context.Background(), msg)
	qt.Assert(t, err, qt.IsNil)
	cid, err := ipfscid.Decode(cidFromPublish)
	qt.Assert(t, err, qt.IsNil)
	t.Log("Publish returned:")
	t.Logf("CID: %s, Hash: %s, Version: %d, MhType: %x, Codec: %x",
		cid.String(), cid.Hash(), cid.Version(), cid.Prefix().MhType, cid.Type())

	cidFromCalculateIPFSCIDv1json := CalculateIPFSCIDv1json(msg)
	cid, err = ipfscid.Decode(cidFromCalculateIPFSCIDv1json)
	qt.Assert(t, err, qt.IsNil)
	t.Log("CalculateIPFSCIDv1json returned:")
	t.Logf("CID: %s, Hash: %s, Version: %d, MhType: %x, Codec: %x",
		cid.String(), cid.Hash(), cid.Version(), cid.Prefix().MhType, cid.Type())

	qt.Assert(t, cidFromCalculateIPFSCIDv1json, qt.Equals, expectedCid)
	qt.Assert(t, cidFromPublish, qt.Equals, cidFromCalculateIPFSCIDv1json)
}
