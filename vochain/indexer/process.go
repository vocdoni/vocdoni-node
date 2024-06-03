package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/log"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
)

var (
	// ErrProcessNotFound is returned if the process is not found in the indexer database.
	ErrProcessNotFound = fmt.Errorf("process not found")
	zeroBytes          = []byte("")
)

// nonNullBytes helps for sql CREATE queries, as most columns are NOT NULL.
func nonNullBytes(p []byte) []byte {
	if p == nil {
		return zeroBytes
	}
	return p
}

// TODO(mvdan): funcs to safely convert integers

// ProcessInfo returns the available information regarding an election process id
func (idx *Indexer) ProcessInfo(pid []byte) (*indexertypes.Process, error) {
	procInner, err := idx.readOnlyQuery.GetProcess(context.TODO(), pid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProcessNotFound
		}
		return nil, err
	}
	return indexertypes.ProcessFromDB(&procInner), nil
}

// ProcessList returns a list of process identifiers (PIDs) registered in the Vochain.
// EntityID, searchTerm, namespace, status, and withResults are optional filters, if
// declared as zero-values will be ignored. SearchTerm is a partial or full PID.
// Status is one of READY, CANCELED, ENDED, PAUSED, RESULTS
func (idx *Indexer) ProcessList(entityID []byte, from, max int, searchTerm string, namespace uint32,
	srcNetworkId int32, status string, withResults bool) ([][]byte, error) {
	if from < 0 {
		return nil, fmt.Errorf("processList: invalid value: from is invalid value %d", from)
	}
	// For filtering on Status we use a badgerhold match function.
	// If status is not defined, then the match function will return always true.
	statusnum := int32(0)
	statusfound := false
	if status != "" {
		if statusnum, statusfound = models.ProcessStatus_value[status]; !statusfound {
			return nil, fmt.Errorf("processList: status %s is unknown", status)
		}
	}
	// Filter match function for source network Id
	if _, ok := models.SourceNetworkId_name[srcNetworkId]; !ok {
		return nil, fmt.Errorf("sourceNetworkId is unknown %d", srcNetworkId)
	}

	procs, err := idx.readOnlyQuery.SearchProcesses(context.TODO(), indexerdb.SearchProcessesParams{
		EntityID:        nonNullBytes(entityID), // so that LENGTH never returns NULL
		Namespace:       int64(namespace),
		Status:          int64(statusnum),
		SourceNetworkID: int64(srcNetworkId),
		IDSubstr:        searchTerm,
		Offset:          int64(from),
		Limit:           int64(max),
		WithResults:     withResults,
	})
	if err != nil {
		return nil, err
	}
	return procs, nil
}

// CountTotalProcesses returns the total number of processes indexed.
func (idx *Indexer) CountTotalProcesses() uint64 {
	count, err := idx.readOnlyQuery.GetProcessCount(context.TODO())
	if err != nil {
		log.Errorf("could not get the process count: %v", err)
		return 0
	}
	return uint64(count)
}

// EntityList returns the list of entities indexed by the indexer
// searchTerm is optional, if declared as zero-value
// will be ignored. Searches against the ID field.
func (idx *Indexer) EntityList(max, from int, searchTerm string) []indexerdb.SearchEntitiesRow {
	rows, err := idx.readOnlyQuery.SearchEntities(context.TODO(), indexerdb.SearchEntitiesParams{
		EntityIDSubstr: searchTerm,
		Offset:         int64(from),
		Limit:          int64(max),
	})
	if err != nil {
		log.Errorf("error listing entities: %v", err)
		return nil
	}
	return rows
}

// CountTotalEntities return the total number of entities indexed by the indexer
func (idx *Indexer) CountTotalEntities() uint64 {
	count, err := idx.readOnlyQuery.GetEntityCount(context.TODO())
	if err != nil {
		log.Errorf("could not get the entity count: %v", err)
		return 0
	}
	return uint64(count)
}

// Return whether a process must have live results or not
func isOpenProcess(process *indexertypes.Process) bool {
	return !process.Envelope.EncryptedVotes
}

