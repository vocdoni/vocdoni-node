package indexer

import (
	"context"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	indexerdb "go.vocdoni.io/dvote/vochain/indexer/db"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"

	"github.com/pressly/goose/v3"
	"golang.org/x/exp/maps"

	// modernc is a pure-Go version, but its errors have less useful info.
	// We use mattn while developing and testing, and we can swap them later.
	// _ "modernc.org/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

//go:generate go run github.com/kyleconroy/sqlc/cmd/sqlc@v1.17.2 generate

//go:embed migrations/*.sql
var embedMigrations embed.FS

const (
	// MaxEnvelopeListSize is the maximum number of envelopes a process can store.
	// 8.3M seems enough for now
	MaxEnvelopeListSize = 32 << 18
)

// EventListener is an interface used for executing custom functions during the
// events of the tally of a process.
type EventListener interface {
	OnComputeResults(results *results.Results, process *indexertypes.Process, height uint32)
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
	// votePool is the list of votes that should be live counted, grouped by processId
	votePool map[string][]*state.Vote

	// lockPool is the lock for all *Pool and blockTx operations
	lockPool sync.Mutex

	oneQuery *indexerdb.Queries

	// blockTx is an in-progress SQL transaction which is committed or rolled
	// back along with the current block.
	blockTx *sql.Tx

	// blockUpdateProcs is the list of process IDs that require sync with the state database.
	// The key is a types.ProcessID as a string, so that it can be used as a map key.
	blockUpdateProcs map[string]bool

	// list of live processes (those on which the votes will be computed on arrival)
	liveResultsProcs sync.Map // TODO: rethink with blockTx
	// eventOnResults is the list of external callbacks that will be executed by the indexer
	eventOnResults []EventListener
	sqlDB          *sql.DB
	// recoveryBootLock prevents Commit() to add new votes while the recovery bootstratp is
	// being executed.
	recoveryBootLock sync.RWMutex
	// ignoreLiveResults if true, partial/live results won't be calculated (only final results)
	ignoreLiveResults bool

	// In the tests, the extra 5s sleeps can make CI really slow at times, to
	// the point that it times out. Skip that in the tests.
	skipTargetHeightSleeps bool

	// Note that cancelling currently only stops asynchronous goroutines started
	// by Commit. In the future we could make it stop all other work as well,
	// like entire calls to Commit.
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
}

// NewIndexer returns an instance of the Indexer
// using the local storage database of dbPath and integrated into the state vochain instance
func NewIndexer(dbPath string, app *vochain.BaseApplication, countLiveResults bool) (*Indexer, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	idx := &Indexer{
		App:               app,
		ignoreLiveResults: !countLiveResults,

		blockUpdateProcs: make(map[string]bool),

		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	startTime := time.Now()

	countMap, err := idx.retrieveCounts()
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
	idx.sqlDB, err = sql.Open("sqlite3", sqlPath) // mattn
	if err != nil {
		return nil, err
	}
	// sqlite doesn't support multiple concurrent writers.
	// Since we execute queries from many goroutines, allowing multiple open
	// connections may lead to concurrent writes, resulting in confusing
	// "database is locked" errors. See TestIndexerConcurrentDB.
	// While here, also set other reasonable maximum values.
	idx.sqlDB.SetMaxOpenConns(1)
	idx.sqlDB.SetMaxIdleConns(2)
	idx.sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	idx.sqlDB.SetConnMaxLifetime(time.Hour)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}
	// goose.SetLogger(log.Logger()) // TODO: interfaces aren't compatible
	goose.SetBaseFS(embedMigrations)
	if err := goose.Up(idx.sqlDB, "migrations"); err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}

	idx.oneQuery = indexerdb.New(idx.sqlDB)

	// Subscribe to events
	idx.App.State.AddEventListener(idx)

	return idx, nil
}

func (idx *Indexer) Close() error {
	idx.cancelFunc()
	if err := idx.sqlDB.Close(); err != nil {
		return err
	}
	return nil
}

