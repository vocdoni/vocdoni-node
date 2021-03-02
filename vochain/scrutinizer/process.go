package scrutinizer

import (
	"fmt"
	"math/big"
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/proto/build/go/models"

	"go.vocdoni.io/dvote/log"
)

// ProcessInfo returns the available information regarding an election process id
func (s *Scrutinizer) ProcessInfo(pid []byte) (*Process, error) {
	proc := &Process{}
	err := s.db.FindOne(proc, badgerhold.Where("ID").Eq(pid))
	return proc, err
}

// ProcessList returns the list of processes (finished or not) for a specific entity.
func (s *Scrutinizer) ProcessList(entityID []byte, namespace uint32,
	from, max int) ([][]byte, error) {
	procs := [][]byte{}
	var err error

	switch {
	case namespace == 0 && len(entityID) > 0:
		err = s.db.ForEach(
			badgerhold.Where("EntityID").
				Eq(entityID).
				Index("EntityID").
				SortBy("CreationTime").
				Skip(from).
				Limit(max),
			func(p *Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	case namespace > 0 && len(entityID) == 0:
		err = s.db.ForEach(
			badgerhold.Where("Namespace").
				Eq(namespace).
				Index("Namespace").
				SortBy("CreationTime").
				Skip(from).
				Limit(max),
			func(p *Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	default:
		err = s.db.ForEach(
			badgerhold.Where("EntityID").
				Eq(entityID).
				And("Namespace").
				Eq(namespace).
				Index("EntityID").
				SortBy("CreationTime").
				Skip(from).
				Limit(max),
			func(p *Process) error {
				procs = append(procs, p.ID)
				return nil
			})
	}

	return procs, err
}

// ProcessListWithResults returns the list of process ID with already computed results.
// TODO: use [][]byte instead of []string as return value
func (s *Scrutinizer) ProcessListWithResults(max, from int) []string {
	procs := []string{}
	if err := s.db.ForEach(
		badgerhold.Where("HaveResults").
			Eq(true).
			SortBy("CreationTime").
			Skip(from).
			Limit(max),
		func(p *Process) error {
			procs = append(procs, fmt.Sprintf("%x", p.ID))
			return nil
		}); err != nil {
		log.Warnf("processListWithResults error while iterating: %v", err)
	}
	return procs
}

// ProcessListWithLiveResults returns the list of process ID which have live results (not encrypted)
func (s *Scrutinizer) ProcessListWithLiveResults(max, from int) []string {
	procs := []string{}
	if err := s.db.ForEach(
		badgerhold.Where("HaveResults").
			Eq(true).And("FinalResults").
			Eq(false).SortBy("CreationTime").
			Skip(from).
			Limit(max),
		func(p *Process) error {
			procs = append(procs, fmt.Sprintf("%x", p.ID))
			return nil
		}); err != nil {
		log.Warnf("processListWithLiveResults error while iterating: %v", err)
	}
	return procs
}

// EntityList returns the list of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityList(max, from int) []string {
	entities := []string{}
	if err := s.db.ForEach(
		badgerhold.Where("ID").
			Ne(&[]byte{}).
			SortBy("CreationTime").
			Skip(from).
			Limit(max),
		func(e *Entity) error {
			entities = append(entities, fmt.Sprintf("%x", e.ID))
			return nil
		}); err != nil {
		log.Warnf("error listing entities: %v", err)
	}
	return entities
}

// EntityCount return the number of entities indexed by the scrutinizer
func (s *Scrutinizer) EntityCount() int64 {
	c, err := s.db.Count(&Entity{}, nil)
	if err != nil {
		log.Warnf("cannot count entities: %v", err)
	}
	return int64(c)
}

// Return whether a process must have live results or not
func (s *Scrutinizer) isLiveResultsProcess(processID []byte) (bool, error) {
	p, err := s.VochainState.Process(processID, false)
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
	if err := s.db.ForEach(badgerhold.Where("ResultsHeight").Eq(height).Index("ResultsHeight"),
		func(p *Process) error {
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
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		return fmt.Errorf("cannot create new empty process: %w", err)
	}
	options := p.GetVoteOptions()
	if options == nil {
		return fmt.Errorf("newEmptyProcess: vote options is nil")
	}
	if options.MaxCount == 0 || options.MaxValue == 0 {
		return fmt.Errorf("newEmptyProcess: maxCount or maxValue are zero")
	}

	// Check for overflows
	if options.MaxCount > MaxQuestions || options.MaxValue > MaxOptions {
		return fmt.Errorf("maxCount or maxValue overflows hardcoded maximums")
	}

	// MaxValue requires +1 since 0 is also an option
	pv := newEmptyResults(int(options.MaxCount), int(options.MaxValue)+1)

	// Create results in the indexer database
	if err := s.db.Insert(pid, &Results{
		ProcessID:  pid,
		Votes:      pv.GetVotes(),
		Signatures: [][]byte{},
	}); err != nil {
		return err
	}

	// Get the block time from the Header
	currentBlockTime := time.Unix(0, s.VochainState.Header(false).Timestamp)

	// Add the entity to the indexer database
	eid := p.GetEntityId()
	if err := s.db.Upsert(eid, &Entity{ID: eid, CreationTime: currentBlockTime}); err != nil {
		return err
	}

	compResultsHeight := uint32(0)
	if live, err := s.isLiveResultsProcess(pid); err != nil {
		return fmt.Errorf("cannot check if process live: %w", err)
	} else {
		if live {
			compResultsHeight = p.GetBlockCount() + p.GetStartBlock()
		}
	}

	// Create process in the indexer database
	return s.db.Insert(pid, &Process{
		ID:            pid,
		EntityID:      eid,
		StartBlock:    p.GetStartBlock(),
		EndBlock:      p.GetBlockCount() + p.GetStartBlock(),
		ResultsHeight: compResultsHeight,
		HaveResults:   compResultsHeight > 0,
		CensusRoot:    p.GetCensusRoot(),
		CensusURI:     p.GetCensusURI(),
		CensusOrigin:  int32(p.GetCensusOrigin()),
		Status:        int32(p.GetStatus()),
		Namespace:     p.GetNamespace(),
		PrivateKeys:   p.EncryptionPrivateKeys,
		PublicKeys:    p.EncryptionPublicKeys,
		Envelope:      p.GetEnvelopeType(),
		Mode:          p.GetMode(),
		VoteOpts:      p.GetVoteOptions(),
		CreationTime:  currentBlockTime,
	})
}

// updateProcess synchronize those fields that can be updated on a existing process
// with the information obtained from the Vochain state
func (s *Scrutinizer) updateProcess(pid []byte) error {
	p, err := s.VochainState.Process(pid, false)
	if err != nil {
		return fmt.Errorf("updateProcess: cannot fetch process %x: %w", pid, err)
	}
	dbProc := &Process{}
	if err := s.db.FindOne(dbProc, badgerhold.Where("ID").Eq(pid)); err != nil {
		return err
	}
	dbProc.EndBlock = p.GetBlockCount() + p.GetStartBlock()
	dbProc.CensusRoot = p.GetCensusRoot()
	dbProc.CensusURI = p.GetCensusURI()
	dbProc.Status = int32(p.GetStatus())
	dbProc.PrivateKeys = p.EncryptionPrivateKeys
	dbProc.PublicKeys = p.EncryptionPublicKeys
	return s.db.Update(pid, dbProc)
}

func (s *Scrutinizer) setResultsHeight(pid []byte, height uint32) error {
	dbProc := &Process{}
	if err := s.db.FindOne(dbProc, badgerhold.Where("ID").Eq(pid)); err != nil {
		return err
	}
	dbProc.ResultsHeight = height
	return s.db.Update(pid, dbProc)
}

func newEmptyResults(questions, options int) *models.ProcessResult {
	if questions == 0 || options == 0 {
		return nil
	}
	pv := new(models.ProcessResult)
	pv.Votes = make([]*models.QuestionResult, questions)
	zero := big.NewInt(0)
	for i := range pv.Votes {
		pv.Votes[i] = new(models.QuestionResult)
		pv.Votes[i].Question = make([][]byte, options)
		for j := range pv.Votes[i].Question {
			pv.Votes[i].Question[j] = zero.Bytes()
		}
	}
	return pv
}
