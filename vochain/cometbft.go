package vochain

import (
	"bytes"
	"cmp"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	cometabcitypes "github.com/cometbft/cometbft/abci/types"
	cometapitypes "github.com/cometbft/cometbft/api/cometbft/types/v1"
	crypto256k1 "github.com/cometbft/cometbft/crypto/secp256k1"
	comettypes "github.com/cometbft/cometbft/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/snapshot"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/ist"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// Info Return information about the application state.
// Used to sync Tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the Header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during Commit,
// ensuring that Commit is never called twice for the same block height.
//
// We use this method to initialize some state variables.
func (app *BaseApplication) Info(_ context.Context,
	req *cometabcitypes.InfoRequest) (*cometabcitypes.InfoResponse, error) {
	lastHeight, err := app.State.LastHeight()
	if err != nil {
		return nil, fmt.Errorf("cannot get State.LastHeight: %w", err)
	}
	app.State.SetHeight(lastHeight)
	appHash := app.State.CommittedHash()
	if err := app.State.SetElectionPriceCalc(); err != nil {
		return nil, fmt.Errorf("cannot set election price calc: %w", err)
	}
	// print some basic version info about tendermint components
	log.Infow("cometbft info", "cometVersion", req.Version, "p2pVersion",
		req.P2PVersion, "blockVersion", req.BlockVersion)
	log.Infow("telling cometbft our state",
		"LastBlockHeight", lastHeight,
		"LastBlockAppHash", hex.EncodeToString(appHash),
	)

	return &cometabcitypes.InfoResponse{
		LastBlockHeight:  int64(lastHeight),
		LastBlockAppHash: appHash,
	}, nil
}

// InitChain called once upon genesis
// InitChainResponse can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(_ context.Context,
	req *cometabcitypes.InitChainRequest) (*cometabcitypes.InitChainResponse, error) {
	// setting the app initial state with validators, height = 0 and empty apphash
	// unmarshal app state from genesis
	var genesisAppState genesis.AppState
	err := json.Unmarshal(req.AppStateBytes, &genesisAppState)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal app state bytes: %w", err)
	}
	// create accounts
	for _, acc := range genesisAppState.Accounts {
		addr := ethcommon.BytesToAddress(acc.Address)
		if err := app.State.CreateAccount(addr, "", nil, acc.Balance); err != nil {
			if err != state.ErrAccountAlreadyExists {
				return nil, fmt.Errorf("cannot create acount %x: %w", addr, err)
			}
			if err := app.State.InitChainMintBalance(addr, acc.Balance); err != nil {
				return nil, fmt.Errorf("cannot initialize chain minintg balance: %w", err)
			}
		}
		log.Infow("created account", "addr", addr.Hex(), "tokens", acc.Balance)
	}
	// get validators
	// TODO pau: unify this code with the one on apputils.go that essentially does the same
	tendermintValidators := []cometabcitypes.ValidatorUpdate{}
	for i := 0; i < len(genesisAppState.Validators); i++ {
		log.Infow("add genesis validator",
			"signingAddress", genesisAppState.Validators[i].Address.String(),
			"consensusPubKey", genesisAppState.Validators[i].PubKey.String(),
			"power", genesisAppState.Validators[i].Power,
			"name", genesisAppState.Validators[i].Name,
			"keyIndex", genesisAppState.Validators[i].KeyIndex,
		)
		v := &models.Validator{
			Address:          genesisAppState.Validators[i].Address,
			PubKey:           genesisAppState.Validators[i].PubKey,
			Power:            genesisAppState.Validators[i].Power,
			KeyIndex:         uint32(genesisAppState.Validators[i].KeyIndex),
			Name:             genesisAppState.Validators[i].Name,
			ValidatorAddress: crypto256k1.PubKey(genesisAppState.Validators[i].PubKey).Address().Bytes(),
		}
		if err = app.State.AddValidator(v); err != nil {
			return nil, fmt.Errorf("cannot add validator %s: %w", log.FormatProto(v), err)
		}
		addr := ethcommon.BytesToAddress(v.Address)
		if err := app.State.CreateAccount(addr, "", nil, 0); err != nil {
			if err != state.ErrAccountAlreadyExists {
				return nil, fmt.Errorf("cannot create validator acount %x: %w", addr, err)
			}
		}
		tendermintValidators = append(tendermintValidators,
			cometabcitypes.UpdateValidator(
				genesisAppState.Validators[i].PubKey,
				int64(genesisAppState.Validators[i].Power),
				crypto256k1.KeyType,
			))
	}
	myValidator, err := app.State.Validator(app.NodeAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get node validator: %w", err)
	}
	if myValidator != nil {
		log.Infow("node is a validator!", "power", myValidator.Power, "name", myValidator.Name,
			"address", hex.EncodeToString(myValidator.Address), "pubKey", hex.EncodeToString(myValidator.PubKey),
			"validatorAddr", hex.EncodeToString(myValidator.ValidatorAddress))
	}

	// add tx costs
	for k, v := range genesisAppState.TxCost.AsMap() {
		err = app.State.SetTxBaseCost(k, v)
		if err != nil {
			return nil, fmt.Errorf("could not set tx cost %q to value %q from genesis file to the State", k, v)
		}
	}

	// create burn account
	if err := app.State.SetAccount(state.BurnAddress, &state.Account{}); err != nil {
		return nil, fmt.Errorf("unable to set burn address")
	}

	// set max election size
	if err := app.State.SetMaxProcessSize(genesisAppState.MaxElectionSize); err != nil {
		return nil, fmt.Errorf("unable to set max election size")
	}

	// set network capacity
	if err := app.State.SetNetworkCapacity(genesisAppState.NetworkCapacity); err != nil {
		return nil, fmt.Errorf("unable to set  network capacity")
	}

	// initialize election price calc
	if err := app.State.SetElectionPriceCalc(); err != nil {
		return nil, fmt.Errorf("cannot set election price calc: %w", err)
	}

	// set initial timestamp
	if err := app.State.SetTimestamp(uint32(req.GetTime().Unix())); err != nil {
		return nil, fmt.Errorf("cannot set timestamp: %w", err)
	}

	// commit state and get hash
	hash, err := app.State.PrepareCommit()
	if err != nil {
		return nil, fmt.Errorf("cannot prepare commit: %w", err)
	}
	if _, err = app.State.Save(); err != nil {
		return nil, fmt.Errorf("cannot save state: %w", err)
	}
	return &cometabcitypes.InitChainResponse{
		Validators: tendermintValidators,
		AppHash:    hash,
	}, nil
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(_ context.Context, req *cometabcitypes.CheckTxRequest) (*cometabcitypes.CheckTxResponse, error) {
	if req == nil || req.Tx == nil {
		return &cometabcitypes.CheckTxResponse{
			Code: 1,
			Data: []byte("nil request or tx"),
		}, nil
	}
	txReference := vochaintx.TxKey(req.Tx)
	ref, ok := app.txReferences.Load(txReference)
	if !ok {
		// store the initial height of the tx if its the first time we see it
		app.txReferences.Store(txReference, &pendingTxReference{
			height: app.Height(),
		})
	} else {
		height := ref.(*pendingTxReference).height
		// check if the tx is referenced by a previous block and the TTL has expired
		if app.Height() > height+transactionBlocksTTL {
			// remove tx reference and return checkTx error
			log.Debugw("pruning expired tx from mempool", "height", app.Height(), "hash", fmt.Sprintf("%x", txReference))
			app.txReferences.Delete(txReference)
			return &cometabcitypes.CheckTxResponse{Code: 1, Data: []byte(fmt.Sprintf("tx expired %x", txReference))}, nil
		}
	}
	// execute recheck mempool every recheckTxHeightInterval blocks
	if req.Type == cometabcitypes.CHECK_TX_TYPE_RECHECK {
		if app.Height()%recheckTxHeightInterval != 0 {
			return &cometabcitypes.CheckTxResponse{Code: 0}, nil
		}
	}
	// unmarshal tx and check it
	tx := new(vochaintx.Tx)
	if err := tx.Unmarshal(req.Tx, app.ChainID()); err != nil {
		return &cometabcitypes.CheckTxResponse{Code: 1, Data: []byte(err.Error())}, nil
	}
	response, err := app.TransactionHandler.CheckTx(tx, false)
	if err != nil {
		if errors.Is(err, transaction.ErrorAlreadyExistInCache) {
			return &cometabcitypes.CheckTxResponse{Code: 0}, nil
		}
		log.Errorw(err, "checkTx")
		return &cometabcitypes.CheckTxResponse{Code: 1, Data: []byte(err.Error())}, nil
	}
	return &cometabcitypes.CheckTxResponse{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}, nil
}

