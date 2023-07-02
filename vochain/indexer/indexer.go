package indexer

import (
	"context"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
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

//go:generate go run github.com/kyleconroy/sqlc/cmd/sqlc@734e06ede7e68dc76e53f41727285abe5301dc69 generate

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

	// votePool is the set of votes that should be live counted,
	// first grouped by processId, then keyed by nullifier.
	// Only keeping one vote per nullifier is important for "overwrite" votes,
	// so that we only count the last one in the live results.
	// TODO: try using blockTx directly, after some more refactors?
	votePool map[string]map[string]*state.Vote

	// lockPool is the lock for all *Pool and blockTx operations
	// TODO: rename to blockMu
	lockPool sync.Mutex

	readOnlyDB  *sql.DB
	readWriteDB *sql.DB

	readOnlyQuery *indexerdb.Queries

	// blockTx is an in-progress SQL transaction which is committed or rolled
	// back along with the current block.
	blockTx      *sql.Tx
	blockQueries *indexerdb.Queries

	// blockUpdateProcs is the list of process IDs that require sync with the state database.
	// The key is a types.ProcessID as a string, so that it can be used as a map key.
	blockUpdateProcs map[string]bool

	// list of live processes (those on which the votes will be computed on arrival)
	// TODO: we could query the procs table, perhaps memoizing to avoid querying the same over and over again?
	liveResultsProcs sync.Map

	// eventOnResults is the list of external callbacks that will be executed by the indexer
	eventOnResults []EventListener

	// recoveryBootLock prevents Commit() to add new votes while the recovery bootstratp is
	// being executed.
	recoveryBootLock sync.RWMutex
	// ignoreLiveResults if true, partial/live results won't be calculated (only final results)
	ignoreLiveResults bool
}

// NewIndexer returns an instance of the Indexer
// using the local storage database in dataDir and integrated into the state vochain instance
func NewIndexer(dataDir string, app *vochain.BaseApplication, countLiveResults bool) (*Indexer, error) {
	idx := &Indexer{
		App:               app,
		ignoreLiveResults: !countLiveResults,

		blockUpdateProcs: make(map[string]bool),
	}
	log.Infow("indexer initialization", "dataDir", dataDir, "liveResults", countLiveResults)

	// The DB itself is opened in "rwc" mode, so it is created if it does not yet exist.
	// Create the parent directory as well if it doesn't exist.
	if err := os.MkdirAll(dataDir, 0o777); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "db.sqlite")
	var err error

	// sqlite doesn't support multiple concurrent writers.
	// For that reason, readWriteDB is limited to one open connection.
	// Per https://github.com/mattn/go-sqlite3/issues/1022#issuecomment-1067353980,
	// we use WAL to allow multiple concurrent readers at the same time.
	idx.readWriteDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=rwc&_journal_mode=wal&_txlock=immediate&_synchronous=normal", dbPath))
	if err != nil {
		return nil, err
	}
	idx.readWriteDB.SetMaxOpenConns(1)
	idx.readWriteDB.SetMaxIdleConns(2)
	idx.readWriteDB.SetConnMaxIdleTime(10 * time.Minute)
	idx.readWriteDB.SetConnMaxLifetime(time.Hour)

	idx.readOnlyDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro&_journal_mode=wal", dbPath))
	if err != nil {
		return nil, err
	}
	// Increasing these numbers can allow for more queries to run concurrently,
	// but it also increases the memory used by sqlite and our connection pool.
	// Most read-only queries we run are quick enough, so a small number seems OK.
	idx.readOnlyDB.SetMaxOpenConns(10)
	idx.readOnlyDB.SetMaxIdleConns(20)
	idx.readOnlyDB.SetConnMaxIdleTime(5 * time.Minute)
	idx.readOnlyDB.SetConnMaxLifetime(time.Hour)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}
	// goose.SetLogger(log.Logger()) // TODO: interfaces aren't compatible
	goose.SetBaseFS(embedMigrations)
	if err := goose.Up(idx.readWriteDB, "migrations"); err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}

	idx.readOnlyQuery = indexerdb.New(idx.readOnlyDB)

	// Subscribe to events
	idx.App.State.AddEventListener(idx)

	return idx, nil
}

