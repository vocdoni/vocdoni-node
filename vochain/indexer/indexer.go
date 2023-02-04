package indexer

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"

	"github.com/pressly/goose/v3"
	// modernc is a pure-Go version, but its errors have less useful info.
	// We use mattn while developing and testing, and we can swap them later.
	// _ "modernc.org/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

const (
	// MaxQuestions is the maximum number of questions allowed in a VotePackage
	MaxQuestions = 64
	// MaxOptions is the maximum number of options allowed in a VotePackage question
	MaxOptions = 128
	// MaxEnvelopeListSize is the maximum number of envelopes a process can store.
	// 8.3M seems enough for now
	MaxEnvelopeListSize = 32 << 18
)

// EventListener is an interface used for executing custom functions during the
// events of the tally of a process.
type EventListener interface {
	OnComputeResults(results *indexertypes.Results, process *indexertypes.Process, height uint32)
	OnOracleResults(oracleResults *models.ProcessResult, pid []byte, height uint32)
}

// AddEventListener adds a new event listener, to receive method calls on block
// events as documented in EventListener.
func (idx *Indexer) AddEventListener(l EventListener) {
	idx.eventOnResults = append(idx.eventOnResults, l)
}

// Indexer is the component which makes the accounting of the voting processes
// and keeps it indexed in a local database.
type Indexer struct {
	App *vochain.BaseApplication
	// voteIndexPool is the list of votes that will be indexed in the database
	voteIndexPool []*VoteWithIndex
	// votePool is the list of votes that should be live counted, grouped by processId
	votePool map[string][]*state.Vote
	// newProcessPool is the list of new process IDs on the current block
	newProcessPool []*indexertypes.IndexerOnProcessData
	// updateProcessPool is the list of process IDs that require sync with the state database
	updateProcessPool [][]byte
	// resultsPool is the list of processes that finish on the current block
	resultsPool []*indexertypes.IndexerOnProcessData
	// newTxPool is the list of new tx references to be indexed
	newTxPool []*indexertypes.TxReference
	// tokenTransferPool is the list of token transfers to be indexed
	tokenTransferPool []*indexertypes.TokenTransferMeta
	// lockPool is the lock for all *Pool operations
	lockPool sync.RWMutex
	// list of live processes (those on which the votes will be computed on arrival)
	liveResultsProcs sync.Map
	// eventOnResults is the list of external callbacks that will be executed by the indexer
	eventOnResults []EventListener
	sqlDB          *sql.DB
	// recoveryBootLock prevents Commit() to add new votes while the recovery bootstratp is
	// being executed.
	recoveryBootLock sync.RWMutex
	// ignoreLiveResults if true, partial/live results won't be calculated (only final results)
	ignoreLiveResults bool

	liveGoroutines atomic.Int64

	// In the tests, the extra 5s sleeps can make CI really slow at times, to
	// the point that it times out. Skip that in the tests.
	skipTargetHeightSleeps bool

	// Note that cancelling currently only stops asynchronous goroutines started
	// by Commit. In the future we could make it stop all other work as well,
	// like entire calls to Commit.
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
}

// VoteWithIndex holds a Vote and a txIndex. Model for the VotePool.
type VoteWithIndex struct {
	vote    *state.Vote
	txIndex int32
}