// FinalizeBlock is executed by cometBFT when a new block is decided.
// Cryptographic commitments to the block and transaction results, returned via the corresponding
// parameters in FinalizeBlockResponse, are included in the header of the next block.
func (app *BaseApplication) FinalizeBlock(_ context.Context,
	req *cometabcitypes.FinalizeBlockRequest) (*cometabcitypes.FinalizeBlockResponse, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	start := time.Now()
	height := uint32(req.GetHeight())

	var root []byte
	var resp []*DeliverTxResponse
	// skip execution if we already have the results and root (from ProcessProposal)
	// or if the stored lastBlockHash is different from the one requested.
	if app.lastRootHash == nil || !bytes.Equal(app.lastBlockHash, req.GetHash()) {
		result, err := app.ExecuteBlock(req.Txs, height, req.GetTime())
		if err != nil {
			return nil, fmt.Errorf("cannot execute block: %w", err)
		}
		root = result.Root
		resp = result.Responses
	} else {
		root = make([]byte, len(app.lastRootHash))
		copy(root, app.lastRootHash[:])
		resp = app.lastDeliverTxResponse
	}

	txResults := make([]*cometabcitypes.ExecTxResult, len(req.Txs))
	for i, tx := range resp {
		txResults[i] = &cometabcitypes.ExecTxResult{
			Code: tx.Code,
			Data: tx.Data,
			Log:  tx.Log,
			Info: tx.Info,
		}
	}

	if len(req.Txs) > 0 {
		log.Debugw("finalize block", "height", height,
			"txs", len(req.Txs), "hash", hex.EncodeToString(root),
			"milliSeconds", time.Since(start).Milliseconds(),
			"proposer", hex.EncodeToString(req.GetProposerAddress()))
	}

	// update validator score as an IST action for the next block. Note that at this point,
	// we cannot modify the state or we would break ProcessProposal optimistic execution
	proposalVotes := [][]byte{}
	for _, v := range req.GetDecidedLastCommit().Votes {
		if idFlag := v.GetBlockIdFlag(); idFlag == cometapitypes.BlockIDFlagAbsent || idFlag == cometapitypes.BlockIDFlagUnknown {
			// skip invalid votes
			continue
		}
		proposalVotes = append(proposalVotes, v.GetValidator().Address)
	}
	if err := app.Istc.Schedule(ist.Action{
		TypeID:            ist.ActionUpdateValidatorScore,
		ValidatorVotes:    proposalVotes,
		ValidatorProposer: req.GetProposerAddress(),
		ID:                ethereum.HashRaw([]byte(fmt.Sprintf("validators-update-score-%d", height))),
		Height:            height + 1,
	}); err != nil {
		return nil, fmt.Errorf("finalize block: could not schedule IST action: %w", err)
	}

	// update current validators
	validators, err := app.State.Validators(false)
	if err != nil {
		return nil, fmt.Errorf("cannot get validators: %w", err)
	}
	return &cometabcitypes.FinalizeBlockResponse{
		AppHash:          root,
		TxResults:        txResults,
		ValidatorUpdates: validatorUpdate(validators),
	}, nil
}

