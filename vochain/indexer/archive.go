package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	marxArchiveFileSize     = 1024 * 100 // 100KB
	timeoutArchiveRetrieval = 120 * time.Second
	archiveFetchInterval    = 60 * time.Minute
	archiveFileNameSize     = types.ProcessIDsize * 2 // 64 hex chars
)

// ArchiveProcess is the struct used to store the process data in the archive.
type ArchiveProcess struct {
	ChainID     string                `json:"chainId,omitempty"`
	ProcessInfo *indexertypes.Process `json:"process"`
	Results     *results.Results      `json:"results"`
	StartDate   *time.Time            `json:"startDate,omitempty"`
	EndDate     *time.Time            `json:"endDate,omitempty"`
}

// ImportArchive imports an archive list of processes into the indexer database.
// It checks if the process already exists in the database and if not, it creates it.
// Returns those processes that have been added to the database.
func (idx *Indexer) ImportArchive(archive []*ArchiveProcess) ([]*ArchiveProcess, error) {
	tx, err := idx.readWriteDB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	height := idx.App.State.CurrentHeight()
	queries := indexerdb.New(tx)
	added := []*ArchiveProcess{}
	for _, p := range archive {
		if idx.App.ChainID() == p.ChainID {
			log.Debugw("skipping import of archive process from current chain", "chainID", p.ChainID, "processID", p.ProcessInfo.ID.String())
			continue
		}
		if p.ProcessInfo == nil {
			log.Debugw("skipping import of archive process with nil process info")
			continue
		}

		// Check if election already exists
		if _, err := idx.ProcessInfo(p.ProcessInfo.ID); err != nil {
			if err != ErrProcessNotFound {
				return nil, fmt.Errorf("process info: %w", err)
			}
		} else {
			continue
		}
		creationTime := time.Now()
		if p.StartDate != nil {
			creationTime = *p.StartDate
		}
		// Create and store process in the indexer database
		procParams := indexerdb.CreateProcessParams{
			ID:                nonNullBytes(p.ProcessInfo.ID),
			EntityID:          nonNullBytes(p.ProcessInfo.EntityID),
			StartBlock:        int64(height),
			EndBlock:          int64(height + 1),
			BlockCount:        int64(1),
			HaveResults:       p.ProcessInfo.HaveResults,
			FinalResults:      p.ProcessInfo.FinalResults,
			CensusRoot:        nonNullBytes(p.ProcessInfo.CensusRoot),
			MaxCensusSize:     int64(p.ProcessInfo.MaxCensusSize),
			CensusUri:         p.ProcessInfo.CensusURI,
			CensusOrigin:      int64(p.ProcessInfo.CensusOrigin),
			Status:            int64(p.ProcessInfo.Status),
			Namespace:         int64(p.ProcessInfo.Namespace),
			Envelope:          indexertypes.EncodeProtoJSON(p.ProcessInfo.Envelope),
			Mode:              indexertypes.EncodeProtoJSON(p.ProcessInfo.Mode),
			VoteOpts:          indexertypes.EncodeProtoJSON(p.ProcessInfo.VoteOpts),
			PrivateKeys:       indexertypes.EncodeJSON(p.ProcessInfo.PrivateKeys),
			PublicKeys:        indexertypes.EncodeJSON(p.ProcessInfo.PublicKeys),
			CreationTime:      creationTime,
			SourceBlockHeight: int64(p.ProcessInfo.SourceBlockHeight),
			SourceNetworkID:   int64(models.SourceNetworkId_value[p.ProcessInfo.SourceNetworkId]),
			Metadata:          p.ProcessInfo.Metadata,
			ResultsVotes:      indexertypes.EncodeJSON(p.Results.Votes),
			VoteCount:         int64(p.ProcessInfo.VoteCount),
		}
		if _, err := queries.CreateProcess(context.TODO(), procParams); err != nil {
			return nil, fmt.Errorf("create archive process: %w", err)
		}
		added = append(added, p)
	}
	return added, tx.Commit()
}

// StartArchiveRetrival starts the archive retrieval process. It is a blocking function that runs continuously.
// Retrieves the archive directory from the storage and imports the processes into the indexer database.
func (idx *Indexer) StartArchiveRetrival(storage data.Storage, archiveURL string) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutArchiveRetrieval)
		dirMap, err := storage.RetrieveDir(ctx, archiveURL, marxArchiveFileSize)
		cancel()
		if err != nil {
			log.Warnw("cannot retrieve archive directory", "url", archiveURL, "err", err)
			continue
		}
		archive := []*ArchiveProcess{}
		for name, data := range dirMap {
			if len(data) == 0 {
				continue
			}
			if len(name) != archiveFileNameSize {
				continue
			}
			var p ArchiveProcess
			if err := json.Unmarshal(data, &p); err != nil {
				log.Warnw("cannot unmarshal archive process", "name", name, "err", err)
				continue
			}
			archive = append(archive, &p)
		}

		log.Debugw("archive processes unmarshaled", "processes", len(archive))
		added, err := idx.ImportArchive(archive)
		if err != nil {
			log.Warnw("cannot import archive", "err", err)
		}
		if len(added) > 0 {
			log.Infow("new archive imported", "count", len(added))
			for _, p := range added {
				ctx, cancel := context.WithTimeout(context.Background(), timeoutArchiveRetrieval)
				if err := storage.Pin(ctx, p.ProcessInfo.Metadata); err != nil {
					log.Warnw("cannot pin metadata", "err", err.Error())
				}
				cancel()
			}
		}
		time.Sleep(archiveFetchInterval)
	}
}