// blockTxQueries assumes that lockPool is locked.
func (idx *Indexer) blockTxQueries() *indexerdb.Queries {
	if idx.lockPool.TryLock() {
		panic("Indexer.blockTxQueries was called without locking Indexer.lockPool")
	}
	if idx.blockTx == nil {
		tx, err := idx.sqlDB.Begin()
		if err != nil {
			panic(err) // shouldn't happen, use an error return if it ever does
		}
		idx.blockTx = tx
	}
	return indexerdb.New(idx.blockTx)
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

	queries := idx.oneQuery // TODO: use a tx

	prcIDs, err := queries.GetProcessIDsByFinalResults(context.TODO(), false)
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
		options := process.VoteOptions

		indxR := &results.Results{
			ProcessID: p,
			// MaxValue requires +1 since 0 is also an option
			Votes:        results.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.EnvelopeType,
			Signatures:   []types.HexBytes{},
		}

		if _, err := queries.UpdateProcessResultByID(context.TODO(), indexerdb.UpdateProcessResultByIDParams{
			ID:         indxR.ProcessID,
			Votes:      encodeVotes(indxR.Votes),
			Weight:     indxR.Weight.String(),
			VoteOptsPb: encodedPb(indxR.VoteOpts),
			EnvelopePb: encodedPb(indxR.EnvelopeType),
		}); err != nil {
			log.Errorw(err, "cannot UpdateProcessResultByID sql")
			continue
		}

		// Count the votes, add them to results (in memory, without any db transaction)
		results := &results.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.EnvelopeType,
		}
		// Get the votes from the state
		idx.App.State.IterateVotes(p, true, func(vote *models.StateDBVote) bool {
			if err := idx.addLiveVote(process, vote.VotePackage, new(big.Int).SetBytes(vote.Weight), results); err != nil {
				log.Errorw(err, "could not add live vote")
			}
			return false
		})
		// Store the results on the persistent database
		if err := idx.commitVotesUnsafe(p, results, nil, idx.App.Height()); err != nil {
			log.Errorw(err, "could not commit live votes")
			continue
		}
		// Add process to live results so new votes will be added
		idx.addProcessToLiveResults(p)
	}
	log.Infof("live results recovery computation finished, took %s", time.Since(startTime))
}

// Commit is called by the APP when a block is confirmed and included into the chain
func (idx *Indexer) Commit(height uint32) error {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()

	// Update existing processes
	updateProcs := maps.Keys(idx.blockUpdateProcs)
	sort.Strings(updateProcs)

	queries := idx.blockTxQueries()
	ctx := context.TODO()

	for _, pidStr := range updateProcs {
		pid := types.ProcessID(pidStr)
		if err := idx.updateProcess(ctx, queries, pid); err != nil {
			log.Errorw(err, "commit: cannot update process")
			continue
		}
		log.Debugw("updated process", "processID", hex.EncodeToString(pid))
	}
	maps.Clear(idx.blockUpdateProcs)

	if err := idx.blockTx.Commit(); err != nil {
		log.Errorw(err, "could not commit tx")
	}
	idx.blockTx = nil

	// Add votes collected by onVote (live results)
	newVotes := 0
	overwritedVotes := 0
	startTime := time.Now()

	for pidStr, votes := range idx.votePool {
		pid := []byte(pidStr)
		// Get the process information
		proc, err := idx.ProcessInfo(pid)
		if err != nil {
			log.Warnf("cannot get process %x", pid)
			continue
		}
		process, err := idx.App.State.Process(pid, false)
		if err != nil {
			log.Errorf("cannot fetch process: %v", err)
			continue
		}

		// results is used to accumulate the new votes for a process
		addedResults := &results.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     proc.VoteOpts,
			EnvelopeType: proc.Envelope,
		}
		// substractedResults is used to substract votes that are overwritten
		substratedResults := &results.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     proc.VoteOpts,
			EnvelopeType: proc.Envelope,
		}
		for _, v := range votes {
			if v.Overwrites > 0 {
				// if overwrite is 1 or more, we need to update the vote (remove the previous
				// one and add the new) to results.
				// We fetch the previous vote from the state by setting committed=false
				previousVote, err := idx.App.State.Vote(v.ProcessID, v.Nullifier, true)
				if err != nil {
					log.Errorw(err, "previous vote cannot be fetch")
					continue
				}
				previousOverwrites := uint32(0)
				if previousVote.OverwriteCount != nil {
					previousOverwrites = *previousVote.OverwriteCount
				}
				log.Debugw("vote overwrite, previous vote",
					"overwrites", v.Overwrites,
					"package", string(previousVote.VotePackage))
				// ensure that overwriteCounter has increased
				if v.Overwrites <= previousOverwrites {
					log.Errorw(fmt.Errorf(
						"state stored overwrite count is equal or smaller than current vote overwrite count (%d <= %d)",
						v.Overwrites, previousOverwrites),
						"check vote overwrite failed")
					continue
				}
				// add the live vote to substracted results
				if err := idx.addLiveVote(process,
					previousVote.VotePackage,
					new(big.Int).SetBytes(previousVote.Weight),
					substratedResults); err != nil {
					log.Errorw(err, "vote cannot be added to substracted results")
					continue
				}
				overwritedVotes++
			} else {
				newVotes++
			}
			// add the new vote to results
			if err := idx.addLiveVote(process,
				v.VotePackage,
				v.Weight,
				addedResults); err != nil {
				log.Errorw(err, "vote cannot be added to results")
				continue
			}
		}
		// Commit votes (store to disk)
		if err := idx.commitVotes(pid, addedResults, substratedResults, idx.App.Height()); err != nil {
			log.Errorf("cannot commit live votes from block %d: (%v)", err, height)
		}
	}
	if newVotes+overwritedVotes > 0 {
		log.Infow("add live votes to results",
			"block", height, "newVotes", newVotes, "overwritedVotes",
			overwritedVotes, "time", time.Since(startTime))
	}

	return nil
}