func validatorUpdate(validators map[string]*models.Validator) cometabcitypes.ValidatorUpdates {
	validatorUpdate := []cometabcitypes.ValidatorUpdate{}
	for _, v := range validators {
		pubKey := make([]byte, len(v.PubKey))
		copy(pubKey, v.PubKey)
		validatorUpdate = append(validatorUpdate, cometabcitypes.UpdateValidator(
			pubKey,
			int64(v.Power),
			crypto256k1.KeyType,
		))
	}
	return validatorUpdate
}

// Commit is the CometBFT implementation of the ABCI Commit method. We currently do nothing here.
func (app *BaseApplication) Commit(_ context.Context, _ *cometabcitypes.CommitRequest) (*cometabcitypes.CommitResponse, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	// save state and get hash
	_, err := app.CommitState()
	if err != nil {
		return nil, err
	}
	return &cometabcitypes.CommitResponse{
		RetainHeight: 0, // When snapshot sync enabled, we can start to remove old blocks
	}, nil
}

// PrepareProposal allows the block proposer to perform application-dependent work in a block before
// proposing it. This enables, for instance, batch optimizations to a block, which has been empirically
// demonstrated to be a key component for improved performance. Method PrepareProposal is called every
// time CometBFT is about to broadcast a Proposal message and validValue is nil. CometBFT gathers
// outstanding transactions from the mempool, generates a block header, and uses them to create a block
// to propose. Then, it calls PrepareProposalRequest with the newly created proposal, called raw proposal.
// The Application can make changes to the raw proposal, such as modifying the set of transactions or the
// order in which they appear, and returns the (potentially) modified proposal, called prepared proposal in
// the PrepareProposalResponse call. The logic modifying the raw proposal MAY be non-deterministic.
func (app *BaseApplication) PrepareProposal(ctx context.Context,
	req *cometabcitypes.PrepareProposalRequest) (*cometabcitypes.PrepareProposalResponse, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	startTime := time.Now()

	if f := config.ForksForChainID(app.chainID); req.GetHeight() >= int64(f.LTS13Fork) {
		return nil, fmt.Errorf("current chain %q reached planned EOL at height %d", app.chainID, req.GetHeight())
	}

	type txInfo struct {
		Data      []byte
		Addr      *ethcommon.Address
		Nonce     uint32
		DecodedTx *vochaintx.Tx
	}
	// Rollback the state to discard previous non saved changes.
	// It might happen if ProcessProposal fails and then this node is selected for preparing the next proposal.
	app.State.Rollback()

	// Get and execute transactions from the mempool
	validTxInfos := []txInfo{}
	for _, tx := range req.GetTxs() {
		vtx := new(vochaintx.Tx)
		if err := vtx.Unmarshal(tx, app.ChainID()); err != nil {
			// invalid transaction
			app.MempoolDeleteTx(vochaintx.TxKey(tx))
			log.Warnw("could not unmarshal transaction", "err", err)
			continue
		}
		senderAddr, nonce, err := app.TransactionHandler.ExtractNonceAndSender(vtx)
		if err != nil {
			log.Warnw("could not extract nonce and/or sender from transaction", "err", err)
			continue
		}

		validTxInfos = append(validTxInfos, txInfo{
			Data:      tx,
			Addr:      senderAddr,
			Nonce:     nonce,
			DecodedTx: vtx,
		})
	}

	// Sort the transactions based on the sender's address and nonce
	slices.SortFunc(validTxInfos, func(a, b txInfo) int {
		// If both addresses are equal, compare by nonce.
		if a.Addr == nil && b.Addr == nil {
		} else if a.Addr == nil {
			return -1 // a < b when only a is nil
		} else if b.Addr == nil {
			return 1 // a > b when only b is nil
		} else if c := a.Addr.Cmp(*b.Addr); c != 0 {
			return c
		}
		return cmp.Compare(a.Nonce, b.Nonce)
	})

	// Check the validity of the transactions
	validTxs := [][]byte{}
	for _, txInfo := range validTxInfos {
		// Check the validity of the transaction using forCommit true
		resp, err := app.TransactionHandler.CheckTx(txInfo.DecodedTx, true)
		if err != nil {
			log.Warnw("discard invalid tx on prepare proposal",
				"err", err,
				"hash", fmt.Sprintf("%x", txInfo.DecodedTx.TxID),
				"data", func() string {
					if resp != nil {
						return string(resp.Data)
					}
					return ""
				}())
			// remove transaction from mempool if max attempts reached
			val, ok := app.txReferences.Load(txInfo.DecodedTx.TxID)
			if ok {
				val.(*pendingTxReference).failedCount++
				if val.(*pendingTxReference).failedCount > maxPendingTxAttempts {
					log.Debugf("transaction %x has reached max attempts, remove from mempool", txInfo.DecodedTx.TxID)
					app.MempoolDeleteTx(txInfo.DecodedTx.TxID)
					app.txReferences.Delete(txInfo.DecodedTx.TxID)
				} else {
					app.txReferences.Store(txInfo.DecodedTx.TxID, val)
				}
			}
			log.Warnw("transaction reference not found on prepare proposal!",
				"hash", fmt.Sprintf("%x", txInfo.DecodedTx.TxID),
			)
			continue
		}
		validTxs = append(validTxs, txInfo.Data)
	}

	// Rollback the state to discard the changes made
	app.State.Rollback()
	log.Debugw("prepare proposal", "height", app.Height(), "txs", len(validTxs),
		"milliSeconds", time.Since(startTime).Milliseconds())
	return &cometabcitypes.PrepareProposalResponse{
		Txs: validTxs,
	}, nil
}

