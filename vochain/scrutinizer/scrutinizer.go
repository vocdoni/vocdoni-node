package scrutinizer

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v3"
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
	OnComputeResults(results *indexertypes.Results, process *indexertypes.Process, height uint32)
	OnOracleResults(oracleResults *models.ProcessResult, pid []byte, height uint32)
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (s *Scrutinizer) AddEventListener(l EventListener) {
	s.eventOnResults = append(s.eventOnResults, l)
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
	// newTxPool is the list of new tx references to be indexed
	newTxPool []*indexertypes.TxReference
	// list of live processes (those on which the votes will be computed on arrival)
	liveResultsProcs sync.Map
	// eventOnResults is the list of external callbacks that will be executed by the scrutinizer
	eventOnResults []EventListener
	db             *badgerhold.Store
	// envelopeHeightCache and countTotalEnvelopes are in memory counters that helps reducing the
	// access time when GenEnvelopeHeight() is called.
	envelopeHeightCache *lru.Cache
	// resultsCache is a memory cache for final results results, stores processId:<results>
	resultsCache *lru.Cache
	// addVoteLock is used to avoid Transaction Conflicts on the KV database.
	// It is not critical and the code should be able to recover from a Conflict, but we
	// try to minimize this situations in order to improve performance on the KV.
	// TODO (pau): remove this mutex and relay on the KV layer
	addVoteLock sync.RWMutex
	// voteTxLock is used to avoid Transaction Conflicts on the vote index KV database.
	voteTxLock sync.RWMutex
	// txIndexLock is used to avoid Transaction Conflicts on the transaction index KV database.
	txIndexLock sync.Mutex
	// recoveryBootLock prevents Commit() to add new votes while the recovery bootstratp is
	// being executed.
	recoveryBootLock sync.RWMutex
	// ignoreLiveResults if true, partial/live results won't be calculated (only final results)
	ignoreLiveResults bool
}

// VoteWithIndex holds a Vote and a txIndex. Model for the VotePool.
type VoteWithIndex struct {
	vote    *models.Vote
	txIndex int32
}

// NewScrutinizer returns an instance of the Scrutinizer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewScrutinizer(dbPath string, app *vochain.BaseApplication, countLiveResults bool) (*Scrutinizer, error) {
	s := &Scrutinizer{App: app, ignoreLiveResults: !countLiveResults}
	var err error
	s.db, err = InitDB(dbPath)
	if err != nil {
		return nil, err
	}
	startTime := time.Now()

	countMap, err := s.retrieveCounts()
	if err != nil {
		return nil, fmt.Errorf("could not create scrutinizer: %v", err)
	}

	log.Infof("indexer initialization took %s, stored %d "+
		"transactions, %d envelopes, %d processes and %d entities",
		time.Since(startTime),
		countMap[indexertypes.CountStoreTransactions],
		countMap[indexertypes.CountStoreEnvelopes],
		countMap[indexertypes.CountStoreProcesses],
		countMap[indexertypes.CountStoreEntities])
	// Subscrive to events
	s.App.State.AddEventListener(s)
	s.envelopeHeightCache = lru.New(countEnvelopeCacheSize)
	s.resultsCache = lru.New(resultsCacheSize)
	return s, nil
}

// retrieveCounts returns a count for txs, envelopes, processes, and entities, in that order.
// If no CountStore model is stored for the type, it counts all db entries of that type.
func (s *Scrutinizer) retrieveCounts() (map[uint8]uint64, error) {
	var err error
	txCountStore := new(indexertypes.CountStore)
	if err = s.db.Get(indexertypes.CountStoreTransactions, txCountStore); err != nil {
		log.Warnf("could not get the transaction count: %v", err)
		count, err := s.db.Count(&indexertypes.TxReference{}, &badgerhold.Query{})
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return nil, fmt.Errorf("could not count total transactions: %v", err)
			}
			// If keyNotFound error, ensure count is 0
			count = 0
		}
		// Store new countStore value
		txCountStore.Count = uint64(count)
		txCountStore.Type = indexertypes.CountStoreTransactions
		if err := s.db.Upsert(txCountStore.Type, txCountStore); err != nil {
			return nil, fmt.Errorf("could not store transaction count: %v", err)
		}
	}
	envelopeCountStore := new(indexertypes.CountStore)
	if err = s.db.Get(indexertypes.CountStoreEnvelopes, envelopeCountStore); err != nil {
		log.Warnf("could not get the envelope count: %v", err)
		count, err := s.db.Count(&indexertypes.VoteReference{}, &badgerhold.Query{})
		if err != nil && err != badger.ErrKeyNotFound {
			return nil, fmt.Errorf("could not count total envelopes: %v", err)
		}
		// Store new countStore value
		envelopeCountStore.Count = uint64(count)
		envelopeCountStore.Type = indexertypes.CountStoreEnvelopes
		if err := s.db.Upsert(envelopeCountStore.Type, envelopeCountStore); err != nil {
			return nil, fmt.Errorf("could not store envelope count: %v", err)
		}
	}
	processCountStore := new(indexertypes.CountStore)
	if err = s.db.Get(indexertypes.CountStoreProcesses, processCountStore); err != nil {
		log.Warnf("could not get the process count: %v", err)
		count, err := s.db.Count(&indexertypes.Process{}, &badgerhold.Query{})
		if err != nil && err != badger.ErrKeyNotFound {
			return nil, fmt.Errorf("could not count total processes: %v", err)
		}
		// Store new countStore value
		processCountStore.Count = uint64(count)
		processCountStore.Type = indexertypes.CountStoreProcesses
		if err := s.db.Upsert(processCountStore.Type, processCountStore); err != nil {
			return nil, fmt.Errorf("could not store process count: %v", err)
		}
	}
	entityCountStore := new(indexertypes.CountStore)
	if err = s.db.Get(indexertypes.CountStoreEntities, entityCountStore); err != nil {
		log.Warnf("could not get the entity count: %v", err)
		count, err := s.db.Count(&indexertypes.Entity{}, &badgerhold.Query{})
		if err != nil && err != badger.ErrKeyNotFound {
			return nil, fmt.Errorf("could not count total entities: %v", err)
		}
		// Store new countStore value
		entityCountStore.Count = uint64(count)
		entityCountStore.Type = indexertypes.CountStoreEntities
		if err := s.db.Upsert(entityCountStore.Type, entityCountStore); err != nil {
			return nil, fmt.Errorf("could not store entity count: %v", err)
		}
	}
	return map[uint8]uint64{
		indexertypes.CountStoreTransactions: txCountStore.Count,
		indexertypes.CountStoreEnvelopes:    envelopeCountStore.Count,
		indexertypes.CountStoreProcesses:    processCountStore.Count,
		indexertypes.CountStoreEntities:     entityCountStore.Count,
	}, nil
}

