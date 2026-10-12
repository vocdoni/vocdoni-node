package indexer

import (
	"context"

	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
)

// GetIPFSContent retrieves raw IPFS content stored in the local cache by its bare
// CID (without the ipfs:// prefix). It returns sql.ErrNoRows when the CID is not
// present, so callers can distinguish "absent" from other errors.
func (idx *Indexer) GetIPFSContent(cid string) ([]byte, error) {
	return idx.readOnlyQuery.GetIPFSContent(context.TODO(), cid)
}

// PutIPFSContent stores raw IPFS content keyed by its bare CID (without the
// ipfs:// prefix). The INSERT OR IGNORE semantics make it safe to call multiple
// times: an existing row is left untouched and the call succeeds.
func (idx *Indexer) PutIPFSContent(cid string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataWriteTimeout)
	defer cancel()
	_, err := indexerdb.New(idx.readWriteDB).InsertIPFSContent(ctx, indexerdb.InsertIPFSContentParams{
		Cid:  cid,
		Data: data,
	})
	return err
}