// ProcessProposal allows a validator to perform application-dependent work in a proposed block. This enables
// features such as immediate block execution, and allows the Application to reject invalid blocks.
// CometBFT calls it when it receives a proposal and validValue is nil. The Application cannot modify the
// proposal at this point but can reject it if invalid. If that is the case, the consensus algorithm will
// prevote nil on the proposal, which has strong liveness implications for CometBFT. As a general rule, the
// Application SHOULD accept a prepared proposal passed via ProcessProposal, even if a part of the proposal
// is invalid (e.g., an invalid transaction); the Application can ignore the invalid part of the prepared
// proposal at block execution time. The logic in ProcessProposal MUST be deterministic.
func (app *BaseApplication) ProcessProposal(_ context.Context,
	req *cometabcitypes.ProcessProposalRequest) (*cometabcitypes.ProcessProposalResponse, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	// Check if the node is a validator, if not, just accept the proposal and return (nothing to say)
	validator, err := app.State.Validator(app.NodeAddress, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get node validator info: %w", err)
	}
	if validator == nil {
		// we reset the last deliver tx response and root hash to avoid using them at finalize block
		app.lastDeliverTxResponse = nil
		app.lastRootHash = nil
		app.lastBlockHash = nil
		return &cometabcitypes.ProcessProposalResponse{Status: cometabcitypes.PROCESS_PROPOSAL_STATUS_ACCEPT}, nil
	}

	startTime := time.Now()
	resp, err := app.ExecuteBlock(req.Txs, uint32(req.GetHeight()), req.GetTime())
	if err != nil {
		return nil, fmt.Errorf("cannot execute block on process proposal: %w", err)
	}
	// invalid tx on a proposed block, should actually never happened if proposer acts honestly
	if len(resp.InvalidTransactions) > 0 {
		log.Warnw("invalid transactions on process proposal", "height", app.Height(), "count", len(resp.InvalidTransactions),
			"proposer", hex.EncodeToString(req.ProposerAddress), "action", "reject")
		for _, tx := range resp.InvalidTransactions {
			// remove transaction from mempool, just in case we have it
			app.MempoolDeleteTx(tx)
			log.Debugw("remove invalid tx from mempool", "hash", fmt.Sprintf("%x", tx))
		}
		return &cometabcitypes.ProcessProposalResponse{
			Status: cometabcitypes.PROCESS_PROPOSAL_STATUS_REJECT,
		}, nil
	}
	app.lastDeliverTxResponse = resp.Responses
	app.lastRootHash = resp.Root
	app.lastBlockHash = req.GetHash()
	log.Debugw("process proposal", "height", app.Height(), "txs", len(req.Txs), "action", "accept",
		"blockHash", hex.EncodeToString(app.lastBlockHash),
		"hash", hex.EncodeToString(resp.Root), "milliSeconds", time.Since(startTime).Milliseconds())
	return &cometabcitypes.ProcessProposalResponse{
		Status: cometabcitypes.PROCESS_PROPOSAL_STATUS_ACCEPT,
	}, nil
}

