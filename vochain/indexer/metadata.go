package indexer

import (
	"context"
	"time"

	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
)

// metadataWriteTimeout bounds how long a metadata write waits for the single
// read-write connection. The columns it fills are a cache of off-chain data, so
// giving up is always preferable to queueing behind a long-running writer such
// as a block reindex: the value will be written again the next time the same
// election or account is resolved, or by the next boot's backfill.
const metadataWriteTimeout = 5 * time.Second

// SetProcessMetadataTitle stores the title resolved from the off-chain metadata
// of a process, so that list endpoints can render it without resolving the
// metadata per row. It is a no-op when the stored title is already the given one.
//
// Callers should not block a request on this: it competes for the same single
// read-write connection block commits use.
func (idx *Indexer) SetProcessMetadataTitle(processID []byte, title string) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataWriteTimeout)
	defer cancel()
	_, err := indexerdb.New(idx.readWriteDB).SetProcessMetadataTitle(ctx, indexerdb.SetProcessMetadataTitleParams{
		ID:            processID,
		MetadataTitle: title,
	})
	return err
}

// SetAccountMetadata stores the name and avatar resolved from the off-chain
// metadata of an account, so that the organization list can render them without
// resolving the metadata per row. It is a no-op when both are already stored, or
// when the account has not been indexed yet.
//
// Callers should not block a request on this: it competes for the same single
// read-write connection block commits use.
func (idx *Indexer) SetAccountMetadata(accountID []byte, name, avatar string) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataWriteTimeout)
	defer cancel()
	_, err := indexerdb.New(idx.readWriteDB).SetAccountMetadata(ctx, indexerdb.SetAccountMetadataParams{
		Account: accountID,
		Name:    name,
		Avatar:  avatar,
	})
	return err
}

// ProcessMetadataURI is a process whose title has not been resolved yet, paired
// with the metadata URI to resolve it from.
type ProcessMetadataURI struct {
	ProcessID []byte
	URI       string
}

// ProcessesMissingMetadataTitle returns up to limit processes which declare a
// metadata URI but whose title was never resolved, in process id order starting
// after afterID. Used to backfill the elections created before the title was
// indexed, or whose metadata was unreachable back then.
func (idx *Indexer) ProcessesMissingMetadataTitle(afterID []byte, limit int) ([]ProcessMetadataURI, error) {
	rows, err := idx.readOnlyQuery.ListProcessesMissingMetadataTitle(context.TODO(),
		indexerdb.ListProcessesMissingMetadataTitleParams{
			// an empty blob sorts before every process id, so a nil cursor starts
			// from the beginning rather than comparing against NULL
			AfterID: nonNullBytes(afterID),
			Limit:   int64(limit),
		})
	if err != nil {
		return nil, err
	}
	list := make([]ProcessMetadataURI, 0, len(rows))
	for _, row := range rows {
		list = append(list, ProcessMetadataURI{ProcessID: row.ID, URI: row.Metadata})
	}
	return list, nil
}

// AccountsMissingName returns up to limit accounts whose name was never resolved,
// in account id order starting after afterAccount. The metadata URI lives in the
// account state rather than in the indexer, so the caller resolves it from there.
func (idx *Indexer) AccountsMissingName(afterAccount []byte, limit int) ([][]byte, error) {
	rows, err := idx.readOnlyQuery.ListAccountsMissingName(context.TODO(),
		indexerdb.ListAccountsMissingNameParams{
			// an empty blob sorts before every account id, see above
			AfterAccount: nonNullBytes(afterAccount),
			Limit:        int64(limit),
		})
	if err != nil {
		return nil, err
	}
	list := make([][]byte, 0, len(rows))
	for _, row := range rows {
		list = append(list, row)
	}
	return list, nil
}