// NewIndexer returns an instance of the Indexer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewIndexer(dbPath string, app *vochain.BaseApplication, countLiveResults bool) (*Indexer, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	s := &Indexer{
		App:               app,
		ignoreLiveResults: !countLiveResults,

		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	startTime := time.Now()

	countMap, err := s.retrieveCounts()
	if err != nil {
		return nil, fmt.Errorf("could not create indexer: %v", err)
	}

	log.Infow("indexer initialization",
		"took", time.Since(startTime),
		"dataDir", dbPath,
		"liveResults", countLiveResults,
		"transactions", countMap[indexertypes.CountStoreTransactions],
		"envelopes", countMap[indexertypes.CountStoreEnvelopes],
		"processes", countMap[indexertypes.CountStoreProcesses],
		"entities", countMap[indexertypes.CountStoreEntities],
	)

	sqlPath := dbPath + "-sqlite"
	// s.sqlDB, err = sql.Open("sqlite", sqlPath) // modernc
	s.sqlDB, err = sql.Open("sqlite3", sqlPath) // mattn
	if err != nil {
		return nil, err
	}
	// sqlite doesn't support multiple concurrent writers.
	// Since we execute queries from many goroutines, allowing multiple open
	// connections may lead to concurrent writes, resulting in confusing
	// "database is locked" errors. See TestIndexerConcurrentDB.
	// While here, also set other reasonable maximum values.
	s.sqlDB.SetMaxOpenConns(1)
	s.sqlDB.SetMaxIdleConns(2)
	s.sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	s.sqlDB.SetConnMaxLifetime(time.Hour)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}
	// goose.SetLogger(log.Logger()) // TODO: interfaces aren't compatible
	goose.SetBaseFS(embedMigrations)
	if err := goose.Up(s.sqlDB, "migrations"); err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}

	// Subscrive to events
	s.App.State.AddEventListener(s)
	return s, nil
}

func (idx *Indexer) Close() error {
	idx.cancelFunc()
	idx.WaitIdle()
	if err := idx.sqlDB.Close(); err != nil {
		return err
	}
	return nil
}

// WaitIdle waits until there are no live asynchronous goroutines, such as those
// started by the Commit method.
//
// Note that this method is racy by nature if the indexer keeps committing
// more blocks, as it simply waits for any goroutines already started to stop.
func (idx *Indexer) WaitIdle() {
	var slept time.Duration
	for {
		const debounce = 100 * time.Millisecond
		time.Sleep(debounce)
		slept += debounce
		if idx.liveGoroutines.Load() == 0 {
			break
		}
		if slept > 10*time.Second {
			log.Warnf("giving up on WaitIdle after 10s")
			return
		}
	}
}

func (idx *Indexer) timeoutQueries() (*indexerdb.Queries, context.Context, context.CancelFunc) {
	ctx := context.TODO()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	queries := indexerdb.New(idx.sqlDB)
	return queries, ctx, cancel
}

