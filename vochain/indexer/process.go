package indexer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
)

var (
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

func encodedPb(msg proto.Message) types.EncodedProtoBuf {
	enc, err := proto.Marshal(msg)
	if err != nil {
		panic(err) // TODO(mvdan): propagate errors via a database/sql interface?
	}
	return nonNullBytes(enc)
}

func encodeVotes(votes [][]*types.BigInt) string {
	// "a,b,c x,y,z ..."
	var b strings.Builder
	for i, row := range votes {
		if i > 0 {
			b.WriteByte(' ')
		}
		for j, n := range row {
			if j > 0 {
				b.WriteByte(',')
			}
			text, err := n.MarshalText()
			if err != nil {
				panic(err) // TODO(mvdan): propagate errors via a database/sql interface?
			}
			b.Write(text)
		}
	}
	return b.String()
}

// ProcessInfo returns the available information regarding an election process id
func (idx *Indexer) ProcessInfo(pid []byte) (*indexertypes.Process, error) {
	procInner, err := idx.oneQuery.GetProcess(context.TODO(), pid)
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
func (idx *Indexer) ProcessList(entityID []byte,
	from,
	max int,
	searchTerm string,
	namespace uint32,
	srcNetworkId int32,
	status string,
	withResults bool) ([][]byte, error) {
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

	procs, err := idx.oneQuery.SearchProcesses(context.TODO(), indexerdb.SearchProcessesParams{
		EntityID:        entityID,
		EntityIDLen:     len(entityID), // see the TODO in queries/process.sql
		Namespace:       int64(namespace),
		Status:          int64(statusnum),
		SourceNetworkID: int64(srcNetworkId),
		IDSubstr:        searchTerm,
		Offset:          int32(from),
		Limit:           int32(max),
		WithResults:     withResults,
	})
	if err != nil {
		return nil, err
	}
	return procs, nil
}

// ProcessCount returns the number of processes of a given entityID indexed.
// If entityID is zero-value, returns the total number of processes of all entities.
func (idx *Indexer) ProcessCount(entityID []byte) uint64 {
	if len(entityID) == 0 {
		count, err := idx.oneQuery.GetProcessCount(context.TODO())
		if err != nil {
			log.Errorf("could not get the process count: %v", err)
			return 0
		}
		return uint64(count)
	}
	count, err := idx.EntityProcessCount(entityID)
	if err != nil {
		log.Errorf("processCount: cannot fetch entity process count: %v", err)
		return 0
	}
	return uint64(count)
}

// EntityList returns the list of entities indexed by the indexer
// searchTerm is optional, if declared as zero-value
// will be ignored. Searches against the ID field.
func (idx *Indexer) EntityList(max, from int, searchTerm string) []types.HexBytes {
	// TODO: EntityList callers in the api package want the process count as well;
	// work it out here so that the api package doesn't cause N sql queries.
	entityIDs, err := idx.oneQuery.SearchEntities(context.TODO(), indexerdb.SearchEntitiesParams{
		EntityIDSubstr: searchTerm,
		Offset:         int32(from),
		Limit:          int32(max),
	})
	if err != nil {
		log.Errorf("error listing entities: %v", err)
		return nil
	}
	hexIDs := make([]types.HexBytes, len(entityIDs))
	for i, id := range entityIDs {
		hexIDs[i] = id
	}
	return hexIDs
}

// EntityProcessCount returns the number of processes that an entity holds
func (idx *Indexer) EntityProcessCount(entityId []byte) (uint32, error) {
	count, err := idx.oneQuery.GetEntityProcessCount(context.TODO(), entityId)
	if err != nil {
		return 0, err
	}
	return uint32(count), nil
}

// EntityCount return the number of entities indexed by the indexer
func (idx *Indexer) EntityCount() uint64 {
	count, err := idx.oneQuery.GetEntityCount(context.TODO())
	if err != nil {
		log.Errorf("could not get the entity count: %v", err)
		return 0
	}
	return uint64(count)
}

// Return whether a process must have live results or not
func isOpenProcess(process *models.Process) bool {
	return !process.EnvelopeType.EncryptedVotes
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

	// Check for overflows
	if options.MaxCount > results.MaxQuestions || options.MaxValue > results.MaxOptions {
		return fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}

	eid := p.EntityId
	// Get the block time from the Header
	currentBlockTime := time.Unix(idx.App.TimestampStartBlock(), 0)

	compResultsHeight := uint32(0)
	if isOpenProcess(p) {
		compResultsHeight = p.BlockCount + p.StartBlock + 1
	}

	// Create and store process in the indexer database
	procParams := indexerdb.CreateProcessParams{
		ID:                pid,
		EntityID:          nonNullBytes(eid),
		StartBlock:        int64(p.StartBlock),
		EndBlock:          int64(p.BlockCount + p.StartBlock),
		HaveResults:       compResultsHeight > 0,
		CensusRoot:        nonNullBytes(p.CensusRoot),
		RollingCensusRoot: nonNullBytes(p.RollingCensusRoot),
		RollingCensusSize: int64(p.GetRollingCensusSize()),
		MaxCensusSize:     int64(p.GetMaxCensusSize()),
		CensusUri:         p.GetCensusURI(),
		CensusOrigin:      int64(p.CensusOrigin),
		Status:            int64(p.Status),
		Namespace:         int64(p.Namespace),
		EnvelopePb:        encodedPb(p.EnvelopeType),
		ModePb:            encodedPb(p.Mode),
		VoteOptsPb:        encodedPb(p.VoteOptions),
		PrivateKeys:       strings.Join(p.EncryptionPrivateKeys, ","),
		PublicKeys:        strings.Join(p.EncryptionPublicKeys, ","),
		CreationTime:      currentBlockTime,
		SourceBlockHeight: int64(p.GetSourceBlockHeight()),
		SourceNetworkID:   int64(p.SourceNetworkId),
		Metadata:          p.GetMetadata(),
		ResultsVotes:      encodeVotes(results.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1)),
	}

	queries := idx.blockTxQueries()
	if _, err := queries.CreateProcess(context.TODO(), procParams); err != nil {
		return fmt.Errorf("sql create process: %w", err)
	}

	return nil
}

// updateProcess synchronize those fields that can be updated on a existing process
// with the information obtained from the Vochain state
func (idx *Indexer) updateProcess(ctx context.Context, queries *indexerdb.Queries, pid []byte) error {
	p, err := idx.App.State.Process(pid, false)
	if err != nil {
		return fmt.Errorf("updateProcess: cannot fetch process %x: %w", pid, err)
	}

	previousStatus, err := queries.GetProcessStatus(ctx, pid)
	if err != nil {
		return err
	}
	if _, err := queries.UpdateProcessFromState(ctx, indexerdb.UpdateProcessFromStateParams{
		ID:                pid,
		EndBlock:          int64(p.BlockCount + p.StartBlock),
		CensusRoot:        nonNullBytes(p.CensusRoot),
		RollingCensusRoot: nonNullBytes(p.RollingCensusRoot),
		RollingCensusSize: int64(p.GetRollingCensusSize()),
		CensusUri:         p.GetCensusURI(),
		PrivateKeys:       strings.Join(p.EncryptionPrivateKeys, ","),
		PublicKeys:        strings.Join(p.EncryptionPublicKeys, ","),
		Metadata:          p.GetMetadata(),
		Status:            int64(p.Status),
	}); err != nil {
		return err
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
		if _, err := queries.SetProcessResultsCancelled(ctx, pid); err != nil {
			return err
		}
	}
	return nil
}