// Example StateSync (Snapshot) successful flow:
// Alice is a comet node, up-to-date with RPC open on port 26657
// Bob is a fresh node that is bootstrapping, with params:
//    * StateSync.Enable = true
//    * StateSync.RPCServers = alice:26657, alice:26657
// * Bob comet will ask Alice for snapshots (app doesn't intervene here)
// * Alice comet calls ListSnapshots, App returns a []*v1.Snapshot
// * Bob comet calls OfferSnapshot passing a single *v1.Snapshot, App returns ACCEPT
// * Bob comet asks Alice for chunks (app doesn't intervene here)
// * Alice comet calls (N times) LoadSnapshotChunk passing height, format, chunk index. App returns []byte
// * Bob comet calls (N times) ApplySnapshotChunk passing []byte, chunk index and sender. App returns ACCEPT

// ListSnapshots provides cometbft with a list of available snapshots.
func (app *BaseApplication) ListSnapshots(_ context.Context,
	_ *cometabcitypes.ListSnapshotsRequest,
) (*cometabcitypes.ListSnapshotsResponse, error) {
	list := app.Snapshots.List()

	response := &cometabcitypes.ListSnapshotsResponse{}
	for height, snap := range list {
		chunks := uint32(math.Ceil(float64(snap.Size()) / float64(app.Snapshots.ChunkSize)))
		metadataBytes, err := json.Marshal(snap.Header())
		if err != nil {
			log.Errorw(err, "couldn't marshal snapshot metadata")
			continue
		}
		response.Snapshots = append(response.Snapshots, &cometabcitypes.Snapshot{
			Height:   uint64(height),
			Format:   0,
			Chunks:   chunks,
			Hash:     snap.Header().Hash,
			Metadata: metadataBytes,
		})
	}
	log.Debugf("cometbft requests our list of snapshots, we offer %d options", len(response.Snapshots))
	return response, nil
}