// retrieveCounts returns a count for txs, envelopes, processes, and entities, in that order.
// If no CountStore model is stored for the type, it counts all db entries of that type.
func (idx *Indexer) retrieveCounts() (map[uint8]uint64, error) {
	txCountStore := new(indexertypes.CountStore)
	envelopeCountStore := new(indexertypes.CountStore)
	processCountStore := new(indexertypes.CountStore)
	entityCountStore := new(indexertypes.CountStore)
	// TODO(mvdan): implement on sqlite if needed
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
// if unecrypted). This method might be called on a goroutine after initializing the Indexer.
// TO-DO: refactor and use blockHeight for reusing existing live results
func (idx *Indexer) AfterSyncBootstrap() {
	// if no live results, we don't need the bootstraping
	if idx.ignoreLiveResults {
		return
	}

	// During the first seconds/milliseconds of the Vochain startup, Tendermint might report that
	// the chain is not synchronizing since it still does not have any peer and do not know the
	// actual size of the blockchain. If afterSyncBootStrap is executed on this specific moment,
	// the Wait loop would pass.
	syncSignals := 5
	for {
		// Add some grace time to avoid false positive on IsSynchronizing()
		if !idx.App.IsSynchronizing() {
			syncSignals--
		} else {
			syncSignals = 5
		}
		if syncSignals == 0 {
			break
		}
		time.Sleep(time.Second * 1)
	}
	log.Infof("running indexer after-sync bootstrap")
	// Block the new votes addition until the recovery finishes.
	idx.recoveryBootLock.Lock()
	defer idx.recoveryBootLock.Unlock()

	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()

	prcIDs, err := queries.GetProcessIDsByFinalResults(ctx, false)
	if err != nil {
		log.Error(err)
	}

	log.Infof("recovered %d live results processes", len(prcIDs))
	log.Infof("starting live results recovery computation")
	startTime := time.Now()
	for _, p := range prcIDs {
		// In order to recover the full list of live results, we need
		// to reset the existing Results and count them again from scratch.
		// Since we cannot be sure if there are votes missing, we need to
		// perform the full computation.
		log.Debugf("recovering live process %x", p)
		process, err := idx.App.State.Process(p, false)
		if err != nil {
			log.Errorf("cannot fetch process: %v", err)
			continue
		}
		options := process.GetVoteOptions()

		indxR := &indexertypes.Results{
			ProcessID: p,
			// MaxValue requires +1 since 0 is also an option
			Votes:        indexertypes.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.GetEnvelopeType(),
			Signatures:   []types.HexBytes{},
		}

		if _, err := queries.UpdateProcessResultByID(ctx, indexerdb.UpdateProcessResultByIDParams{
			ID:                indxR.ProcessID,
			Votes:             encodeVotes(indxR.Votes),
			Weight:            indxR.Weight.String(),
			VoteOptsPb:        encodedPb(indxR.VoteOpts),
			EnvelopePb:        encodedPb(indxR.EnvelopeType),
			ResultsSignatures: joinHexBytes(indxR.Signatures),
		}); err != nil {
			log.Errorw(err, "cannot UpdateProcessResultByID sql")
			continue
		}

		// Count the votes, add them to results (in memory, without any db transaction)
		results := &indexertypes.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.EnvelopeType,
		}
		// Get the votes from the state
		idx.App.State.IterateVotes(p, true, func(vote *models.StateDBVote) bool {
			if err := idx.addLiveVote(p, vote.VotePackage, new(big.Int).SetBytes(vote.Weight), results); err != nil {
				log.Errorw(err, "could not add live vote")
			}
			return false
		})
		// Store the results on the persistent database
		if err := idx.commitVotesUnsafe(p, results, idx.App.Height()); err != nil {
			log.Errorw(err, "could not commit live votes")
			continue
		}
		log.Infow("partial results recovered",
			"electionID", fmt.Sprintf("%x", p),
			"weight", results.Weight,
			"votes", len(results.Votes),
			"results", results.String(),
		)
		// Add process to live results so new votes will be added
		idx.addProcessToLiveResults(p)
	}
	log.Infof("live results recovery computation finished, took %s", time.Since(startTime))
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (idx *Indexer) Commit(height uint32) error {
	idx.lockPool.RLock()
	defer idx.lockPool.RUnlock()
	// Add Entity and register new active process
	for _, p := range idx.newProcessPool {
		if err := idx.newEmptyProcess(p.ProcessID); err != nil {
			log.Errorf("commit: cannot create new empty process: %v", err)
			continue
		}
		if !idx.App.IsSynchronizing() {
			idx.addProcessToLiveResults(p.ProcessID)
		}
	}

	// Update existing processes
	for _, p := range idx.updateProcessPool {
		if err := idx.updateProcess(p); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p, err)
		}
	}

	// Index new transactions
	idx.liveGoroutines.Add(1)
	go idx.indexNewTxs(idx.newTxPool)

	// Schedule results computation
	for _, p := range idx.resultsPool {
		if err := idx.setResultsHeight(p.ProcessID, height+1); err != nil {
			log.Errorf("commit: cannot update process %x: %v", p.ProcessID, err)
			continue
		}
		idx.delProcessFromLiveResults(p.ProcessID)
		log.Infof("scheduled results computation on next block for %x", p.ProcessID)
	}

	startTime := time.Now()
	for _, v := range idx.voteIndexPool {
		if err := idx.addVoteIndex(v.vote, v.txIndex); err != nil {
			log.Errorw(err, "could not index vote")
		}
	}
	if len(idx.voteIndexPool) > 0 {
		log.Infof("indexed %d new envelopes, took %s",
			len(idx.voteIndexPool), time.Since(startTime))
	}
	// index token transfers
	for _, tt := range idx.tokenTransferPool {
		if err := idx.newTokenTransfer(tt); err != nil {
			log.Errorw(err, "commit: cannot create new token transfer")
		}
	}
	idx.tokenTransferPool = []*indexertypes.TokenTransferMeta{}

	// Add votes collected by onVote (live results)
	nvotes := 0
	startTime = time.Now()

	for pid, votes := range idx.votePool {
		// Get the process information
		proc, err := idx.ProcessInfo([]byte(pid))
		if err != nil {
			log.Warnf("cannot get process %x", []byte(pid))
			continue
		}
		// This is a temporary "results" for computing votes
		// of a single processId for the current block.
		results := &indexertypes.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     proc.VoteOpts,
			EnvelopeType: proc.Envelope,
		}
		for _, v := range votes {
			if err := idx.addLiveVote(v.ProcessID,
				v.VotePackage,
				// TBD: Not 100% sure what happens if weight=nil
				v.Weight,
				results); err != nil {
				log.Warnf("vote cannot be added: %v", err)
			} else {
				nvotes++
			}
		}
		// Commit votes (store to disk)
		if err := idx.commitVotes([]byte(pid), results, idx.App.Height()); err != nil {
			log.Errorf("cannot commit live votes from block %d: (%v)", err, height)
		}
	}
	if nvotes > 0 {
		log.Infof("added %d live votes on block %d, took %s",
			nvotes, height, time.Since(startTime))
	}

	// Check if there are processes that need results computing.
	// This can be run asynchronously.
	// Note that we skip it if height==0, as some tests like TestResults use
	// an initial results height of 0, and we don't want to compute results
	// for such an initial height.
	if height > 0 {
		idx.liveGoroutines.Add(1)
		go idx.computePendingProcesses(height)
	}
	return nil
}