// Rollback removes the non committed pending operations
func (idx *Indexer) Rollback() {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.votePool = make(map[string][]*state.Vote)
	if idx.blockTx != nil {
		if err := idx.blockTx.Rollback(); err != nil {
			log.Errorw(err, "could not rollback tx")
		}
		idx.blockTx = nil
	}
	maps.Clear(idx.blockUpdateProcs)
}

// OnProcess indexer stores the processID and entityID
func (idx *Indexer) OnProcess(pid, eid []byte, censusRoot, censusURI string, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	if err := idx.newEmptyProcess(pid); err != nil {
		log.Errorw(err, "commit: cannot create new empty process")
	}
	if !idx.App.IsSynchronizing() {
		idx.addProcessToLiveResults(pid)
	}
	log.Debugw("new process", "processID", hex.EncodeToString(pid))
}

// OnVote indexer stores the votes if the processId is live results (on going)
// and the blockchain is not synchronizing.
// voterID is the identifier of the voter, the most common case is an ethereum address
// but can be any kind of id expressed as bytes.
func (idx *Indexer) OnVote(vote *state.Vote, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	if !idx.ignoreLiveResults && idx.isProcessLiveResults(vote.ProcessID) {
		idx.votePool[string(vote.ProcessID)] = append(idx.votePool[string(vote.ProcessID)], vote)
	}

	queries := idx.blockTxQueries()
	if err := idx.addVoteIndex(context.TODO(), queries, vote, txIndex); err != nil {
		log.Errorw(err, "could not index vote")
	}
}

// OnCancel indexer stores the processID and entityID
func (idx *Indexer) OnCancel(pid []byte, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessKeys does nothing
func (idx *Indexer) OnProcessKeys(pid []byte, pub string, txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessStatusChange adds the process to blockUpdateProcs and, if ended, the resultsPool
func (idx *Indexer) OnProcessStatusChange(pid []byte, status models.ProcessStatus,
	txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
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
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessResults verifies the results for a process and appends it to blockUpdateProcs
func (idx *Indexer) OnProcessResults(pid []byte, presults *models.ProcessResult,
	txIndex int32) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessesStart adds the processes to blockUpdateProcs.
// This is required to update potential changes when a process is started, such as the rolling census.
func (idx *Indexer) OnProcessesStart(pids [][]byte) {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	for _, pid := range pids {
		idx.blockUpdateProcs[string(pid)] = true
	}
}

// NOT USED but required for implementing the vochain.EventListener interface
func (idx *Indexer) OnSetAccount(addr []byte, account *state.Account) {}

func (idx *Indexer) OnTransferTokens(tx *vochaintx.TokenTransfer) {
	if err := idx.indexTokenTransfer(tx); err != nil {
		log.Errorw(err, "cannot index new transaction")
	}
}

// newTokenTransfer creates a new token transfer and stores it in the database
func (idx *Indexer) indexTokenTransfer(tx *vochaintx.TokenTransfer) error {
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()

	queries := idx.blockTxQueries()
	if _, err := queries.CreateTokenTransfer(context.TODO(), indexerdb.CreateTokenTransferParams{
		TxHash:       tx.TxHash,
		Height:       int64(idx.App.Height()),
		FromAccount:  tx.FromAddress.Bytes(),
		ToAccount:    tx.ToAddress.Bytes(),
		Amount:       int64(tx.Amount),
		TransferTime: time.Now(),
	}); err != nil {
		return err
	}
	return nil
}

// GetTokenTransfersByFromAccount returns all the token transfers made from a given account
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenTransfersByFromAccount(from []byte, offset, maxItems int32) ([]*indexertypes.TokenTransferMeta, error) {
	ttFromDB, err := idx.oneQuery.GetTokenTransfersByFromAccount(context.TODO(), indexerdb.GetTokenTransfersByFromAccountParams{
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
