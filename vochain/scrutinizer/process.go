package scrutinizer

import (
	"encoding/hex"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	scrutinizerdb "go.vocdoni.io/dvote/vochain/scrutinizer/db"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
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

func sqliteWarnf(format string, args ...interface{}) {
	if flag.Lookup("test.run") != nil {
		log.Fatalf(format, args...)
	}
	log.Warnf(format, args...)
}

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(pid []byte) (*indexertypes.Process, error) {
	bhStartTime := time.Now()
	proc := &indexertypes.Process{}
	if err := s.db.FindOne(proc, badgerhold.Where(badgerhold.Key).Eq(pid)); err != nil {
		return nil, err
	}
	log.Debugf("ProcessInfo badgerhold took %s", time.Since(bhStartTime))

	sqlStartTime := time.Now()

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	sqlProcInner, err := queries.GetProcess(ctx, pid)
	if err != nil {
		return nil, err
	}
	log.Debugf("ProcessInfo sqlite took %s", time.Since(sqlStartTime))
	sqlProc := indexertypes.ProcessFromDB(&sqlProcInner)
	if diff := cmp.Diff(proc, sqlProc, cmpopts.IgnoreUnexported(
		models.EnvelopeType{},
		models.ProcessMode{},
		models.ProcessVoteOptions{},
	)); diff != "" {
		sqliteWarnf("ping mvdan to fix the bug with the information below:\nparams: %x\ndiff (-badger +sql):\n%s", pid, diff)
	}

	return proc, nil
}

// ProcessList returns a list of process identifiers (PIDs) registered in the Vochain.
// EntityID, searchTerm, namespace, status, and withResults are optional filters, if
// declared as zero-values will be ignored. SearchTerm is a partial or full PID.
// Status is one of READY, CANCELED, ENDED, PAUSED, RESULTS
func (s *Scrutinizer) ProcessList(entityID []byte,
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
	statusMatchFunc := func(r *badgerhold.RecordAccess) (bool, error) {
		if statusnum == 0 {
			return true, nil
		}
		if r.Field().(int32) == statusnum {
			return true, nil
		}
		return false, nil
	}
	// Filter match function for source network Id
	if srcNetworkIdstr != "" {
		if _, ok := models.SourceNetworkId_value[srcNetworkIdstr]; !ok {
			return nil, fmt.Errorf("sourceNetworkId is unknown %s", srcNetworkIdstr)
		}
	}
	netIdMatchFunc := func(r *badgerhold.RecordAccess) (bool, error) {
		if srcNetworkIdstr == "" {
			return true, nil
		}
		if r.Field().(string) == srcNetworkIdstr {
			return true, nil
		}
		return false, nil
	}
	// For filtering on withResults we use also a match function
	wResultsMatchFunc := func(r *badgerhold.RecordAccess) (bool, error) {
		if !withResults {
			return true, nil
		}
		return r.Field().(bool), nil
	}
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()

	bhStartTime := time.Now()

	// For EntityID and Namespace we use different queries, since they are indexes and the
	// performance improvement is quite relevant.
	var err error
	var procs [][]byte
	switch {
	case namespace == 0 && len(entityID) > 0:
		err = s.db.ForEach(
			badgerhold.Where("EntityID").Eq(entityID).
				Index("EntityID").
				And("Status").MatchFunc(statusMatchFunc).
				And("SourceNetworkId").MatchFunc(netIdMatchFunc).
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime", "ID").
				Skip(from).
				Limit(max),
			func(p *indexertypes.Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	case namespace > 0 && len(entityID) == 0:
		err = s.db.ForEach(
			badgerhold.Where("Namespace").Eq(namespace).
				Index("Namespace").
				And("Status").MatchFunc(statusMatchFunc).
				And("SourceNetworkId").MatchFunc(netIdMatchFunc).
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime", "ID").
				Skip(from).
				Limit(max),
			func(p *indexertypes.Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	case namespace == 0 && len(entityID) == 0:
		err = s.db.ForEach(
			badgerhold.Where("Status").MatchFunc(statusMatchFunc).
				Index("Status").
				And("SourceNetworkId").MatchFunc(netIdMatchFunc).
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime", "ID").
				Skip(from).
				Limit(max),
			func(p *indexertypes.Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	default:
		err = s.db.ForEach(
			badgerhold.Where("EntityID").Eq(entityID).
				Index("EntityID").
				And("Namespace").Eq(namespace).
				And("SourceNetworkId").MatchFunc(netIdMatchFunc).
				And("Status").MatchFunc(statusMatchFunc).
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime", "ID").
				Skip(from).
				Limit(max),
			func(p *indexertypes.Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	}
	log.Debugf("ProcessList badgerhold took %s", time.Since(bhStartTime))
	if err != nil {
		return nil, err
	}
	sqlStartTime := time.Now()
	sqlProcs, err := queries.SearchProcesses(ctx, scrutinizerdb.SearchProcessesParams{
		EntityID:        string(entityID),
		Namespace:       int64(namespace),
		Status:          int64(statusnum),
		SourceNetworkID: srcNetworkIdstr,
		IDSubstr:        searchTerm,
		Offset:          int32(from),
		Limit:           int32(max),
	})
	log.Debugf("ProcessList sqlite took %s", time.Since(sqlStartTime))
	if err != nil {
		return nil, err
	}
	// []string in hex form is easier to debug when we get mismatches
	if diff := cmp.Diff(procs, sqlProcs); diff != "" {
		params := []interface{}{
			entityID, from, max,
			searchTerm, namespace,
			srcNetworkIdstr, status, withResults,
		}
		sqliteWarnf("ping mvdan to fix the bug with the information below:\nparams: %#v\ndiff (-badger +sql):\n%s", params, diff)
	}

	return procs, nil
}

// ProcessCount returns the number of processes indexed
func (s *Scrutinizer) ProcessCount(entityID []byte) uint64 {
	startTime := time.Now()
	defer func() { log.Debugf("ProcessCount took %s", time.Since(startTime)) }()
	var c uint32
	var err error
	if len(entityID) == 0 {
		// If no entity ID, return the stored count of all processes
		processCountStore := &indexertypes.CountStore{}
		if err = s.db.Get(indexertypes.CountStoreProcesses, processCountStore); err != nil {
			log.Errorf("could not get the process count: %v", err)
			return 0
		}
		return processCountStore.Count
	}
	if c, err = s.EntityProcessCount(entityID); err != nil {
		log.Errorf("processCount: cannot fetch entity process count: %v", err)
		return 0
	}
	return uint64(c)
}

// EntityList returns the list of entities indexed by the scrutinizer
// searchTerm is optional, if declared as zero-value
// will be ignored. Searches against the ID field.
func (s *Scrutinizer) EntityList(max, from int, searchTerm string) []string {
	entities := []string{}
	if err := s.db.ForEach(
		badgerhold.Where("ID").MatchFunc(searchMatchFunc(searchTerm)).
			SortBy("CreationTime", "ID").
			Skip(from).
			Limit(max),
		func(e *indexertypes.Entity) error {
			entities = append(entities, fmt.Sprintf("%x", e.ID))
			return nil
		}); err != nil {
		log.Warnf("error listing entities: %v", err)
	}
	return entities
}

// EntityProcessCount returns the number of processes that an entity holds
func (s *Scrutinizer) EntityProcessCount(entityId []byte) (uint32, error) {
	entity := &indexertypes.Entity{}
	if err := s.db.FindOne(entity, badgerhold.Where(badgerhold.Key).Eq(entityId)); err != nil {
		return 0, err
	}
	return entity.ProcessCount, nil
}

// EntityCount return the number of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityCount() uint64 {
	entityCountStore := &indexertypes.CountStore{}
	if err := s.db.Get(indexertypes.CountStoreEntities, entityCountStore); err != nil {
		log.Errorf("could not get the entity count: %v", err)
		return 0
	}
	return entityCountStore.Count
}

// Return whether a process must have live results or not
func (s *Scrutinizer) isOpenProcess(processID []byte) (bool, error) {
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
func (s *Scrutinizer) computePendingProcesses(height uint32) {
	if err := s.db.ForEach(badgerhold.Where("Rheight").Eq(height).Index("Rheight"),
		func(p *indexertypes.Process) error {
			initT := time.Now()
			if err := s.ComputeResult(p.ID); err != nil {
				log.Warnf("cannot compute results for %x: (%v)", p.ID, err)
				return nil
			}
			log.Infof("results computation on %x took %s", p.ID, time.Since(initT).String())
			return nil
		}); err != nil {
		log.Warn(err)
	}
}

// newEmptyProcess creates a new empty process and stores it into the database.
// The process must exist on the Vochain state, else an error is returned.
func (s *Scrutinizer) newEmptyProcess(pid []byte) error {
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

	// Create results in the indexer database
	s.addVoteLock.Lock()
	if err := s.queryWithRetries(func() error {
		return s.db.Insert(pid, &indexertypes.Results{
			ProcessID: pid,
			// MaxValue requires +1 since 0 is also an option
			Votes:        indexertypes.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
			Weight:       new(types.BigInt).SetUint64(0),
			Signatures:   []types.HexBytes{},
			VoteOpts:     p.GetVoteOptions(),
			EnvelopeType: p.GetEnvelopeType(),
		})
	}); err != nil {
		s.addVoteLock.Unlock()
		return err
	}
	s.addVoteLock.Unlock()

	// Increment the total process count storage
	s.db.UpdateMatching(&indexertypes.CountStore{}, badgerhold.Where(badgerhold.Key).
		Eq(indexertypes.CountStoreProcesses), func(record interface{}) error {
		update, ok := record.(*indexertypes.CountStore)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted CountStore, got %T", record)
		}
		update.Count++
		return nil
	},
	)

	// Get the block time from the Header
	currentBlockTime := time.Unix(s.App.TimestampStartBlock(), 0)

	// Add the entity to the indexer database
	eid := p.GetEntityId()
	entity := &indexertypes.Entity{}
	// If entity is not registered in db, add to entity count cache and insert to db
	if err := s.db.FindOne(entity, badgerhold.Where(badgerhold.Key).Eq(eid)); err != nil {
		if err != badgerhold.ErrNotFound {
			return err
		}
		entity.ID = eid
		entity.CreationTime = currentBlockTime
		entity.ProcessCount = 0
		// Increment the total entity count storage
		s.db.UpdateMatching(&indexertypes.CountStore{}, badgerhold.Where(badgerhold.Key).Eq(indexertypes.CountStoreEntities), func(record interface{}) error {
			update, ok := record.(*indexertypes.CountStore)
			if !ok {
				return fmt.Errorf("record isn't the correct type! Wanted CountStore, got %T", record)
			}
			update.Count++
			return nil
		})
	}
	// Increase the entity process count (and create new entity if does not exist)
	entity.ProcessCount++
	if err := s.queryWithRetries(func() error {
		return s.db.Upsert(eid, entity)
	}); err != nil {
		return err
	}

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
		EntityIndex:       entity.ProcessCount,
		MaxCensusSize:     p.GetMaxCensusSize(),
		RollingCensusSize: p.GetRollingCensusSize(),
	}
	log.Debugf("new indexer process %s", proc.String())

	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	if _, err := queries.CreateProcess(ctx, scrutinizerdb.CreateProcessParams{
		ID:                pid,
		EntityID:          string(eid), // NOTE: we use string instead of []byte; see sqlc.yaml
		EntityIndex:       int64(entity.ProcessCount),
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
	}); err != nil {
		return fmt.Errorf("sql create process: %w", err)
	}

	return s.queryWithRetries(func() error { return s.db.Insert(pid, proc) })
}

// updateProcess synchronize those fields that can be updated on a existing process
// with the information obtained from the Vochain state
func (s *Scrutinizer) updateProcess(pid []byte) error {
	p, err := s.App.State.Process(pid, false)
	if err != nil {
		return fmt.Errorf("updateProcess: cannot fetch process %x: %w", pid, err)
	}

	updateFunc := func(record interface{}) error {
		update, ok := record.(*indexertypes.Process)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted Process, got %T", record)
		}
		update.EndBlock = p.GetBlockCount() + p.GetStartBlock()
		update.CensusRoot = p.GetCensusRoot()
		update.RollingCensusRoot = p.GetRollingCensusRoot()
		update.CensusURI = p.GetCensusURI()
		update.PrivateKeys = p.EncryptionPrivateKeys
		update.PublicKeys = p.EncryptionPublicKeys
		update.Metadata = p.GetMetadata()
		update.RollingCensusSize = p.GetRollingCensusSize()
		// If the process is transacting to CANCELED, ensure results are not computed and remove
		// them from the KV database.
		if update.Status != int32(models.ProcessStatus_CANCELED) &&
			p.GetStatus() == models.ProcessStatus_CANCELED {
			update.HaveResults = false
			update.FinalResults = true
			update.Rheight = 0
			if err := s.db.UpdateMatching(&indexertypes.Results{},
				badgerhold.Where(badgerhold.Key).Eq(pid), func(record interface{}) error {
					results, ok := record.(*indexertypes.Results)
					if !ok {
						return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
					}
					// On cancelled process, remove all results except for envelope height, weight, pid
					results.Votes = [][]*types.BigInt{}
					results.EnvelopeType = &models.EnvelopeType{}
					results.VoteOpts = &models.ProcessVoteOptions{}
					results.Signatures = []types.HexBytes{}
					results.Final = false
					results.BlockHeight = 0
					return nil
				}); err != nil {
				if err != badgerhold.ErrNotFound {
					log.Warnf("cannot remove CANCELED results: %v", err)
				}
			}
		}
		update.Status = int32(p.GetStatus())
		return nil
	}
	if err := s.queryWithRetries(func() error {
		return s.db.UpdateMatching(&indexertypes.Process{},
			badgerhold.Where(badgerhold.Key).Eq(pid), updateFunc)
	}); err != nil {
		return err
	}
	// TODO: remove from results table
	// TODO: hold a sql db transaction for multiple queries
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	previousStatus, err := queries.GetProcessStatus(ctx, pid)
	if err != nil {
		return err
	}
	if _, err := queries.UpdateProcessFromState(ctx, scrutinizerdb.UpdateProcessFromStateParams{
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

		if _, err := queries.SetProcessResultsHeight(ctx, scrutinizerdb.SetProcessResultsHeightParams{
			ID:            pid,
			ResultsHeight: 0,
		}); err != nil {
			return err
		}
		if _, err := queries.SetProcessResultsCancelled(ctx, pid); err != nil {
			return err
		}
	}
	return nil
}

// setResultsHeight updates the Rheight of any process whose ID is pid.
func (s *Scrutinizer) setResultsHeight(pid []byte, height uint32) error {
	if height == 0 {
		panic("setting results height to 0?")
	}
	if err := s.queryWithRetries(func() error {
		return s.db.UpdateMatching(&indexertypes.Process{}, badgerhold.Where(badgerhold.Key).Eq(pid),
			func(record interface{}) error {
				update, ok := record.(*indexertypes.Process)
				if !ok {
					return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
				}
				update.Rheight = height
				return nil
			})
	}); err != nil {
		return err
	}
	queries, ctx, cancel := s.timeoutQueries()
	defer cancel()
	if _, err := queries.SetProcessResultsHeight(ctx, scrutinizerdb.SetProcessResultsHeightParams{
		ID:            pid,
		ResultsHeight: int64(height),
	}); err != nil {
		return err
	}
	return nil
}

// searchMatchFunc generates a function which compares a badgerhold record against searchTerm.
// The record should be types.HexBytes, and is encoded as a string to compare to searchTerm.
func searchMatchFunc(searchTerm string) func(r *badgerhold.RecordAccess) (bool, error) {
	return func(r *badgerhold.RecordAccess) (bool, error) {
		if searchTerm == "" {
			return true, nil
		}
		// TODO search without having to convert each field to strings
		if strings.Contains(hex.EncodeToString(r.Field().(types.HexBytes)), searchTerm) {
			return true, nil
		}
		return false, nil
	}
}
