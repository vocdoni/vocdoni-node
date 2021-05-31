package scrutinizer

import (
	"bytes"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/timshannon/badgerhold/v3"
	"go.vocdoni.io/dvote/db/lru"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
)

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
	// MaxOptions is the maximum number of options allowed in a VotePackage question
	MaxOptions = 128
	// MaxEnvelopeListSize is the maximum number of envelopes a process can store.
	// 8.3M seems enough for now
	MaxEnvelopeListSize = 32 << 18

	countEnvelopeCacheSize = 1024
	resultsCacheSize       = 512
)

// EventListener is an interface used for executing custom functions during the
// events of the tally of a process.
type EventListener interface {
	OnComputeResults(results *indexertypes.Results)
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (s *Scrutinizer) AddEventListener(l EventListener) {
	s.eventListeners = append(s.eventListeners, l)
}

// Scrutinizer is the component which makes the accounting of the voting processes
// and keeps it indexed in a local database.
type Scrutinizer struct {
	App *vochain.BaseApplication
	// voteIndexPool is the list of votes that will be indexed in the database
	voteIndexPool []*VoteWithIndex
	// votePool is the list of votes that should be live counted, grouped by processId
	votePool map[string][]*models.Vote
	// newProcessPool is the list of new process IDs on the current block
	newProcessPool []*indexertypes.ScrutinizerOnProcessData
	// updateProcessPool is the list of process IDs that require sync with the state database
	updateProcessPool [][]byte
	// resultsPool is the list of processes that finish on the current block
	resultsPool []*indexertypes.ScrutinizerOnProcessData
	// list of live processes (those on which the votes will be computed on arrival)
	liveResultsProcs sync.Map
	// eventListeners is the list of external callbacks that will be executed by the scrutinizer
	eventListeners []EventListener
	db             *badgerhold.Store
	// envelopeHeightCache and countTotalEnvelopes are in memory counters that helps reducing the
	// access time when GenEnvelopeHeight() is called.
	envelopeHeightCache    *lru.Cache
	countTotalEnvelopes    uint64
	countTotalEntities     uint64
	countTotalProcesses    uint64
	countTotalTransactions uint64
	// resultsCache is a memory cache for final results results, stores processId:<results>
	resultsCache *lru.Cache
	// addVoteLock is used to avoid Transaction Conflicts on the KV database.
	// It is not critical and the code should be able to recover from a Conflict, but we
	// try to minimize this situations in order to improve performance on the KV.
	// TODO (pau): remove this mutex and relay on the KV layer
	addVoteLock sync.RWMutex
	// indexTxLock is used to avoid Transaction Conflicts on the vote index KV database.
	indexTxLock sync.RWMutex
	// recoveryBootLock prevents Commit() to add new votes while the recovery bootstratp is
	// being executed.
	recoveryBootLock sync.RWMutex
}

// VoteWithIndex holds a Vote and a txIndex. Model for the VotePool.
type VoteWithIndex struct {
	vote    *models.Vote
	txIndex int32
}

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, app *vochain.BaseApplication) (*Scrutinizer, error) {
	s := &Scrutinizer{App: app}
	var err error
	s.db, err = InitDB(dbPath)
	if err != nil {
		return nil, err
	}
	startTime := time.Now()

	txCountStore := new(indexertypes.CountStore)
	err = s.db.FindOne(txCountStore,
		badgerhold.Where(badgerhold.Key).Eq(indexertypes.Transactions))
	if err != nil {
		log.Warnf("could not get the transaction count: %v", err)
	}
	envelopeCountStore := new(indexertypes.CountStore)
	err = s.db.FindOne(envelopeCountStore,
		badgerhold.Where(badgerhold.Key).Eq(indexertypes.Envelopes))
	if err != nil {
		log.Warnf("could not get the envelope count: %v", err)
	}
	processCountStore := new(indexertypes.CountStore)
	err = s.db.FindOne(processCountStore,
		badgerhold.Where(badgerhold.Key).Eq(indexertypes.Processes))
	if err != nil {
		log.Warnf("could not get the process count: %v", err)
	}
	entityCountStore := new(indexertypes.CountStore)
	err = s.db.FindOne(entityCountStore,
		badgerhold.Where(badgerhold.Key).Eq(indexertypes.Entities))
	if err != nil {
		log.Warnf("could not get the entity count: %v", err)
	}

	s.countTotalTransactions = txCountStore.Count
	s.countTotalEnvelopes = envelopeCountStore.Count
	s.countTotalProcesses = processCountStore.Count
	s.countTotalEntities = entityCountStore.Count

	log.Infof("indexer initialization took %s, stored %d "+
		"transactions, %d envelopes, %d processes and %d entities",
		time.Since(startTime),
		s.countTotalTransactions,
		s.countTotalEnvelopes,
		s.countTotalProcesses,
		s.countTotalEntities)
	// Subscrive to events
	s.App.State.AddEventListener(s)
	s.envelopeHeightCache = lru.New(countEnvelopeCacheSize)
	s.resultsCache = lru.New(resultsCacheSize)
	return s, nil
}