// snapshotFromComet is used when receiving a Snapshot from CometBFT.
// Only 1 snapshot at a time is processed by CometBFT, hence we keep it simple.
var snapshotFromComet struct {
	height atomic.Int64
	chunks atomic.Int32
}

// OfferSnapshot is called by cometbft during StateSync, when another node offers a Snapshot.
func (app *BaseApplication) OfferSnapshot(_ context.Context,
	req *cometabcitypes.OfferSnapshotRequest,
) (*cometabcitypes.OfferSnapshotResponse, error) {
	log.Debugw("cometbft offers us a snapshot",
		"appHash", hex.EncodeToString(req.AppHash),
		"snapHash", hex.EncodeToString(req.Snapshot.Hash),
		"height", req.Snapshot.Height, "format", req.Snapshot.Format, "chunks", req.Snapshot.Chunks)

	var metadata snapshot.SnapshotHeader
	if err := json.Unmarshal(req.Snapshot.Metadata, &metadata); err != nil {
		log.Errorw(err, "couldn't unmarshal snapshot metadata")
		return &cometabcitypes.OfferSnapshotResponse{
			Result: cometabcitypes.OFFER_SNAPSHOT_RESULT_REJECT,
		}, nil
	}
	if metadata.Version > snapshot.Version {
		log.Debugw("reject snapshot due to unsupported version",
			"height", req.Snapshot.Height, "version", metadata.Version, "chunks", req.Snapshot.Chunks)
		return &cometabcitypes.OfferSnapshotResponse{
			Result: cometabcitypes.OFFER_SNAPSHOT_RESULT_REJECT,
		}, nil
	}

	snapshotFromComet.height.Store(int64(req.Snapshot.Height))
	snapshotFromComet.chunks.Store(int32(req.Snapshot.Chunks))

	return &cometabcitypes.OfferSnapshotResponse{
		Result: cometabcitypes.OFFER_SNAPSHOT_RESULT_ACCEPT,
	}, nil
}

