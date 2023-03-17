package indexer

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
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
func (s *Indexer) ProcessInfo(pid []byte) (*indexertypes.Process, error) {
	startTime := time.Now()

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	procInner, err := queries.GetProcess(ctx, pid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProcessNotFound
		}
		return nil, err
	}
	log.Debugf("processInfo sqlite took %s", time.Since(startTime))
	return indexertypes.ProcessFromDB(&procInner), nil
}

// ProcessList returns a list of process identifiers (PIDs) registered in the Vochain.
// EntityID, searchTerm, namespace, status, and withResults are optional filters, if
// declared as zero-values will be ignored. SearchTerm is a partial or full PID.
// Status is one of READY, CANCELED, ENDED, PAUSED, RESULTS
func (s *Indexer) ProcessList(entityID []byte,
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
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	startTime := time.Now()
	procs, err := queries.SearchProcesses(ctx, indexerdb.SearchProcessesParams{
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
	log.Debugf("ProcessList sqlite took %s", time.Since(startTime))
	if err != nil {
		return nil, err
	}
	return procs, nil
}

// ProcessCount returns the number of processes of a given entityID indexed.
// If entityID is zero-value, returns the total number of processes of all entities.
func (s *Indexer) ProcessCount(entityID []byte) uint64 {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	if len(entityID) == 0 {
		count, err := queries.GetProcessCount(ctx)
		if err != nil {
			log.Errorf("could not get the process count: %v", err)
			return 0
		}
		return uint64(count)
	}
	count, err := s.EntityProcessCount(entityID)
	if err != nil {
		log.Errorf("processCount: cannot fetch entity process count: %v", err)
		return 0
	}
	return uint64(count)
}

// EntityList returns the list of entities indexed by the indexer
// searchTerm is optional, if declared as zero-value
// will be ignored. Searches against the ID field.
func (s *Indexer) EntityList(max, from int, searchTerm string) []types.HexBytes {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	entityIDs, err := queries.SearchEntities(ctx, indexerdb.SearchEntitiesParams{
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
		hexIDs[i] = types.HexBytes(id)
	}
	return hexIDs
}

// EntityProcessCount returns the number of processes that an entity holds
func (s *Indexer) EntityProcessCount(entityId []byte) (uint32, error) {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	count, err := queries.GetEntityProcessCount(ctx, entityId)
	if err != nil {
		return 0, err
	}
	return uint32(count), nil
}

// EntityCount return the number of entities indexed by the indexer
func (s *Indexer) EntityCount() uint64 {
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	count, err := queries.GetEntityCount(ctx)
	if err != nil {
		log.Errorf("could not get the entity count: %v", err)
		return 0
	}
	return uint64(count)
}

// Return whether a process must have live results or not
func (s *Indexer) isOpenProcess(processID []byte) (bool, error) {
	p, err := s.App.State.Process(processID, false)
	if err != nil {
		return false, err
	}
	if p == nil || p.EnvelopeType == nil {
		return false, fmt.Errorf("cannot fetch process %x or envelope type not defined", processID)
	}
	return !p.EnvelopeType.EncryptedVotes, nil
}

// compute results if the current heigh has scheduled ending processes
func (s *Indexer) computePendingProcesses(height uint32) {
	defer s.liveGoroutines.Add(-1)
	// We wait a random number of blocks (between 0 and 5) in order to decrease the collision risk
	// between several Oracles.
	targetHeight := s.App.Height() + uint32(util.RandomInt(0, 5))
	for !s.skipTargetHeightSleeps {
		if s.cancelCtx.Err() != nil {
			return // closing
		}
		if s.App.Height() >= targetHeight {
			break
		}
		time.Sleep(5 * time.Second)
	}
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	procIDs, err := queries.GetProcessIDsByResultsHeight(ctx, int64(height))
	if err != nil {
		log.Warn(err)
		return
	}
	for _, pid := range procIDs {
		start := time.Now()
		if err := s.ComputeResult(pid); err != nil {
			log.Warnf("cannot compute results for %x: (%v)", pid, err)
			continue
		}
		log.Infof("results computation on %x took %s", pid, time.Since(start).String())
	}
}

// newEmptyProcess creates a new empty process and stores it into the database.
// The process must exist on the Vochain state, else an error is returned.
func (s *Indexer) newEmptyProcess(pid []byte) error {
	p, err := s.App.State.Process(pid, false)
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
	currentBlockTime := time.Unix(s.App.TimestampStartBlock(), 0)

	compResultsHeight := uint32(0)
	if live, err := s.isOpenProcess(pid); err != nil {
		return fmt.Errorf("cannot check if process is live: %w", err)
	} else {
		if live {
			compResultsHeight = p.BlockCount + p.StartBlock + 1
		}
	}

	// Create and store process in the indexer database
	procParams := indexerdb.CreateProcessParams{
		ID:                pid,
		EntityID:          nonNullBytes(eid),
		StartBlock:        int64(p.StartBlock),
		EndBlock:          int64(p.BlockCount + p.StartBlock),
		ResultsHeight:     int64(compResultsHeight),
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
	log.Debugw("new indexer process",
		"processID", hex.EncodeToString(pid),
		"entityID", hex.EncodeToString(eid),
		"startBlock", procParams.StartBlock,
		"endBlock", procParams.EndBlock,
		"resultsHeight", procParams.ResultsHeight,
		"haveResults", procParams.HaveResults,
		"censusRoot", hex.EncodeToString(p.CensusRoot),
		"rollingCensusRoot", hex.EncodeToString(p.RollingCensusRoot),
		"rollingCensusSize", procParams.RollingCensusSize,
		"maxCensusSize", procParams.MaxCensusSize,
		"censusUri", p.GetCensusURI(),
		"censusOrigin", procParams.CensusOrigin,
		"status", procParams.Status,
		"namespace", procParams.Namespace,
		"creationTime", procParams.CreationTime,
		"sourceBlockHeight", procParams.SourceBlockHeight,
		"sourceNetworkID", procParams.SourceNetworkID,
		"metadata", procParams.Metadata,
	)

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	tx := queries.WithTx(s.blockTx)
	if _, err := tx.CreateProcess(ctx, procParams); err != nil {
		return fmt.Errorf("sql create process: %w", err)
	}

	return nil
}

// updateProcess synchronize those fields that can be updated on a existing process
// with the information obtained from the Vochain state
func (s *Indexer) updateProcess(pid []byte) error {
	p, err := s.App.State.Process(pid, false)
	if err != nil {
		return fmt.Errorf("updateProcess: cannot fetch process %x: %w", pid, err)
	}
	// TODO: remove from results table
	// TODO: hold a sql db transaction for multiple queries
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
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
	if models.ProcessStatus(previousStatus) != models.ProcessStatus_CANCELED &&
		p.Status == models.ProcessStatus_CANCELED {

		// We use two SQL queries, so use a transaction to apply them together.
		tx, err := s.sqlDB.Begin()
		if err != nil {
			return err
		}
		queries = queries.WithTx(tx)

		if _, err := queries.SetProcessResultsHeight(ctx, indexerdb.SetProcessResultsHeightParams{
			ID:            pid,
			ResultsHeight: 0,
		}); err != nil {
			return err
		}
		if _, err := queries.SetProcessResultsCancelled(ctx, pid); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// setResultsHeight updates the Rheight of any process whose ID is pid.
func (s *Indexer) setResultsHeight(pid []byte, height uint32) error {
	if height == 0 {
		panic("setting results height to 0?")
	}
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	if _, err := queries.SetProcessResultsHeight(ctx, indexerdb.SetProcessResultsHeightParams{
		ID:            pid,
		ResultsHeight: int64(height),
	}); err != nil {
		return err
	}
	return nil
}