// AfterSyncBootstrap is a blocking function that waits until the Vochain is synchronized
// and then execute a set of recovery actions. It mainly checks for those processes which are
// still open (live) and updates all temporary data (current voting weight and live results
// if unecrypted). This method might be called on a goroutine after initializing the Scrutinizer.
// TO-DO: refactor and use blockHeight for reusing existing live results
func (s *Scrutinizer) AfterSyncBootstrap() {
	// During the first seconds/milliseconds of the Vochain startup, Tendermint might report that
	// the chain is not synchronizing since it still does not have any peer and do not know the
	// actual size of the blockchain. If afterSyncBootStrap is executed on this specific moment,
	// the Wait loop would pass.
	syncSignals := 5
	for {
		// Add some grace time to avoid false positive on IsSynchronizing()
		if !s.App.IsSynchronizing() {
			syncSignals--
		} else {
			syncSignals = 5
		}
		if syncSignals == 0 {
			break
		}
		time.Sleep(time.Second * 1)
	}
	log.Infof("running scrutinizer after-sync bootstrap")
	// Block the new votes addition until the recovery finishes.
	s.recoveryBootLock.Lock()
	defer s.recoveryBootLock.Unlock()
	// Find those processes which do not have yet final results,
	// they are considered live so we need to compute the temporary
	// results (or only its weight in case of Encrypted)
	prcs := [][]byte{}
	err := s.db.ForEach(
		badgerhold.Where("FinalResults").Eq(false),
		func(p *indexertypes.Process) error {
			prcs = append(prcs, p.ID)
			return nil
		})
	if err != nil {
		log.Error(err)
	}
	log.Infof("recovered %d live results processes", len(prcs))
	log.Infof("starting live results recovery computation")
	startTime := time.Now()
	for _, p := range prcs {
		// In order to recover the full list of live results, we need
		// to reset the existing Results and count them again from scratch.
		// Since we cannot be sure if there are votes missing, we need to
		// perform the full computation.
		log.Infof("recovering live process %x", p)
		process, err := s.App.State.Process(p, false)
		if err != nil {
			log.Errorf("cannot fetch process: %v", err)
			continue
		}
		options := process.GetVoteOptions()
		if err := s.db.Upsert(p, &indexertypes.Results{
			ProcessID: p,
			// MaxValue requires +1 since 0 is also an option
			Votes:        indexertypes.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
			Weight:       new(big.Int).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.GetEnvelopeType(),
			Signatures:   []types.HexBytes{},
		}); err != nil {
			log.Errorf("cannot upsert results to db: %v", err)
			continue
		}

		// Count the votes, add them to results (in memory, without any db transaction)
		results := &indexertypes.Results{
			Weight:       new(big.Int).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.EnvelopeType,
		}
		if err := s.WalkEnvelopes(p, false, func(vote *models.VoteEnvelope, weight *big.Int) {
			if err := s.addLiveVote(vote.ProcessId, vote.VotePackage,
				weight, results); err != nil {
				log.Warn(err)
			}
		}); err != nil {
			log.Error(err)
			continue
		}
		// Store the results on the persisten database
		if err := s.commitVotesUnsafe(p, results, s.App.Height()); err != nil {
			log.Errorf("cannot commit live votes: (%v)", err)
			continue
		}
		// Add process to live results so new votes will be added
		s.addProcessToLiveResults(p)
	}
	log.Infof("live resuts recovery computation finished, took %s", time.Since(startTime))
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (s *Scrutinizer) Commit(height uint32) error {
	// Add Entity and register new active process
	for _, p := range s.newProcessPool {
		if err := s.newEmptyProcess(p.ProcessID); err != nil {
			log.Errorf("commit: cannot create new empty process: %v", err)
			continue
		}
		if !s.App.IsSynchronizing() {
			s.addProcessToLiveResults(p.ProcessID)
		}
	}

	// Update existing processes
	for _, p := range s.updateProcessPool {
		if err := s.updateProcess(p); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p, err)
		}
	}

	// Schedule results computation
	for _, p := range s.resultsPool {
		if err := s.setResultsHeight(p.ProcessID, uint32(height+1)); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p.ProcessID, err)
			continue
		}
		s.delProcessFromLiveResults(p.ProcessID)
		log.Infof("scheduled results computation on next block for %x", p.ProcessID)
	}

	startTime := time.Now()
	txn := s.db.Badger().NewTransaction(true)
	for _, v := range s.voteIndexPool {
		if err := s.addVoteIndex(
			v.vote.Nullifier,
			v.vote.ProcessId,
			height,
			v.vote.Weight,
			v.txIndex, txn); err != nil {
			log.Warn(err)
		}
	}
	if len(s.voteIndexPool) > 0 {
		s.indexTxLock.Lock()
		wg := sync.WaitGroup{}
		wg.Add(1)
		txn.CommitWith(func(err error) {
			if err != nil {
				log.Error(err)
			}
			wg.Done()
		})
		wg.Wait()
		s.indexTxLock.Unlock()
		log.Infof("indexed %d new envelopes, took %s",
			len(s.voteIndexPool), time.Since(startTime))
		// Add the envelopes to the countTotalEnvelopes var
		atomic.AddUint64(&s.countTotalEnvelopes, uint64(len(s.voteIndexPool)))
	}
	txn.Discard()

	// Add votes collected by onVote (live results), can be run async
	nvotes := 0
	startTime = time.Now()

	for pid, votes := range s.votePool {
		// Get the process information
		proc, err := s.ProcessInfo([]byte(pid))
		if err != nil {
			log.Warnf("cannot get process %x", []byte(pid))
			continue
		}
		// This is a temporary "results" for computing votes
		// of a single processId for the current block.
		results := &indexertypes.Results{
			Weight:       new(big.Int).SetUint64(0),
			VoteOpts:     proc.VoteOpts,
			EnvelopeType: proc.Envelope,
		}
		for _, v := range votes {
			if err := s.addLiveVote(v.ProcessId,
				v.VotePackage,
				// TBD: Not 100% sure what happens if weight=nil
				new(big.Int).SetBytes(v.GetWeight()),
				results); err != nil {
				log.Warnf("vote cannot be added: %v", err)
			} else {
				nvotes++
			}
		}
		// Commit votes (store to disk)
		if err := s.commitVotes([]byte(pid), results, s.App.Height()); err != nil {
			log.Errorf("cannot commit live votes from block %d: (%v)", err, height)
		}
	}
	if nvotes > 0 {
		log.Infof("added %d live votes on block %d, took %s",
			nvotes, height, time.Since(startTime))
	}
	s.storeCountCaches()

	// Check if there are processes that need results computing
	// this can be run async
	go s.computePendingProcesses(height)
	return nil
}

