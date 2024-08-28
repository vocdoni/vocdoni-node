package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
// all args (entityID, processID, etc) are optional filters, if
// declared as zero-values will be ignored. entityID and processID are partial or full hex strings.
// Status is one of READY, CANCELED, ENDED, PAUSED, RESULTS
func (idx *Indexer) ProcessList(limit, offset int, entityID string, processID string,
	namespace uint32, srcNetworkID int32, status models.ProcessStatus,
	withResults, finalResults, manuallyEnded *bool,
	startDateAfter, startDateBefore, endDateAfter, endDateBefore *time.Time,
	title, description string,
) ([][]byte, uint64, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid value: offset cannot be %d", offset)
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid value: limit cannot be %d", limit)
	}
	// Filter match function for source network Id
	if _, ok := models.SourceNetworkId_name[srcNetworkID]; !ok {
		return nil, 0, fmt.Errorf("sourceNetworkId is unknown %d", srcNetworkID)
	}
	results, err := idx.readOnlyQuery.SearchProcesses(context.TODO(), indexerdb.SearchProcessesParams{
		EntityIDSubstr:  entityID,
		Namespace:       int64(namespace),
		Status:          int64(status),
		SourceNetworkID: int64(srcNetworkID),
		IDSubstr:        strings.ToLower(processID), // we search in lowercase
		Offset:          int64(offset),
		Limit:           int64(limit),
		HaveResults:     boolToInt(withResults),
		FinalResults:    boolToInt(finalResults),
		ManuallyEnded:   boolToInt(manuallyEnded),
		StartDateAfter:  startDateAfter,
		StartDateBefore: startDateBefore,
		EndDateAfter:    endDateAfter,
		EndDateBefore:   endDateBefore,
		Title:           title,
		Description:     description,
	})
	if err != nil {
		return nil, 0, err
	}
	list := [][]byte{}
	for _, row := range results {
		list = append(list, row.ID)
	}
	if len(results) == 0 {
		return list, 0, nil
	}
	return list, uint64(results[0].TotalCount), nil
}

// ProcessExists returns whether the passed processID exists in the db.
// If passed arg is not the full hex string, returns false (i.e. no substring matching)
func (idx *Indexer) ProcessExists(processID string) bool {
	if len(processID) != 64 {
		return false
	}
	_, count, err := idx.ProcessList(1, 0, "", processID, 0, 0, 0, nil, nil, nil, nil, nil, nil, nil, "", "")
	if err != nil {
		log.Errorw(err, "indexer query failed")
	}
	return count > 0
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
// entityID is optional, if declared as zero-value
// will be ignored. Searches against the entityID field as lowercase hex.
func (idx *Indexer) EntityList(limit, offset int, entityID string) ([]indexertypes.Entity, uint64, error) {
	if offset < 0 {
		return nil, 0, fmt.Errorf("invalid value: offset cannot be %d", offset)
	}
	if limit <= 0 {
		return nil, 0, fmt.Errorf("invalid value: limit cannot be %d", limit)
	}
	results, err := idx.readOnlyQuery.SearchEntities(context.TODO(), indexerdb.SearchEntitiesParams{
		EntityIDSubstr: strings.ToLower(entityID), // we search in lowercase
		Offset:         int64(offset),
		Limit:          int64(limit),
	})
	if err != nil {
		return nil, 0, err
	}
	list := []indexertypes.Entity{}
	for _, row := range results {
		list = append(list, indexertypes.Entity{
			EntityID:     row.EntityID,
			ProcessCount: row.ProcessCount,
		})
	}
	if len(results) == 0 {
		return list, 0, nil
	}
	return list, uint64(results[0].TotalCount), nil
}

// EntityExists returns whether the passed entityID exists in the db.
// If passed arg is not the full hex string, returns false (i.e. no substring matching)
func (idx *Indexer) EntityExists(entityID string) bool {
	if len(entityID) != 40 {
		return false
	}
	_, count, err := idx.EntityList(1, 0, entityID)
	if err != nil {
		log.Errorw(err, "indexer query failed")
	}
	return count > 0
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

// boolToInt returns -1 if pointer is nil, or 1 for true and 0 for false
func boolToInt(b *bool) int {
	if b == nil {
		return -1
	}
	if *b {
		return 1
	}
	return 0
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
		EndDate:       time.Unix(int64(p.StartTime+p.Duration), 0),
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
