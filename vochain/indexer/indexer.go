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
	"strings"
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

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.23.0 generate

//go:embed migrations/*.sql
var embedMigrations embed.FS

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

	readOnlyDB  *sql.DB
	readWriteDB *sql.DB

	readOnlyQuery *indexerdb.Queries

	// blockMu protects blockTx, blockQueries, and blockUpdateProcs.
	blockMu sync.Mutex
	// blockTx is an in-progress SQL transaction which is committed or rolled
	// back along with the current block.
	blockTx *sql.Tx
	// blockQueries wraps blockTx. Note that it is kept between multiple transactions
	// so that we can reuse the same prepared statements.
	blockQueries *indexerdb.Queries
	// blockUpdateProcs is the list of process IDs that require sync with the state database.
	// The key is a types.ProcessID as a string, so that it can be used as a map key.
	blockUpdateProcs          map[string]bool
	blockUpdateProcVoteCounts map[string]bool

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

type Options struct {
	CountLiveResults bool
}

// New returns an instance of the Indexer
// using the local storage database in dataDir and integrated into the state vochain instance
func New(dataDir string, app *vochain.BaseApplication, opts Options) (*Indexer, error) {
	idx := &Indexer{
		App:               app,
		ignoreLiveResults: !opts.CountLiveResults,

		// TODO(mvdan): these three maps are all keyed by process ID,
		// and each of them needs to query existing data from the DB.
		// Since the map keys very often overlap, consider joining the maps
		// so that we can also reuse queries to the DB.
		votePool:                  make(map[string]map[string]*state.Vote),
		blockUpdateProcs:          make(map[string]bool),
		blockUpdateProcVoteCounts: make(map[string]bool),
	}
	log.Infow("indexer initialization", "dataDir", dataDir, "liveResults", opts.CountLiveResults)

	// The DB itself is opened in "rwc" mode, so it is created if it does not yet exist.
	// Create the parent directory as well if it doesn't exist.
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
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
	goose.SetLogger(log.GooseLogger())
	goose.SetBaseFS(embedMigrations)
	if err := goose.Up(idx.readWriteDB, "migrations"); err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}

	idx.readOnlyQuery, err = indexerdb.Prepare(context.TODO(), idx.readOnlyDB)
	if err != nil {
		return nil, err
	}
	idx.blockQueries, err = indexerdb.Prepare(context.TODO(), idx.readWriteDB)
	if err != nil {
		panic(err)
	}

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
	if idx.blockMu.TryLock() {
		panic("Indexer.blockTxQueries was called without locking Indexer.lockPool")
	}
	if idx.blockTx == nil {
		tx, err := idx.readWriteDB.Begin()
		if err != nil {
			panic(err) // shouldn't happen, use an error return if it ever does
		}
		idx.blockTx = tx
		idx.blockQueries = idx.blockQueries.WithTx(tx)
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

	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
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
			Votes:        results.NewEmptyVotes(options),
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.Envelope,
		}

		if _, err := queries.UpdateProcessResultByID(ctx, indexerdb.UpdateProcessResultByIDParams{
			ID:       indxR.ProcessID,
			Votes:    indexertypes.EncodeJSON(indxR.Votes),
			Weight:   indexertypes.EncodeJSON(indxR.Weight),
			VoteOpts: indexertypes.EncodeProtoJSON(indxR.VoteOpts),
			Envelope: indexertypes.EncodeProtoJSON(indxR.EnvelopeType),
		}); err != nil {
			log.Errorw(err, "cannot UpdateProcessResultByID sql")
			continue
		}

		// Count the votes, add them to partialResults (in memory, without any db transaction)
		partialResults := &results.Results{
			Weight:       new(types.BigInt).SetUint64(0),
			VoteOpts:     options,
			EnvelopeType: process.Envelope,
		}
		// Get the votes from the state
		idx.App.State.IterateVotes(p, true, func(vote *models.StateDBVote) bool {
			if err := idx.addLiveVote(process, vote.VotePackage, new(big.Int).SetBytes(vote.Weight), partialResults); err != nil {
				log.Errorw(err, "could not add live vote")
			}
			return false
		})
		// Store the results on the persistent database
		if err := idx.commitVotesUnsafe(queries, p, indxR, partialResults, nil, idx.App.Height()); err != nil {
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
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()

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
	clear(idx.blockUpdateProcs)

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
		// subtractedResults is used to subtract votes that are overwritten
		subtractedResults := &results.Results{
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
			// then it wasn't counted in the results yet, so don't add it to subtractedResults.
			// TODO: can we get previousVote from sqlite via blockTx?
			var previousVote *models.StateDBVote
			if v.Overwrites > 0 {
				previousVote, err = idx.App.State.Vote(v.ProcessID, v.Nullifier, true)
				if err != nil {
					log.Warnw("cannot get previous vote",
						"nullifier", hex.EncodeToString(v.Nullifier),
						"processID", hex.EncodeToString(v.ProcessID),
						"error", err.Error())
				}
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
				// add the live vote to subtracted results
				if err := idx.addLiveVote(proc, previousVote.VotePackage,
					new(big.Int).SetBytes(previousVote.Weight), subtractedResults); err != nil {
					log.Errorw(err, "vote cannot be added to subtracted results")
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
		func() {
			// If the recovery bootstrap is running, wait
			idx.recoveryBootLock.RLock()
			defer idx.recoveryBootLock.RUnlock()

			if err := idx.commitVotesUnsafe(queries, pid, proc.Results(), addedResults, subtractedResults, idx.App.Height()); err != nil {
				log.Errorf("cannot commit live votes from block %d: (%v)", err, height)
			}
		}()
	}
	clear(idx.votePool)

	// Note that we re-compute each process vote count from the votes table,
	// since simply incrementing the vote count would break with vote overwrites.
	for pidStr := range idx.blockUpdateProcVoteCounts {
		pid := []byte(pidStr)
		voteCount, err := queries.CountVotesByProcessID(ctx, pid)
		if err != nil {
			log.Errorw(err, "could not get vote count")
			continue
		}
		if _, err := queries.SetProcessVoteCount(ctx, indexerdb.SetProcessVoteCountParams{
			ID:        pid,
			VoteCount: voteCount,
		}); err != nil {
			log.Errorw(err, "could not set vote count")
		}
	}
	clear(idx.blockUpdateProcVoteCounts)

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
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	clear(idx.votePool)
	clear(idx.blockUpdateProcs)
	clear(idx.blockUpdateProcVoteCounts)
	if idx.blockTx != nil {
		if err := idx.blockTx.Rollback(); err != nil {
			log.Errorw(err, "could not rollback tx")
		}
		idx.blockTx = nil
	}
}

// OnProcess indexer stores the processID
func (idx *Indexer) OnProcess(pid, _ []byte, _, _ string, _ int32) {
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
	pid := string(vote.ProcessID)
	if !idx.ignoreLiveResults && idx.isProcessLiveResults(vote.ProcessID) {
		// Since []byte in Go isn't comparable, but we can convert any bytes to string.
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

	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if err := idx.addVoteIndex(context.TODO(), queries, vote, txIndex); err != nil {
		log.Errorw(err, "could not index vote")
	}
	idx.blockUpdateProcVoteCounts[pid] = true
}

// OnCancel indexer stores the processID and entityID
func (idx *Indexer) OnCancel(pid []byte, _ int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessKeys does nothing
func (idx *Indexer) OnProcessKeys(pid []byte, _ string, _ int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessStatusChange adds the process to blockUpdateProcs and, if ended, the resultsPool
func (idx *Indexer) OnProcessStatusChange(pid []byte, _ models.ProcessStatus, _ int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnRevealKeys checks if all keys have been revealed and in such case add the
// process to the results queue
func (idx *Indexer) OnRevealKeys(pid []byte, _ string, _ int32) {
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
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessResults verifies the results for a process and appends it to blockUpdateProcs
func (idx *Indexer) OnProcessResults(pid []byte, _ *models.ProcessResult, _ int32) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
}

// OnProcessesStart adds the processes to blockUpdateProcs.
// This is required to update potential changes when a process is started, such as the census root.
func (idx *Indexer) OnProcessesStart(pids [][]byte) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	for _, pid := range pids {
		idx.blockUpdateProcs[string(pid)] = true
	}
}

func (idx *Indexer) OnSetAccount(accountAddress []byte, account *state.Account) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateAccount(context.TODO(), indexerdb.CreateAccountParams{
		Account: accountAddress,
		Balance: int64(account.Balance),
		Nonce:   int64(account.Nonce),
	}); err != nil {
		log.Errorw(err, "cannot index new account")
	}
}

func (idx *Indexer) OnTransferTokens(tx *vochaintx.TokenTransfer) {
	t := time.Now()
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateTokenTransfer(context.TODO(), indexerdb.CreateTokenTransferParams{
		TxHash:       tx.TxHash,
		BlockHeight:  int64(idx.App.Height()),
		FromAccount:  tx.FromAddress.Bytes(),
		ToAccount:    tx.ToAddress.Bytes(),
		Amount:       int64(tx.Amount),
		TransferTime: t,
	}); err != nil {
		log.Errorw(err, "cannot index new transaction")
	}
}

// OnCensusUpdate adds the process to blockUpdateProcs in order to update the census.
// This function call is triggered by the SET_PROCESS_CENSUS tx.
func (idx *Indexer) OnCensusUpdate(pid, _ []byte, _ string) {
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	idx.blockUpdateProcs[string(pid)] = true
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

// OnSpendTokens indexes a token spending event.
func (idx *Indexer) OnSpendTokens(address []byte, txType models.TxType, cost uint64, reference string) {
	t := time.Now()
	idx.blockMu.Lock()
	defer idx.blockMu.Unlock()
	queries := idx.blockTxQueries()
	if _, err := queries.CreateTokenFee(context.TODO(), indexerdb.CreateTokenFeeParams{
		FromAccount: address,
		TxType:      strings.ToLower(txType.String()),
		Cost:        int64(cost),
		Reference:   reference,
		SpendTime:   t,
		BlockHeight: int64(idx.App.Height()),
	}); err != nil {
		log.Errorw(err, "cannot index new token spending")
	}
}

// GetTokenFeesByFromAccount returns all the token transfers made from a given account
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenFeesByFromAccount(from []byte, offset, maxItems int32) ([]*indexertypes.TokenFeeMeta, error) {
	ttFromDB, err := idx.readOnlyQuery.GetTokenFeesByFromAccount(context.TODO(), indexerdb.GetTokenFeesByFromAccountParams{
		FromAccount: from,
		Limit:       int64(maxItems),
		Offset:      int64(offset),
	})
	if err != nil {
		return nil, err
	}
	tt := []*indexertypes.TokenFeeMeta{}
	for _, t := range ttFromDB {
		tt = append(tt, &indexertypes.TokenFeeMeta{
			Cost:      uint64(t.Cost),
			From:      t.FromAccount,
			TxType:    t.TxType,
			Height:    uint64(t.BlockHeight),
			Reference: t.Reference,
			Timestamp: t.SpendTime,
		})
	}
	return tt, nil
}

// GetTokenFees returns all the token transfers from the database, ordered
// by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenFees(offset, maxItems int32) ([]*indexertypes.TokenFeeMeta, error) {
	ttFromDB, err := idx.readOnlyQuery.GetTokenFees(context.TODO(), indexerdb.GetTokenFeesParams{
		Limit:  int64(maxItems),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	tt := []*indexertypes.TokenFeeMeta{}
	for _, t := range ttFromDB {
		tt = append(tt, &indexertypes.TokenFeeMeta{
			Cost:      uint64(t.Cost),
			From:      t.FromAccount,
			TxType:    t.TxType,
			Height:    uint64(t.BlockHeight),
			Reference: t.Reference,
			Timestamp: t.SpendTime,
		})
	}
	return tt, nil
}

// GetTokenFeesByReference returns all the token fees associated with a given reference
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenFeesByReference(reference string, offset, maxItems int32) ([]*indexertypes.TokenFeeMeta, error) {
	ttFromDB, err := idx.readOnlyQuery.GetTokenFeesByReference(context.TODO(), indexerdb.GetTokenFeesByReferenceParams{
		Reference: reference,
		Limit:     int64(maxItems),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, err
	}
	tt := []*indexertypes.TokenFeeMeta{}
	for _, t := range ttFromDB {
		tt = append(tt, &indexertypes.TokenFeeMeta{
			Cost:      uint64(t.Cost),
			From:      t.FromAccount,
			TxType:    t.TxType,
			Height:    uint64(t.BlockHeight),
			Reference: t.Reference,
			Timestamp: t.SpendTime,
		})
	}
	return tt, nil
}

// GetTokenFeesByType returns all the token fees associated with a given transaction type
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenFeesByType(txType string, offset, maxItems int32) ([]*indexertypes.TokenFeeMeta, error) {
	ttFromDB, err := idx.readOnlyQuery.GetTokenFeesByTxType(context.TODO(), indexerdb.GetTokenFeesByTxTypeParams{
		TxType: txType,
		Limit:  int64(maxItems),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	tt := []*indexertypes.TokenFeeMeta{}
	for _, t := range ttFromDB {
		tt = append(tt, &indexertypes.TokenFeeMeta{
			Cost:      uint64(t.Cost),
			From:      t.FromAccount,
			TxType:    t.TxType,
			Height:    uint64(t.BlockHeight),
			Reference: t.Reference,
			Timestamp: t.SpendTime,
		})
	}
	return tt, nil
}

// GetTokenTransfersByToAccount returns all the token transfers made to a given account
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenTransfersByToAccount(to []byte, offset, maxItems int32) ([]*indexertypes.TokenTransferMeta, error) {
	ttFromDB, err := idx.readOnlyQuery.GetTokenTransfersByToAccount(context.TODO(), indexerdb.GetTokenTransfersByToAccountParams{
		ToAccount: to,
		Limit:     int64(maxItems),
		Offset:    int64(offset),
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

// GetTokenTransfersByAccount returns all the token transfers made to and from a given account
// from the database, ordered by timestamp and paginated by maxItems and offset
func (idx *Indexer) GetTokenTransfersByAccount(acc []byte, offset, maxItems int32) (indexertypes.TokenTransfersAccount, error) {
	transfersTo, err := idx.GetTokenTransfersByToAccount(acc, offset, maxItems)
	if err != nil {
		return indexertypes.TokenTransfersAccount{}, err
	}
	transfersFrom, err := idx.GetTokenTransfersByFromAccount(acc, offset, maxItems)
	if err != nil {
		return indexertypes.TokenTransfersAccount{}, err
	}

	return indexertypes.TokenTransfersAccount{
		Received: transfersTo,
		Sent:     transfersFrom}, nil
}

// CountTokenTransfersByAccount returns the count all the token transfers made from a given account
func (idx *Indexer) CountTokenTransfersByAccount(acc []byte) (uint64, error) {
	count, err := idx.readOnlyQuery.CountTokenTransfersByAccount(context.TODO(), acc)
	return uint64(count), err
}

// CountTotalAccounts returns the total number of accounts indexed.
func (idx *Indexer) CountTotalAccounts() (uint64, error) {
	count, err := idx.readOnlyQuery.CountAccounts(context.TODO())
	return uint64(count), err
}

func (idx *Indexer) GetListAccounts(offset, maxItems int32) ([]indexertypes.Account, error) {
	accsFromDB, err := idx.readOnlyQuery.GetListAccounts(context.TODO(), indexerdb.GetListAccountsParams{
		Limit:  int64(maxItems),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	tt := []indexertypes.Account{}
	for _, acc := range accsFromDB {
		tt = append(tt, indexertypes.Account{
			Address: acc.Account,
			Balance: uint64(acc.Balance),
			Nonce:   uint32(acc.Nonce),
		})
	}
	return tt, nil
}
