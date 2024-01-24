package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.vocdoni.io/dvote/data/downloader"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	maxArchiveFileSize      = 1024 * 100 // 100KB
	timeoutArchiveRetrieval = 120 * time.Second
	archiveFetchInterval    = 60 * time.Minute
	archiveFileNameSize     = types.ProcessIDsize * 2 // 64 hex chars
)

// ArchiveProcess is the struct used to store the process data in the archive.
type ArchiveProcess struct {
	ChainID     string                `json:"chainId,omitempty"` // Legacy
	ProcessInfo *indexertypes.Process `json:"process"`
	Results     *results.Results      `json:"results"`
	StartDate   *time.Time            `json:"startDate,omitempty"` // Legacy
	EndDate     *time.Time            `json:"endDate,omitempty"`   // Legacy
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

	queries := indexerdb.New(tx)
	added := []*ArchiveProcess{}
	for _, p := range archive {
		if idx.App.ChainID() == p.ChainID {
			// skip process from current chain
			continue
		}
		if p.ProcessInfo == nil {
			// skip process without process info
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

		// For backward compatibility, we try to fetch the start/end date from multiple sources.
		// If not found, we calculate them from the block count and the default block time.
		startDate := p.ProcessInfo.StartDate
		if startDate.IsZero() {
			if p.StartDate != nil {
				startDate = *p.StartDate
			} else {
				// Calculate startDate equal to time.Now() minus defaultBlockTime*p.ProcessInfo.BlockCount
				startDate = time.Now().Add(-types.DefaultBlockTime * time.Duration(p.ProcessInfo.BlockCount))
			}
		}
		endDate := p.ProcessInfo.EndDate
		if endDate.IsZero() {
			// Calculate endDate equal to startDate plus defaultBlockTime*p.ProcessInfo.BlockCount
			endDate = startDate.Add(types.DefaultBlockTime * time.Duration(p.ProcessInfo.BlockCount))
		}

		// Create and store process in the indexer database
		procParams := indexerdb.CreateProcessParams{
			ID:                nonNullBytes(p.ProcessInfo.ID),
			EntityID:          nonNullBytes(p.ProcessInfo.EntityID),
			StartBlock:        int64(p.ProcessInfo.StartBlock),
			StartDate:         startDate,
			EndBlock:          int64(p.ProcessInfo.EndBlock),
			EndDate:           endDate,
			BlockCount:        int64(p.ProcessInfo.BlockCount),
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
			CreationTime:      p.ProcessInfo.CreationTime,
			SourceBlockHeight: int64(p.ProcessInfo.SourceBlockHeight),
			SourceNetworkID:   int64(models.SourceNetworkId_value[p.ProcessInfo.SourceNetworkId]),
			Metadata:          p.ProcessInfo.Metadata,
			ResultsVotes:      indexertypes.EncodeJSON(p.Results.Votes),
			VoteCount:         int64(p.ProcessInfo.VoteCount),
			ChainID:           p.ChainID,
			FromArchive:       true,
		}

		if _, err := queries.CreateProcess(context.TODO(), procParams); err != nil {
			return nil, fmt.Errorf("create archive process: %w", err)
		}
		added = append(added, p)
	}
	return added, tx.Commit()
}

// StartArchiveRetrieval starts the archive retrieval process. It is a blocking function that runs continuously.
// Retrieves the archive directory from the storage and imports the processes into the indexer database.
func (idx *Indexer) StartArchiveRetrieval(storage *downloader.Downloader, archiveURL string) {
	if storage == nil || archiveURL == "" {
		log.Warnw("cannot start archive retrieval", "downloader", storage != nil, "url", archiveURL)
		return
	}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutArchiveRetrieval)
		dirMap, err := storage.RemoteStorage.RetrieveDir(ctx, archiveURL, maxArchiveFileSize)
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
				if p.ProcessInfo.Metadata != "" {
					storage.AddToQueue(p.ProcessInfo.Metadata, nil, true)
				}
			}
		}
		time.Sleep(archiveFetchInterval)
	}
}