// Rollback removes the non committed pending operations
func (s *Scrutinizer) Rollback() {
	s.votePool = make(map[string][]*models.Vote)
	s.voteIndexPool = []*VoteWithIndex{}
	s.newProcessPool = []*indexertypes.ScrutinizerOnProcessData{}
	s.resultsPool = []*indexertypes.ScrutinizerOnProcessData{}
	s.updateProcessPool = [][]byte{}
}

// OnProcess scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32) {
	data := &indexertypes.ScrutinizerOnProcessData{EntityID: eid, ProcessID: pid}
	s.newProcessPool = append(s.newProcessPool, data)
}

// OnVote scrutinizer stores the votes if the processId is live results (on going)
// and the blockchain is not synchronizing.
func (s *Scrutinizer) OnVote(v *models.Vote, txIndex int32) {
	if s.isProcessLiveResults(v.ProcessId) {
		s.votePool[string(v.ProcessId)] = append(s.votePool[string(v.ProcessId)], v)
	}
	s.voteIndexPool = append(s.voteIndexPool, &VoteWithIndex{vote: v, txIndex: txIndex})
}

// OnCancel scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnCancel(pid []byte, txIndex int32) {
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// OnProcessKeys does nothing
func (s *Scrutinizer) OnProcessKeys(pid []byte, pub, commit string, txIndex int32) {
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// OnProcessStatusChange adds the process to the updateProcessPool and, if ended, the resultsPool
func (s *Scrutinizer) OnProcessStatusChange(pid []byte, status models.ProcessStatus,
	txIndex int32) {
	if status == models.ProcessStatus_ENDED {
		if live, err := s.isOpenProcess(pid); err != nil {
			log.Warn(err)
		} else if live {
			s.resultsPool = append(s.resultsPool, &indexertypes.ScrutinizerOnProcessData{ProcessID: pid})
		}
	}
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// OnRevealKeys checks if all keys have been revealed and in such case add the
// process to the results queue
func (s *Scrutinizer) OnRevealKeys(pid []byte, priv, reveal string, txIndex int32) {
	p, err := s.App.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot fetch process %s from state: (%s)", pid, err)
		return
	}
	if p.KeyIndex == nil {
		log.Errorf("keyindex is nil")
		return
	}
	// if all keys have been revealed, compute the results
	if *p.KeyIndex < 1 {
		data := indexertypes.ScrutinizerOnProcessData{EntityID: p.EntityId, ProcessID: pid}
		s.resultsPool = append(s.resultsPool, &data)
	}
	s.updateProcessPool = append(s.updateProcessPool, pid)
}

// OnProcessResults verifies the results for  a process and appends it to the updateProcessPool
func (s *Scrutinizer) OnProcessResults(pid []byte, results []*models.QuestionResult,
	txIndex int32) error {
	// TODO: check results are valid and return an error if not.
	// This is very dangerous since an Oracle would be able to create a consensus failure,
	// the validaros (that do not check the results) and the full-nodes (with the scrutinizer enabled)
	// would compute different state hash.
	// As a temporary solution, lets compare results but just print the error.

	// This code must be run async in order to not delay the consensus. The results retreival
	// could require some time.
	go func() {
		var myResults *indexertypes.Results
		var err error
		retries := 20
		for {
			if retries == 0 {
				log.Errorf("could not fetch results after max retries")
				return
			}
			myResults, err = s.GetResults(pid)
			if err == nil {
				break
			}
			if err == ErrNoResultsYet {
				time.Sleep(2 * time.Second)
				retries--
				continue
			}
			log.Errorf("cannot validate results: %v", err)
			return
		}

		myVotes := BuildProcessResult(myResults, nil).GetVotes()
		if len(myVotes) != len(results) {
			log.Errorf("results validation failed: wrong result questions size")
			return
		}
		for i, q := range results {
			if len(q.Question) != len(myVotes[i].Question) {
				log.Errorf("results validation failed: wrong question size")
				return
			}
			for j, v := range q.Question {
				if !bytes.Equal(v, myVotes[i].Question[j]) {
					log.Errorf("results validation failed: wrong question result")
					return
				}
			}
		}
		log.Infof("published results for process %x are correct!", pid)
	}()
	s.updateProcessPool = append(s.updateProcessPool, pid)
	return nil
}

// storeCountCaches stores the transaction, envelope, process, and entity count
// caches to the database
func (s *Scrutinizer) storeCountCaches() {
	if err := s.db.Upsert(indexertypes.Transactions, &indexertypes.CountStore{
		Type:  indexertypes.Transactions,
		Count: s.countTotalTransactions,
	}); err != nil {
		log.Errorf("cannot store transaction count: %v", err)
	}
	if err := s.db.Upsert(indexertypes.Envelopes, &indexertypes.CountStore{
		Type:  indexertypes.Envelopes,
		Count: s.countTotalEnvelopes,
	}); err != nil {
		log.Errorf("cannot store envelope count: %v", err)
	}
	if err := s.db.Upsert(indexertypes.Processes, &indexertypes.CountStore{
		Type:  indexertypes.Processes,
		Count: s.countTotalProcesses,
	}); err != nil {
		log.Errorf("cannot store process count: %v", err)
	}
	if err := s.db.Upsert(indexertypes.Entities, &indexertypes.CountStore{
		Type:  indexertypes.Entities,
		Count: s.countTotalEntities,
	}); err != nil {
		log.Errorf("cannot store entity count: %v", err)
	}
}

// GetFriendlyResults translates votes into a matrix of strings
func GetFriendlyResults(votes [][]*big.Int) [][]string {
	r := [][]string{}
	for i := range votes {
		r = append(r, []string{})
		for j := range votes[i] {
			r[i] = append(r[i], votes[i][j].String())
		}
	}
	return r
}