func (idx *Indexer) Close() error {
	if err := idx.readOnlyDB.Close(); err != nil {
		return err
	}
	if err := idx.readWriteDB.Close(); err != nil {
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
		tx, err := idx.readWriteDB.Begin()
		if err != nil {
			panic(err) // shouldn't happen, use an error return if it ever does
		}
		idx.blockTx = tx
		idx.blockQueries = indexerdb.New(idx.blockTx)
	}
	return idx.blockQueries
}

// AfterSyncBootstrap is a blocking function that waits until the Vochain is synchronized
// and then execute a set of recovery actions. It mainly checks for those processes which are
// still open (live) and updates all temporary data (current voting weight and live results
// if unecrypted). This method might be called on a goroutine after initializing the Indexer.
// TO-DO: refactor and use blockHeight for reusing existing live results
func (idx *Indexer) AfterSyncBootstrap(inTest bool) {
	// if no live results, we don't need the bootstraping
	if idx.ignoreLiveResults {
		return
	}

	// During the first seconds/milliseconds of the Vochain startup, Tendermint might report that
	// the chain is not synchronizing since it still does not have any peer and do not know the
	// actual size of the blockchain. If afterSyncBootStrap is executed on this specific moment,
	// the Wait loop would pass.
	syncSignals := 5
	for !inTest {
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
	// TODO: perhaps redundant with lockPool now?
	idx.recoveryBootLock.Lock()
	defer idx.recoveryBootLock.Unlock()

	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	queries := idx.blockTxQueries()
	ctx := context.TODO()

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
		process, err := idx.ProcessInfo(p)
		if err != nil {
			log.Errorf("cannot fetch process: %v", err)
			continue
		}
		options := process.VoteOpts

		indxR := &results.Results{
			ProcessID: p,
			// MaxValue requires +1 since 0 is also an option
			Votes:        results.NewEmptyVotes(int(options.MaxCount), int(options.MaxValue)+1),
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.Envelope,
		}

		if _, err := queries.UpdateProcessResultByID(ctx, indexerdb.UpdateProcessResultByIDParams{
			ID:         indxR.ProcessID,
			Votes:      encodeVotes(indxR.Votes),
			Weight:     encodeBigint(indxR.Weight),
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
			EnvelopeType: process.Envelope,
		}
		// Get the votes from the state
		idx.App.State.IterateVotes(p, true, func(vote *models.StateDBVote) bool {
			if err := idx.addLiveVote(process, vote.VotePackage, new(big.Int).SetBytes(vote.Weight), results); err != nil {
				log.Errorw(err, "could not add live vote")
			}
			return false
		})
		// Store the results on the persistent database
		if err := idx.commitVotesUnsafe(queries, p, results, nil, idx.App.Height()); err != nil {
			log.Errorw(err, "could not commit live votes")
			continue
		}
		// Add process to live results so new votes will be added
		idx.addProcessToLiveResults(p)
	}

	// don't wait until the next Commit call to commit blockTx
	if err := idx.blockTx.Commit(); err != nil {
		log.Errorw(err, "could not commit tx")
	}
	idx.blockTx = nil

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

	// Add votes collected by onVote (live results)
	newVotes := 0
	overwritedVotes := 0
	startTime := time.Now()

	for pidStr, votesByNullifier := range idx.votePool {
		pid := []byte(pidStr)
		// Get the process information while reusing blockTx
		procInner, err := queries.GetProcess(ctx, pid)
		if err != nil {
			log.Warnf("cannot get process %x", pid)
			continue
		}
		proc := indexertypes.ProcessFromDB(&procInner)

		// results is used to accumulate the new votes for a process
		addedResults := &results.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     proc.VoteOpts,
			EnvelopeType: proc.Envelope,
		}
		// substractedResults is used to substract votes that are overwritten
		substractedResults := &results.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     proc.VoteOpts,
			EnvelopeType: proc.Envelope,
		}
		// The order here isn't deterministic, but we assume that to be OK.
		for _, v := range votesByNullifier {
			// If overwrite is 1 or more, we need to update the vote (remove the previous
			// one and add the new) to results.
			// We fetch the previous vote from the state by setting committed=true.
			// Note that if there wasn't a previous vote in the committed state,
			// then it wasn't counted in the results yet, so don't add it to substractedResults.
			// TODO: can we get previousVote from sqlite via blockTx?
			var previousVote *models.StateDBVote
			if v.Overwrites > 0 {
				previousVote, _ = idx.App.State.Vote(v.ProcessID, v.Nullifier, true)
			}
			if previousVote != nil {
				log.Debugw("vote overwrite, previous vote",
					"overwrites", v.Overwrites,
					"package", string(previousVote.VotePackage))
				// ensure that overwriteCounter has increased
				if v.Overwrites <= previousVote.GetOverwriteCount() {
					log.Errorw(fmt.Errorf(
						"state stored overwrite count is equal or smaller than current vote overwrite count (%d <= %d)",
						v.Overwrites, previousVote.GetOverwriteCount()),
						"check vote overwrite failed")
					continue
				}
				// add the live vote to substracted results
				if err := idx.addLiveVote(proc, previousVote.VotePackage,
					new(big.Int).SetBytes(previousVote.Weight), substractedResults); err != nil {
					log.Errorw(err, "vote cannot be added to substracted results")
					continue
				}
				overwritedVotes++
			} else {
				newVotes++
			}
			// add the new vote to results
			if err := idx.addLiveVote(proc, v.VotePackage, v.Weight, addedResults); err != nil {
				log.Errorw(err, "vote cannot be added to results")
				continue
			}
		}
		// Commit votes (store to disk)
		if err := idx.commitVotes(queries, pid, addedResults, substractedResults, idx.App.Height()); err != nil {
			log.Errorf("cannot commit live votes from block %d: (%v)", err, height)
		}
	}

	if err := idx.blockTx.Commit(); err != nil {
		log.Errorw(err, "could not commit tx")
	}
	idx.blockTx = nil

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
	idx.votePool = make(map[string]map[string]*state.Vote)
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
	if !idx.ignoreLiveResults && idx.isProcessLiveResults(vote.ProcessID) {
		// Since []byte in Go isn't comparable, but we can convert any bytes to string.
		pid := string(vote.ProcessID)
		nullifier := string(vote.Nullifier)
		if idx.votePool[pid] == nil {
			idx.votePool[pid] = make(map[string]*state.Vote)
		}
		prevVote := idx.votePool[pid][nullifier]
		if prevVote != nil && vote.Overwrites < prevVote.Overwrites {
			log.Warnw("OnVote called with a lower overwrite value than before",
				"previous", prevVote.Overwrites, "latest", vote.Overwrites)
		}
		idx.votePool[pid][nullifier] = vote
	}

	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
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
	// TODO: can we get KeyIndex from ProcessInfo? perhaps len(PublicKeys), or adding a new sqlite column?
	p, err := idx.App.State.Process(pid, false)
	if err != nil {
		log.Errorf("cannot fetch process %s from state: (%s)", pid, err)
		return
	}
	if p.KeyIndex == nil {
		log.Errorf("keyindex is nil")
		return
	}
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
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
	idx.lockPool.Lock()
	defer idx.lockPool.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateTokenTransfer(context.TODO(), indexerdb.CreateTokenTransferParams{
		TxHash:       tx.TxHash,
		BlockHeight:  int64(idx.App.Height()),
		FromAccount:  tx.FromAddress.Bytes(),
		ToAccount:    tx.ToAddress.Bytes(),
		Amount:       int64(tx.Amount),
		TransferTime: time.Now(),
	}); err != nil {
		log.Errorw(err, "cannot index new transaction")
	}
}

// GetTokenTransfersByFromAccount returns all the token transfers made from a given account
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenTransfersByFromAccount(from []byte, offset, maxItems int32) ([]*indexertypes.TokenTransferMeta, error) {
	ttFromDB, err := idx.readOnlyQuery.GetTokenTransfersByFromAccount(context.TODO(), indexerdb.GetTokenTransfersByFromAccountParams{
		FromAccount: from,
		Limit:       int64(maxItems),
		Offset:      int64(offset),
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
			Height:    uint64(t.BlockHeight),
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
			r[i] = append(r[i], encodeBigint(votes[i][j]))
		}
	}
	return r
}
