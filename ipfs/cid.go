package ipfs

import (
	"bytes"
	"strings"

	"github.com/ipfs/boxo/blockservice"
	chunk "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	ihelper "github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	ipfscid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/ipfs/boxo/ipld/merkledag"

	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"go.vocdoni.io/dvote/log"
)

const (
	// ChunkerTypeSize is the chunker type used by IPFS to calculate to build the DAG.
	ChunkerTypeSize = "size-262144"
)

// dAG returns a new, thread-safe, dummy DAGService.
func dAG() ipld.DAGService {
	return merkledag.NewDAGService(bserv())
}

// bserv returns a new, thread-safe, mock BlockService.
func bserv() blockservice.BlockService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	return blockservice.New(bstore, offline.Exchange(bstore))
}

// CalculateCIDv1json calculates the IPFS Cid hash (v1) from a bytes buffer,
// using parameters Codec: DagJSON, MhType: SHA2_256
func CalculateCIDv1json(data []byte) string {
	// Note that we cannot just use ipfscid.NewCidV1() and hash the data.
	// We need to create a DAG node, set the chunker size correctly and then create
	// the IPLD node to get the correct Cid.
	// This is because when the data exceeds the chunker size, the chunker, the resulting
	// hash of the CID is the merkle root of the DAG, not the hash of the data.
	// For files smaller than the chunker size, the hash of the CID is the hash of the data, but
	// we cannot make this assumption.
	chnk, err := chunk.FromString(bytes.NewReader(data), ChunkerTypeSize)
	if err != nil {
		log.Errorw(err, "could not create chunk")
	}

	format := ipfscid.V1Builder{
		Codec:  uint64(multicodec.DagJson),
		MhType: uint64(multihash.SHA2_256),
	}
	params := ihelper.DagBuilderParams{
		Dagserv:    dAG(),
		RawLeaves:  false,
		Maxlinks:   ihelper.DefaultLinksPerBlock,
		NoCopy:     false,
		CidBuilder: &format,
	}

	dbh, err := params.New(chnk)
	if err != nil {
		log.Errorw(err, "could not create dag builder")
	}
	nd, err := balanced.Layout(dbh)
	if err != nil {
		log.Errorw(err, "could not create balanced layout")
	}

	return nd.Cid().String()
}

// CIDequals compares two Cids (v0 or v1) and returns true if they are equal.
// It compares the hash of the Cid, not the Cid itself (which contains also the codec and encoding).
// It strips the ipfs:// prefix and the /ipfs/ prefix if present.
func CIDequals(cid1, cid2 string) bool {
	cid1 = strings.TrimPrefix(strings.TrimPrefix(cid1, "ipfs://"), "/ipfs/")
	cid2 = strings.TrimPrefix(strings.TrimPrefix(cid2, "ipfs://"), "/ipfs/")
	c1, err := ipfscid.Decode(cid1)
	if err != nil {
		log.Errorw(err, "could not decode cid1 "+cid1)
		return false
	}
	c2, err := ipfscid.Decode(cid2)
	if err != nil {
		log.Errorw(err, "could not decode cid2 "+cid2)
		return false
	}

	// skip version length
	/*	b := c1.Bytes()
		_, n1, _ := varint.FromUvarint(b)
		// skip codec length
		_, n2, _ := varint.FromUvarint(b[n1:])
		log.Warnf("==1==> %x", b[n1+n2:])

		b = c2.Bytes()
		_, n1, _ = varint.FromUvarint(b)
		// skip codec length
		_, n2, _ = varint.FromUvarint(b[n1:])
		log.Warnf("==2==> %x", b[n1+n2:])

		log.Warnf("cid1=%s cid2=%s | cid1hash=%s cid2hash=%s", c1.String(), c2.String(), c1.Hash().String(), c2.Hash().String())
	*/
	return c1.Hash().String() == c2.Hash().String()
}
