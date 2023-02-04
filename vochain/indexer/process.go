package indexer

import (
	"flag"
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
)

var zeroBytes = []byte("")

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

func sqliteWarnf(format string, args ...interface{}) {
	// We only complain when running the tests.
	// In production, the two databases are modified in parallel.
	// For example, when a process is finished, one database is updated
	// immediately after the other.
	// If a client query comes in between those two write operations,
	// the response to the query might see inconsistent results from the DBs.
	// That is fine for production.
	//
	// We still want to catch the inconsistencies while running the tests,
	// because they do help catch SQL schema and query bugs.
	// Tests tend to be sequential with their DBs, so there aren't such races.
	if flag.Lookup("test.run") != nil {
		log.Fatalf(format, args...)
	}
}

// ProcessInfo returns the available information regarding an election process id
func (s *Indexer) ProcessInfo(pid []byte) (*indexertypes.Process, error) {
	sqlStartTime := time.Now()

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlProcInner, err := queries.GetProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	log.Debugf("ProcessInfo sqlite took %s", time.Since(sqlStartTime))
	sqlProc := indexertypes.ProcessFromDB(&sqlProcInner)

	return sqlProc, nil
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
	srcNetworkIdstr,
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
	if srcNetworkIdstr != "" {
		if _, ok := models.SourceNetworkId_value[srcNetworkIdstr]; !ok {
			return nil, fmt.Errorf("sourceNetworkId is unknown %s", srcNetworkIdstr)
		}
	}
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	sqlStartTime := time.Now()
	sqlProcs, err := queries.SearchProcesses(ctx, indexerdb.SearchProcessesParams{
		EntityID:        entityID,
		EntityIDLen:     len(entityID), // see the TODO in queries/process.sql
		Namespace:       int64(namespace),
		Status:          int64(statusnum),
		SourceNetworkID: srcNetworkIdstr,
		IDSubstr:        searchTerm,
		Offset:          int32(from),
		Limit:           int32(max),
		WithResults:     withResults,
	})
	log.Debugf("ProcessList sqlite took %s", time.Since(sqlStartTime))
	if err != nil {
		return nil, err
	}
	return sqlProcs, nil
}

// ProcessCount returns the number of processes indexed
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
	options := p.GetVoteOptions()
	if options == nil {
		return fmt.Errorf("newEmptyProcess: vote options is nil")
	}
	if options.MaxCount == 0 {
		return fmt.Errorf("newEmptyProcess: maxCount is zero")
	}

	// Check for overflows
	if options.MaxCount > MaxQuestions || options.MaxValue > MaxOptions {
		return fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}

	eid := p.GetEntityId()
	// Get the block time from the Header
	currentBlockTime := time.Unix(s.App.TimestampStartBlock(), 0)

	compResultsHeight := uint32(0)
	if live, err := s.isOpenProcess(pid); err != nil {
		return fmt.Errorf("cannot check if process is live: %w", err)
	} else {
		if live {
			compResultsHeight = p.GetBlockCount() + p.GetStartBlock() + 1
		}
	}

	// Create and store process in the indexer database
	proc := &indexertypes.Process{
		ID:                pid,
		EntityID:          eid,
		StartBlock:        p.GetStartBlock(),
		EndBlock:          p.GetBlockCount() + p.GetStartBlock(),
		Rheight:           compResultsHeight,
		HaveResults:       compResultsHeight > 0,
		CensusRoot:        p.GetCensusRoot(),
		RollingCensusRoot: p.GetRollingCensusRoot(),
		CensusURI:         p.GetCensusURI(),
		CensusOrigin:      int32(p.GetCensusOrigin()),
		Status:            int32(p.GetStatus()),
		Namespace:         p.GetNamespace(),
		PrivateKeys:       p.EncryptionPrivateKeys,
		PublicKeys:        p.EncryptionPublicKeys,
		Envelope:          p.GetEnvelopeType(),
		Mode:              p.GetMode(),
		VoteOpts:          p.GetVoteOptions(),
		CreationTime:      currentBlockTime,
		SourceBlockHeight: p.GetSourceBlockHeight(),
		SourceNetworkId:   p.SourceNetworkId.String(),
		Metadata:          p.GetMetadata(),
		// EntityIndex: entity.ProcessCount, // TODO(sqlite): replace with a COUNT
		MaxCensusSize:     p.GetMaxCensusSize(),
		RollingCensusSize: p.GetRollingCensusSize(),
	}
	log.Debugf("new indexer process %s", proc.String())

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	if _, err := queries.CreateProcess(ctx, indexerdb.CreateProcessParams{
		ID:       pid,
		EntityID: nonNullBytes(eid),
		// EntityIndex: int64(entity.ProcessCount), // TODO(sqlite): replace with a COUNT
		StartBlock:        int64(p.GetStartBlock()),
		EndBlock:          int64(p.GetBlockCount() + p.GetStartBlock()),
		ResultsHeight:     int64(compResultsHeight),
		HaveResults:       compResultsHeight > 0,
		CensusRoot:        nonNullBytes(p.GetCensusRoot()),
		RollingCensusRoot: nonNullBytes(p.GetRollingCensusRoot()),
		RollingCensusSize: int64(p.GetRollingCensusSize()),
		MaxCensusSize:     int64(p.GetMaxCensusSize()),
		CensusUri:         p.GetCensusURI(),
		CensusOrigin:      int64(p.GetCensusOrigin()),
		Status:            int64(p.GetStatus()),
		Namespace:         int64(p.GetNamespace()),
		EnvelopePb:        encodedPb(p.GetEnvelopeType()),
		ModePb:            encodedPb(p.GetMode()),
		VoteOptsPb:        encodedPb(p.GetVoteOptions()),
		PrivateKeys:       strings.Join(p.EncryptionPrivateKeys, ","),
		PublicKeys:        strings.Join(p.EncryptionPublicKeys, ","),
		CreationTime:      currentBlockTime,
		SourceBlockHeight: int64(p.GetSourceBlockHeight()),
		SourceNetworkID:   p.SourceNetworkId.String(), // TODO: store the integer?
		Metadata:          p.GetMetadata(),

		ResultsVotes: encodeVotes(indexertypes.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1)),
	}); err != nil {
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
		EndBlock:          int64(p.GetBlockCount() + p.GetStartBlock()),
		CensusRoot:        nonNullBytes(p.GetCensusRoot()),
		RollingCensusRoot: nonNullBytes(p.GetRollingCensusRoot()),
		RollingCensusSize: int64(p.GetRollingCensusSize()),
		CensusUri:         p.GetCensusURI(),
		PrivateKeys:       strings.Join(p.EncryptionPrivateKeys, ","),
		PublicKeys:        strings.Join(p.EncryptionPublicKeys, ","),
		Metadata:          p.GetMetadata(),
		Status:            int64(p.GetStatus()),
	}); err != nil {
		return err
	}
	if models.ProcessStatus(previousStatus) != models.ProcessStatus_CANCELED &&
		p.GetStatus() == models.ProcessStatus_CANCELED {

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