// AfterSyncBootstrap is a blocking function that waits until the Vochain is synchronized
// and then execute a set of recovery actions. It mainly checks for those processes which are
// still open (live) and updates all temporary data (current voting weight and live results
// if unecrypted). This method might be called on a goroutine after initializing the Scrutinizer.
// TO-DO: refactor and use blockHeight for reusing existing live results
func (s *Scrutinizer) AfterSyncBootstrap() {
	// if no live results, we don't need the bootstraping
	if s.ignoreLiveResults {
		return
	}
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
		if err := s.queryWithRetries(func() error {
			return s.db.Upsert(p, &indexertypes.Results{
				ProcessID: p,
				// MaxValue requires +1 since 0 is also an option
				Votes:        indexertypes.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
				Weight:       new(big.Int).SetUint64(0),
				VoteOpts:     options,
				EnvelopeType: process.GetEnvelopeType(),
				Signatures:   []types.HexBytes{},
			})
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

	// Index new transactions
	go s.indexNewTxs(s.newTxPool)

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
		s.voteTxLock.Lock()
		wg := sync.WaitGroup{}
		wg.Add(1)
		txn.CommitWith(func(err error) {
			if err != nil {
				log.Error(err)
			}
			wg.Done()
		})
		wg.Wait()
		s.voteTxLock.Unlock()
		log.Infof("indexed %d new envelopes, took %s",
			len(s.voteIndexPool), time.Since(startTime))

		if err := s.db.UpdateMatching(&indexertypes.CountStore{},
			badgerhold.Where(badgerhold.Key).Eq(indexertypes.CountStoreEnvelopes),
			func(record interface{}) error {
				update, ok := record.(*indexertypes.CountStore)
				if !ok {
					return fmt.Errorf("record isn't the correct type! Wanted CountStore, got %T", record)
				}
				update.Count += uint64(len(s.voteIndexPool))
				return nil
			},
		); err != nil {
			log.Errorf("could not get envelope count: %v", err)
		}
	}
	txn.Discard()

	// Add votes collected by onVote (live results)
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
	s.newTxPool = []*indexertypes.TxReference{}
}

// OnProcess scrutinizer stores the processID and entityID
func (s *Scrutinizer) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32) {
	data := &indexertypes.ScrutinizerOnProcessData{EntityID: eid, ProcessID: pid}
	s.newProcessPool = append(s.newProcessPool, data)
}

// OnVote scrutinizer stores the votes if the processId is live results (on going)
// and the blockchain is not synchronizing.
func (s *Scrutinizer) OnVote(v *models.Vote, txIndex int32) {
	if !s.ignoreLiveResults && s.isProcessLiveResults(v.ProcessId) {
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

// OnProcessResults verifies the results for a process and appends it to the updateProcessPool
func (s *Scrutinizer) OnProcessResults(pid []byte, results *models.ProcessResult,
	txIndex int32) error {
	// Execute callbacks
	for _, l := range s.eventOnResults {
		go l.OnOracleResults(results, pid, s.App.Height())
	}

	// We don't execute any action if the blockchain is being syncronized
	if s.App.IsSynchronizing() {
		return nil
	}

	// TODO: check results are valid and return an error if not.
	// This is very dangerous since an Oracle would be able to create a consensus failure,
	// the validaros (that do not check the results) and the full-nodes (with the scrutinizer enabled)
	// would compute different state hash.
	// As a temporary solution, lets compare results but just print the error.

	// This code must be run async in order to not delay the consensus. The results retreival
	// could require some time.
	go func() {
		if results == nil || results.Votes == nil {
			log.Errorf("results are nil")
			return
		}
		var myResults *indexertypes.Results
		var err error
		retries := 50
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

		myVotes := BuildProcessResult(myResults, results.EntityId).GetVotes()
		correct := len(myVotes) != len(results.Votes)
		for i, q := range results.GetVotes() {
			if !correct {
				break
			}
			if len(q.Question) != len(myVotes[i].Question) {
				log.Errorf("results validation failed: wrong question size")
				correct = false
				break
			}
			for j, v := range q.Question {
				if !bytes.Equal(v, myVotes[i].Question[j]) {
					log.Errorf("results validation failed: wrong question result")
					correct = false
					break
				}
			}
		}
		if correct {
			log.Infof("published results for process %x are correct", pid)
		} else {
			log.Warnf("published results for process %x are not correct", pid)
		}
	}()
	s.updateProcessPool = append(s.updateProcessPool, pid)
	return nil
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