// LoadSnapshotChunk provides cometbft with a snapshot chunk, during StateSync.
//
// cometbft will reject a len(Chunk) > 16M
func (app *BaseApplication) LoadSnapshotChunk(_ context.Context,
	req *cometabcitypes.LoadSnapshotChunkRequest,
) (*cometabcitypes.LoadSnapshotChunkResponse, error) {
	log.Debugw("cometbft requests a chunk from our snapshot",
		"height", req.Height, "format", req.Format, "chunk", req.Chunk)

	buf, err := app.Snapshots.SliceChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		return &cometabcitypes.LoadSnapshotChunkResponse{}, err
	}

	return &cometabcitypes.LoadSnapshotChunkResponse{
		Chunk: buf,
	}, nil
}

// ApplySnapshotChunk saves to disk a snapshot chunk provided by cometbft StateSync.
// When all chunks of a Snapshot are in disk, the Snapshot is restored.
//
// cometbft will never pass a Chunk bigger than 16M
func (app *BaseApplication) ApplySnapshotChunk(_ context.Context,
	req *cometabcitypes.ApplySnapshotChunkRequest,
) (*cometabcitypes.ApplySnapshotChunkResponse, error) {
	log.Debugw("cometbft provides us a chunk",
		"index", req.Index, "size", len(req.Chunk))

	if err := app.Snapshots.WriteChunkToDisk(req.Index, req.Chunk); err != nil {
		return &cometabcitypes.ApplySnapshotChunkResponse{
			Result: cometabcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_RETRY,
		}, nil
	}

	if app.Snapshots.CountChunksInDisk() == int(snapshotFromComet.chunks.Load()) {
		// if we got here, all chunks are on disk
		s, err := app.Snapshots.JoinChunks(snapshotFromComet.chunks.Load(), snapshotFromComet.height.Load())
		if err != nil {
			log.Error(err)
			return &cometabcitypes.ApplySnapshotChunkResponse{
				Result: cometabcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_REJECT_SNAPSHOT,
			}, nil
		}
		defer func() {
			if err := s.Close(); err != nil {
				log.Error(err)
			}
		}()

		if err := app.RestoreStateFromSnapshot(s); err != nil {
			log.Error(err)
			return &cometabcitypes.ApplySnapshotChunkResponse{
				Result: cometabcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ABORT,
			}, nil
		}

		// Fetch the block before the snapshot, we'll need it to bootstrap other nodes
		fetchAndSaveBlock := func(height int64) error {
			cli, err := newCometRPCClient(app.Node.Config().StateSync.RPCServers[0])
			if err != nil {
				return fmt.Errorf("cannot connect to remote RPC server: %w", err)
			}

			b, err := cli.Block(context.TODO(), &height)
			if err != nil {
				return fmt.Errorf("cannot fetch block %d from remote RPC: %w", height, err)
			}

			blockParts, err := b.Block.MakePartSet(comettypes.BlockPartSizeBytes)
			if err != nil {
				return err
			}

			c, err := cli.Commit(context.TODO(), &height)
			if err != nil {
				return fmt.Errorf("cannot fetch commit %d from remote RPC: %w", height, err)
			}
			app.Node.BlockStore().SaveBlock(b.Block, blockParts, c.Commit)

			return nil
		}

		if err := fetchAndSaveBlock(snapshotFromComet.height.Load()); err != nil {
			log.Error("couldn't fetch and save block, rejecting snapshot: ", err)
			return &cometabcitypes.ApplySnapshotChunkResponse{
				Result: cometabcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_REJECT_SNAPSHOT,
			}, nil
		}
	}

	return &cometabcitypes.ApplySnapshotChunkResponse{
		Result: cometabcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ACCEPT,
	}, nil
}