// Rollback removes the non committed pending operations
func (idx *Indexer) Rollback() {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.votePool = make(map[string][]*state.Vote)
	idx.voteIndexPool = []*VoteWithIndex{}
	idx.newProcessPool = []*indexertypes.IndexerOnProcessData{}
	idx.resultsPool = []*indexertypes.IndexerOnProcessData{}
	idx.updateProcessPool = [][]byte{}
	idx.newTxPool = []*indexertypes.TxReference{}
	idx.tokenTransferPool = []*indexertypes.TokenTransferMeta{}
}

// OnProcess indexer stores the processID and entityID
func (idx *Indexer) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	data := &indexertypes.IndexerOnProcessData{EntityID: eid, ProcessID: pid}
	idx.newProcessPool = append(idx.newProcessPool, data)
}

// OnVote indexer stores the votes if the processId is live results (on going)
// and the blockchain is not synchronizing.
// voterID is the identifier of the voter, the most common case is an ethereum address
// but can be any kind of id expressed as bytes.
func (idx *Indexer) OnVote(v *state.Vote, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	if !idx.ignoreLiveResults && idx.isProcessLiveResults(v.ProcessID) {
		idx.votePool[string(v.ProcessID)] = append(idx.votePool[string(v.ProcessID)], v)
	}
	idx.voteIndexPool = append(idx.voteIndexPool, &VoteWithIndex{vote: v, txIndex: txIndex})
}

// OnCancel indexer stores the processID and entityID
func (idx *Indexer) OnCancel(pid []byte, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.updateProcessPool = append(idx.updateProcessPool, pid)
}

// OnProcessKeys does nothing
func (idx *Indexer) OnProcessKeys(pid []byte, pub string, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.updateProcessPool = append(idx.updateProcessPool, pid)
}

// OnProcessStatusChange adds the process to the updateProcessPool and, if ended, the resultsPool
func (idx *Indexer) OnProcessStatusChange(pid []byte, status models.ProcessStatus,
	txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	if status == models.ProcessStatus_ENDED {
		if live, err := idx.isOpenProcess(pid); err != nil {
			log.Warn(err)
		} else if live {
			idx.resultsPool = append(idx.resultsPool, &indexertypes.IndexerOnProcessData{ProcessID: pid})
		}
	}
	idx.updateProcessPool = append(idx.updateProcessPool, pid)
}

// OnRevealKeys checks if all keys have been revealed and in such case add the
// process to the results queue
func (idx *Indexer) OnRevealKeys(pid []byte, priv string, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	p, err := idx.App.State.Process(pid, false)
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
		data := indexertypes.IndexerOnProcessData{EntityID: p.EntityId, ProcessID: pid}
		idx.resultsPool = append(idx.resultsPool, &data)
	}
	idx.updateProcessPool = append(idx.updateProcessPool, pid)
}