// newEmptyProcess creates a new empty process and stores it into the database.
// The process must exist on the Vochain state, else an error is returned.
func (idx *Indexer) newEmptyProcess(pid []byte) error {
	p, err := idx.App.State.Process(pid, false)
	if err != nil {
		return fmt.Errorf("cannot create new empty process: %w", err)
	}
	options := p.VoteOptions
	if options == nil {
		return fmt.Errorf("newEmptyProcess: vote options is nil")
	}
	if options.MaxCount == 0 {
		return fmt.Errorf("newEmptyProcess: maxCount is zero")
	}

	// Check for maxCount overflow
	if options.MaxCount > results.MaxQuestions {
		return fmt.Errorf("maxCount overflow %d", options.MaxCount)
	}

	eid := p.EntityId

	// Create and store process in the indexer database
	procParams := indexerdb.CreateProcessParams{
		ID:                pid,
		EntityID:          nonNullBytes(eid),
		StartDate:         time.Unix(int64(p.StartTime), 0),
		EndDate:           time.Unix(int64(p.StartTime+p.Duration), 0),
		ManuallyEnded:     false,
		VoteCount:         0,                              // an empty process has no votes yet
		HaveResults:       !p.EnvelopeType.EncryptedVotes, // like isOpenProcess, but on the state type
		CensusRoot:        nonNullBytes(p.CensusRoot),
		MaxCensusSize:     int64(p.GetMaxCensusSize()),
		CensusUri:         p.GetCensusURI(),
		CensusOrigin:      int64(p.CensusOrigin),
		Status:            int64(p.Status),
		Namespace:         int64(p.Namespace),
		Envelope:          indexertypes.EncodeProto(p.EnvelopeType),
		Mode:              indexertypes.EncodeProto(p.Mode),
		VoteOpts:          indexertypes.EncodeProto(p.VoteOptions),
		PrivateKeys:       indexertypes.EncodeJSON(p.EncryptionPrivateKeys),
		PublicKeys:        indexertypes.EncodeJSON(p.EncryptionPublicKeys),
		CreationTime:      time.Unix(idx.App.Timestamp(), 0),
		SourceBlockHeight: int64(p.GetSourceBlockHeight()),
		SourceNetworkID:   int64(p.SourceNetworkId),
		Metadata:          p.GetMetadata(),
		ResultsVotes:      indexertypes.EncodeJSON(results.NewEmptyVotes(options)),
		ChainID:           idx.App.ChainID(),
	}

	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateProcess(context.TODO(), procParams); err != nil {
		return fmt.Errorf("sql create process: %w", err)
	}
	return nil
}

// updateProcess synchronize those fields that can be updated on an existing process
// with the information obtained from the Vochain state
func (idx *Indexer) updateProcess(ctx context.Context, queries *indexerdb.Queries, pid []byte) error {
	p, err := idx.App.State.Process(pid, false)
	if err != nil {
		return fmt.Errorf("updateProcess: cannot fetch process %x: %w", pid, err)
	}

	dbProc, err := queries.GetProcess(ctx, pid)
	if err != nil {
		return fmt.Errorf("updateProcess: cannot fetch process %x: %w", pid, err)
	}
	previousStatus := dbProc.Status

	// We need to use the time of start/end from the blockchain state, as we might be syncing the blockchain
	currentBlockTime := time.Unix(idx.App.Timestamp(), 0)

	// Update the process in the indexer database
	if _, err := queries.UpdateProcessFromState(ctx, indexerdb.UpdateProcessFromStateParams{
		ID:            pid,
		CensusRoot:    nonNullBytes(p.CensusRoot),
		CensusUri:     p.GetCensusURI(),
		PrivateKeys:   indexertypes.EncodeJSON(p.EncryptionPrivateKeys),
		PublicKeys:    indexertypes.EncodeJSON(p.EncryptionPublicKeys),
		Metadata:      p.GetMetadata(),
		Status:        int64(p.Status),
		MaxCensusSize: int64(p.GetMaxCensusSize()),
	}); err != nil {
		return err
	}

	// If the process is in ENDED status, and it was not in ENDED status before, then update the end block
	if p.Status == models.ProcessStatus_ENDED &&
		models.ProcessStatus(previousStatus) != models.ProcessStatus_ENDED {
		// set mauallyEnded to true if the current bloc time is less than the original end date
		manuallyEnded := currentBlockTime.Before(dbProc.EndDate)
		if _, err := queries.UpdateProcessEndDate(ctx, indexerdb.UpdateProcessEndDateParams{
			ID:            pid,
			EndDate:       currentBlockTime,
			ManuallyEnded: manuallyEnded,
		}); err != nil {
			return err
		}
	}

	// If the process is in RESULTS status, and it was not in RESULTS status before, then finalize the results
	if p.Status == models.ProcessStatus_RESULTS &&
		models.ProcessStatus(previousStatus) != models.ProcessStatus_RESULTS {
		if err := idx.finalizeResults(ctx, queries, p); err != nil {
			return err
		}
	}

	// If the process is in CANCELED status, and it was not in CANCELED status before, then remove the results
	if models.ProcessStatus(previousStatus) != models.ProcessStatus_CANCELED &&
		p.Status == models.ProcessStatus_CANCELED {
		if _, err := queries.SetProcessResultsCancelled(ctx,
			indexerdb.SetProcessResultsCancelledParams{
				EndDate:       currentBlockTime,
				ManuallyEnded: true,
				ID:            pid,
			}); err != nil {
			return err
		}
	}
	return nil
}