func (app *BaseApplication) RestoreStateFromSnapshot(snap *snapshot.Snapshot) error {
	tmpDir, err := snap.Restore(app.dbType, app.dataDir)
	if err != nil {
		log.Error(err)
		return err
	}

	// some components (Indexer, OffChainDataHandler) are initialized before statesync,
	// and their EventListeners are added to the old state. So we need to copy them to the new state
	oldEventListeners := app.State.EventListeners()

	if err := app.State.Close(); err != nil {
		return fmt.Errorf("cannot close old state: %w", err)
	}

	if err := os.RemoveAll(filepath.Join(app.dataDir, StateDataDir)); err != nil {
		return fmt.Errorf("error removing existing dataDir: %w", err)
	}

	if err := os.Rename(tmpDir, filepath.Join(app.dataDir, StateDataDir)); err != nil {
		return fmt.Errorf("error moving newDataDir into dataDir: %w", err)
	}

	// TODO: dedup this from vochain/app.go NewBaseApplication
	newState, err := state.New(app.dbType, filepath.Join(app.dataDir, StateDataDir))
	if err != nil {
		return fmt.Errorf("cannot open new state: %w", err)
	}

	// we also need to SetChainID again (since app.SetChainID propagated chainID to the old state)
	newState.SetChainID(app.ChainID())

	newState.CleanEventListeners()
	for _, l := range oldEventListeners {
		newState.AddEventListener(l)
	}

	istc := ist.NewISTC(newState)
	// Create the transaction handler for checking and processing transactions
	transactionHandler := transaction.NewTransactionHandler(
		newState,
		istc,
		filepath.Join(app.dataDir, TxHandlerDataDir),
	)

	// This looks racy but actually it's OK, since State Sync happens during very early init,
	// when the app is blocked waiting for cometbft to finish startup.
	app.State = newState
	app.Istc = istc
	app.TransactionHandler = transactionHandler
	return nil
}

// Query does nothing
func (*BaseApplication) Query(_ context.Context,
	_ *cometabcitypes.QueryRequest) (*cometabcitypes.QueryResponse, error) {
	return &cometabcitypes.QueryResponse{}, nil
}

// ExtendVote creates application specific vote extension
func (*BaseApplication) ExtendVote(_ context.Context,
	req *cometabcitypes.ExtendVoteRequest) (*cometabcitypes.ExtendVoteResponse, error) {
	return &cometabcitypes.ExtendVoteResponse{}, nil
}

// VerifyVoteExtension verifies application's vote extension data
func (*BaseApplication) VerifyVoteExtension(context.Context,
	*cometabcitypes.VerifyVoteExtensionRequest) (*cometabcitypes.VerifyVoteExtensionResponse, error) {
	return &cometabcitypes.VerifyVoteExtensionResponse{}, nil
}