// OnProcessResults verifies the results for a process and appends it to the updateProcessPool
func (idx *Indexer) OnProcessResults(pid []byte, results *models.ProcessResult,
	txIndex int32) {
	// Execute callbacks
	for _, l := range idx.eventOnResults {
		go l.OnOracleResults(results, pid, idx.App.Height())
	}

	// We don't execute any action if the blockchain is being syncronized
	if idx.App.IsSynchronizing() {
		return
	}

	// TODO: check results are valid and return an error if not.
	// This is very dangerous since an Oracle would be able to create a consensus failure,
	// the validators (that do not check the results) and the full-nodes (with the indexer enabled)
	// would compute different state hash.
	// As a temporary solution, lets compare results but just print the error.

	// This code must be run async in order to not delay the consensus. The results retrieval
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
			myResults, err = idx.GetResults(pid)
			if err == nil {
				break
			}
			if errors.Is(err, ErrNoResultsYet) {
				time.Sleep(2 * time.Second)
				retries--
				continue
			}
			log.Errorf("cannot validate results: %v", err)
			return
		}

		myVotes := BuildProcessResult(myResults, results.EntityId).GetVotes()
		correct := len(myVotes) == len(results.Votes)
		if !correct {
			log.Errorf("results validation failed: wrong number of votes")
		}
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
			log.Errorf("published results for process %x are not correct", pid)
		}
	}()
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.updateProcessPool = append(idx.updateProcessPool, pid)
}

// OnProcessesStart adds the processes to the updateProcessPool.
// This is required to update potential changes when a process is started, such as the rolling census.
func (idx *Indexer) OnProcessesStart(pids [][]byte) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.updateProcessPool = append(idx.updateProcessPool, pids...)
}

// NOT USED but required for implementing the vochain.EventListener interface
func (idx *Indexer) OnSetAccount(addr []byte, account *state.Account) {}
func (idx *Indexer) OnTransferTokens(tx *vochaintx.TokenTransfer) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.tokenTransferPool = append(idx.tokenTransferPool, &indexertypes.TokenTransferMeta{
		From:      tx.FromAddress.Bytes(),
		To:        tx.ToAddress.Bytes(),
		Amount:    tx.Amount,
		Height:    uint64(idx.App.Height()),
		TxHash:    tx.TxHash,
		Timestamp: time.Now(),
	})
}

// newTokenTransfer creates a new token transfer and stores it in the database
func (idx *Indexer) newTokenTransfer(tt *indexertypes.TokenTransferMeta) error {
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	if _, err := queries.CreateTokenTransfer(ctx, indexerdb.CreateTokenTransferParams{
		TxHash:       tt.TxHash,
		Height:       int64(tt.Height),
		FromAccount:  tt.From,
		ToAccount:    tt.To,
		Amount:       int64(tt.Amount),
		TransferTime: tt.Timestamp,
	}); err != nil {
		return err
	}
	log.Debugw("new token transfer",
		"from", fmt.Sprintf("%x", tt.From),
		"to", fmt.Sprintf("%x", tt.To),
		"amount", fmt.Sprintf("%d", tt.Amount),
	)
	return nil
}

// GetTokenTransfersByFromAccount returns all the token transfers made from a given account
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenTransfersByFromAccount(from []byte, offset, maxItems int32) ([]*indexertypes.TokenTransferMeta, error) {
	queries, ctx, cancel := idx.timeoutQueries()
	defer cancel()
	ttFromDB, err := queries.GetTokenTransfersByFromAccount(ctx, indexerdb.GetTokenTransfersByFromAccountParams{
		FromAccount: from,
		Limit:       maxItems,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}
	tt := []*indexertypes.TokenTransferMeta{}
	for _, t := range ttFromDB {
		tt = append(tt, &indexertypes.TokenTransferMeta{
			Amount:    uint64(t.Amount),
			From:      t.FromAccount,
			To:        t.ToAccount,
			Height:    uint64(t.Height),
			TxHash:    t.TxHash,
			Timestamp: t.TransferTime,
		})
	}
	return tt, nil
}

// GetFriendlyResults translates votes into a matrix of strings
func GetFriendlyResults(votes [][]*types.BigInt) [][]string {
	r := [][]string{}
	for i := range votes {
		r = append(r, []string{})
		for j := range votes[i] {
			r[i] = append(r[i], votes[i][j].String())
		}
	}
	return r
}
