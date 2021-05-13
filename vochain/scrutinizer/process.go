package scrutinizer

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(pid []byte) (*indexertypes.Process, error) {
	proc := &indexertypes.Process{}
	err := s.db.FindOne(proc, badgerhold.Where(badgerhold.Key).Eq(pid))
	return proc, err
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
	status string,
	withResults bool) ([][]byte, error) {
	if from < 0 {
		return nil, fmt.Errorf("processList: invalid value: from is invalid value %d", from)
	}
	// For filtering on Status we use a badgerhold match function.
	// If status is not defined, then the match function will return always true.
	statusnum := int32(-1)
	statusfound := false
	if status != "" {
		if statusnum, statusfound = models.ProcessStatus_value[status]; !statusfound {
			return nil, fmt.Errorf("processList: status %s is unknown", status)
		}
	}
	statusMatchFunc := func(r *badgerhold.RecordAccess) (bool, error) {
		if statusnum == -1 {
			return true, nil
		}
		if r.Field().(int32) == statusnum {
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
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime").
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
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime").
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
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime").
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
				And("Status").MatchFunc(statusMatchFunc).
				And("HaveResults").MatchFunc(wResultsMatchFunc).
				And("ID").MatchFunc(searchMatchFunc(searchTerm)).
				SortBy("CreationTime").
				Skip(from).
				Limit(max),
			func(p *indexertypes.Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	}

	return procs, err
}

// ProcessCount return the number of processes indexed
func (s *Scrutinizer) ProcessCount(entityID []byte) int64 {
	var c int
	var err error
	if len(entityID) == 0 {
		// If no entity ID, return the cached count of all processes
		return atomic.LoadInt64(s.countTotalProcesses)
	}
	if c, err = s.db.Count(&indexertypes.Process{},
		badgerhold.Where("EntityID").Eq(entityID).Index("EntityID")); err != nil {
		log.Warnf("cannot count processes: %v", err)
	}
	return int64(c)
}

// EntityList returns the list of entities indexed by the scrutinizer
// searchTerm is optional, if declared as zero-value
// will be ignored. Searches against the ID field.
func (s *Scrutinizer) EntityList(max, from int, searchTerm string) []string {
	entities := []string{}
	if err := s.db.ForEach(
		badgerhold.Where("ID").MatchFunc(searchMatchFunc(searchTerm)).
			SortBy("CreationTime").
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

// EntityCount return the number of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityCount() int64 {
	return atomic.LoadInt64(s.countTotalEntities)
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
			log.Infof("results compute on %x took %s", p.ID, time.Since(initT).String())
			p.FinalResults = true
			p.HaveResults = true
			if err := s.db.Update(p.ID, p); err != nil {
				log.Warnf("cannot update results for process %x: %v", p.ID, err)
			}
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
	if err := s.db.Insert(pid, &indexertypes.Results{
		ProcessID: pid,
		// MaxValue requires +1 since 0 is also an option
		Votes:        indexertypes.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
		Weight:       new(big.Int).SetUint64(0),
		Signatures:   []types.HexBytes{},
		VoteOpts:     p.GetVoteOptions(),
		EnvelopeType: p.GetEnvelopeType(),
	}); err != nil {
		s.addVoteLock.Unlock()
		return err
	}
	s.addVoteLock.Unlock()

	// Increment the total process count cache
	atomic.AddInt64(s.countTotalProcesses, 1)

	// Get the block time from the Header
	currentBlockTime := time.Unix(s.App.State.Header(false).Timestamp, 0)

	// Add the entity to the indexer database
	eid := p.GetEntityId()
	entity := &indexertypes.Entity{}
	// If entity is not registered in db, add to entity count cache and insert to db
	if err := s.db.FindOne(entity, badgerhold.Where("ID").Eq(eid)); err != nil {
		if err != badgerhold.ErrNotFound {
			return err
		}
		atomic.AddInt64(s.countTotalEntities, 1)
		if err := s.db.Insert(eid, &indexertypes.Entity{
			ID:           eid,
			CreationTime: currentBlockTime,
		}); err != nil {
			return err
		}
	}

	compResultsHeight := uint32(0)
	if live, err := s.isOpenProcess(pid); err != nil {
		return fmt.Errorf("cannot check if process live: %w", err)
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
	}
	log.Debugf("new indexer process %s", proc.String())
	return s.db.Insert(pid, proc)
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
			return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
		}
		update.EndBlock = p.GetBlockCount() + p.GetStartBlock()
		update.CensusRoot = p.GetCensusRoot()
		update.CensusURI = p.GetCensusURI()
		update.PrivateKeys = p.EncryptionPrivateKeys
		update.PublicKeys = p.EncryptionPublicKeys
		// If the process is transacting to CANCELED, ensure results are not computed and remove
		// them from the KV database.
		if update.Status != int32(models.ProcessStatus_CANCELED) &&
			p.GetStatus() == models.ProcessStatus_CANCELED {
			update.HaveResults = false
			update.FinalResults = true
			update.Rheight = 0
			if err := s.db.Delete(pid, &indexertypes.Results{}); err != nil {
				if err != badgerhold.ErrNotFound {
					log.Warnf("cannot remove CANCELED results: %v", err)
				}
			}
		}
		update.Status = int32(p.GetStatus())
		return nil
	}
	// Retry if error is "Transaction Conflict"
	for {
		if err := s.db.UpdateMatching(&indexertypes.Process{},
			badgerhold.Where(badgerhold.Key).Eq(pid), updateFunc); err != nil {
			if strings.Contains(err.Error(), kvErrorStringForRetry) {
				continue
			}
			return err
		}
		break
	}
	return nil
}

func (s *Scrutinizer) setResultsHeight(pid []byte, height uint32) error {
	return s.db.UpdateMatching(&indexertypes.Process{}, badgerhold.Where(badgerhold.Key).Eq(pid),
		func(record interface{}) error {
			update, ok := record.(*indexertypes.Process)
			if !ok {
				return fmt.Errorf("record isn't the correct type! Wanted Result, got %T", record)
			}
			update.Rheight = height
			return nil
		})
}

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
