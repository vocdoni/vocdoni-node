package vochain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	crypto256k1 "github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/proto/tendermint/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/ist"
	vstate "go.vocdoni.io/dvote/vochain/state"
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
	req *abcitypes.RequestInfo) (*abcitypes.ResponseInfo, error) {
	app.isSynchronizing.Store(true)
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
		req.P2PVersion, "blockVersion", req.BlockVersion, "lastHeight",
		lastHeight, "appHash", hex.EncodeToString(appHash))

	return &abcitypes.ResponseInfo{
		LastBlockHeight:  int64(lastHeight),
		LastBlockAppHash: appHash,
	}, nil
}

// InitChain called once upon genesis
// ResponseInitChain can return a list of validators. If the list is empty,
// Tendermint will use the validators loaded in the genesis file.
func (app *BaseApplication) InitChain(_ context.Context,
	req *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
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
			if err != vstate.ErrAccountAlreadyExists {
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
	tendermintValidators := []abcitypes.ValidatorUpdate{}
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
			if err != vstate.ErrAccountAlreadyExists {
				return nil, fmt.Errorf("cannot create validator acount %x: %w", addr, err)
			}
		}
		tendermintValidators = append(tendermintValidators,
			abcitypes.UpdateValidator(
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
	if err := app.State.SetAccount(vstate.BurnAddress, &vstate.Account{}); err != nil {
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

	// commit state and get hash
	hash, err := app.State.PrepareCommit()
	if err != nil {
		return nil, fmt.Errorf("cannot prepare commit: %w", err)
	}
	if _, err = app.State.Save(); err != nil {
		return nil, fmt.Errorf("cannot save state: %w", err)
	}
	return &abcitypes.ResponseInitChain{
		Validators: tendermintValidators,
		AppHash:    hash,
	}, nil
}

// CheckTx unmarshals req.Tx and checks its validity
func (app *BaseApplication) CheckTx(_ context.Context,
	req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	if req == nil || req.Tx == nil {
		return &abcitypes.ResponseCheckTx{
			Code: 1,
			Data: []byte("nil request or tx"),
		}, fmt.Errorf("nil request or tx")
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
			return &abcitypes.ResponseCheckTx{Code: 1, Data: []byte(fmt.Sprintf("tx expired %x", txReference))}, nil
		}
	}
	// execute recheck mempool every recheckTxHeightInterval blocks
	if req.Type == abcitypes.CheckTxType_Recheck {
		if app.Height()%recheckTxHeightInterval != 0 {
			return &abcitypes.ResponseCheckTx{Code: 0}, nil
		}
	}
	// unmarshal tx and check it
	tx := new(vochaintx.Tx)
	if err := tx.Unmarshal(req.Tx, app.ChainID()); err != nil {
		return &abcitypes.ResponseCheckTx{Code: 1, Data: []byte("unmarshalTx " + err.Error())}, err
	}
	response, err := app.TransactionHandler.CheckTx(tx, false)
	if err != nil {
		if errors.Is(err, transaction.ErrorAlreadyExistInCache) {
			return &abcitypes.ResponseCheckTx{Code: 0}, nil
		}
		log.Errorw(err, "checkTx")
		return &abcitypes.ResponseCheckTx{Code: 1, Data: []byte("checkTx " + err.Error())}, err
	}
	return &abcitypes.ResponseCheckTx{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}, nil
}

// FinalizeBlock is executed by cometBFT when a new block is decided.
// Cryptographic commitments to the block and transaction results, returned via the corresponding
// parameters in ResponseFinalizeBlock, are included in the header of the next block.
func (app *BaseApplication) FinalizeBlock(_ context.Context,
	req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
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

	txResults := make([]*abcitypes.ExecTxResult, len(req.Txs))
	for i, tx := range resp {
		txResults[i] = &abcitypes.ExecTxResult{
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
		if idFlag := v.GetBlockIdFlag(); idFlag == types.BlockIDFlagAbsent || idFlag == types.BlockIDFlagUnknown {
			// skip invalid votes
			continue
		}
		proposalVotes = append(proposalVotes, v.GetValidator().Address)
	}
	if err := app.Istc.Schedule(height+1, []byte(fmt.Sprintf("validators-update-score-%d", height)), ist.Action{
		ID:                ist.ActionUpdateValidatorScore,
		ValidatorVotes:    proposalVotes,
		ValidatorProposer: req.GetProposerAddress(),
	}); err != nil {
		return nil, fmt.Errorf("finalize block: could not schedule IST action: %w", err)
	}

	// update current validators
	validators, err := app.State.Validators(false)
	if err != nil {
		return nil, fmt.Errorf("cannot get validators: %w", err)
	}
	return &abcitypes.ResponseFinalizeBlock{
		AppHash:          root,
		TxResults:        txResults,
		ValidatorUpdates: validatorUpdate(validators),
	}, nil
}

func validatorUpdate(validators map[string]*models.Validator) abcitypes.ValidatorUpdates {
	validatorUpdate := []abcitypes.ValidatorUpdate{}
	for _, v := range validators {
		pubKey := make([]byte, len(v.PubKey))
		copy(pubKey, v.PubKey)
		validatorUpdate = append(validatorUpdate, abcitypes.UpdateValidator(
			pubKey,
			int64(v.Power),
			crypto256k1.KeyType,
		))
	}
	return validatorUpdate
}

// Commit is the CometBFT implementation of the ABCI Commit method. We currently do nothing here.
func (app *BaseApplication) Commit(_ context.Context, _ *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	// save state and get hash
	_, err := app.CommitState()
	if err != nil {
		return nil, err
	}
	return &abcitypes.ResponseCommit{
		RetainHeight: 0, // When snapshot sync enabled, we can start to remove old blocks
	}, nil
}

// PrepareProposal allows the block proposer to perform application-dependent work in a block before
// proposing it. This enables, for instance, batch optimizations to a block, which has been empirically
// demonstrated to be a key component for improved performance. Method PrepareProposal is called every
// time CometBFT is about to broadcast a Proposal message and validValue is nil. CometBFT gathers
// outstanding transactions from the mempool, generates a block header, and uses them to create a block
// to propose. Then, it calls RequestPrepareProposal with the newly created proposal, called raw proposal.
// The Application can make changes to the raw proposal, such as modifying the set of transactions or the
// order in which they appear, and returns the (potentially) modified proposal, called prepared proposal in
// the ResponsePrepareProposal call. The logic modifying the raw proposal MAY be non-deterministic.
func (app *BaseApplication) PrepareProposal(ctx context.Context,
	req *abcitypes.RequestPrepareProposal) (*abcitypes.ResponsePrepareProposal, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	startTime := time.Now()

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
	sort.Slice(validTxInfos, func(i, j int) bool {
		if validTxInfos[i].Addr == nil && validTxInfos[j].Addr != nil {
			return true
		}
		if validTxInfos[i].Addr != nil && validTxInfos[j].Addr == nil {
			return false
		}
		if validTxInfos[i].Addr != nil && validTxInfos[j].Addr != nil {
			if bytes.Equal(validTxInfos[i].Addr.Bytes(), validTxInfos[j].Addr.Bytes()) {
				return validTxInfos[i].Nonce < validTxInfos[j].Nonce
			}
			return bytes.Compare(validTxInfos[i].Addr.Bytes(), validTxInfos[j].Addr.Bytes()) == -1
		}
		return false
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
				}
			}
			continue
		}
		validTxs = append(validTxs, txInfo.Data)
	}

	// Rollback the state to discard the changes made
	app.State.Rollback()
	log.Debugw("prepare proposal", "height", app.Height(), "txs", len(validTxs),
		"milliSeconds", time.Since(startTime).Milliseconds())
	return &abcitypes.ResponsePrepareProposal{
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
	req *abcitypes.RequestProcessProposal) (*abcitypes.ResponseProcessProposal, error) {
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
		return &abcitypes.ResponseProcessProposal{Status: abcitypes.ResponseProcessProposal_ACCEPT}, nil
	}

	startTime := time.Now()

	// TODO: check if we can enable this check without breaking consensus
	//
	// Check if the time difference is within the allowed threshold
	//	timeDiff := startTime.Sub(req.Time)
	//	if timeDiff > allowedConsensusTimeDiff {
	//		return nil, fmt.Errorf("the time difference for the proposal exceeds the threshold")
	//	}

	resp, err := app.ExecuteBlock(req.Txs, uint32(req.GetHeight()), req.GetTime())
	if err != nil {
		return nil, fmt.Errorf("cannot execute block on process proposal: %w", err)
	}
	// invalid txx on a proposed block, should actually never happened if proposer acts honestly
	if len(resp.InvalidTransactions) > 0 {
		log.Warnw("invalid transactions on process proposal", "height", app.Height(), "count", len(resp.InvalidTransactions),
			"proposer", hex.EncodeToString(req.ProposerAddress), "action", "reject")
		return &abcitypes.ResponseProcessProposal{
			Status: abcitypes.ResponseProcessProposal_REJECT,
		}, nil
	}
	app.lastDeliverTxResponse = resp.Responses
	app.lastRootHash = resp.Root
	app.lastBlockHash = req.GetHash()
	log.Debugw("process proposal", "height", app.Height(), "txs", len(req.Txs), "action", "accept",
		"blockHash", hex.EncodeToString(app.lastBlockHash),
		"hash", hex.EncodeToString(resp.Root), "milliSeconds", time.Since(startTime).Milliseconds())
	return &abcitypes.ResponseProcessProposal{
		Status: abcitypes.ResponseProcessProposal_ACCEPT,
	}, nil
}

// ListSnapshots returns a list of available snapshots.
func (*BaseApplication) ListSnapshots(context.Context,
	*abcitypes.RequestListSnapshots) (*abcitypes.ResponseListSnapshots, error) {
	return &abcitypes.ResponseListSnapshots{}, nil
}

// OfferSnapshot returns the response to a snapshot offer.
func (*BaseApplication) OfferSnapshot(context.Context,
	*abcitypes.RequestOfferSnapshot) (*abcitypes.ResponseOfferSnapshot, error) {
	return &abcitypes.ResponseOfferSnapshot{}, nil
}

// LoadSnapshotChunk returns the response to a snapshot chunk loading request.
func (*BaseApplication) LoadSnapshotChunk(context.Context,
	*abcitypes.RequestLoadSnapshotChunk) (*abcitypes.ResponseLoadSnapshotChunk, error) {
	return &abcitypes.ResponseLoadSnapshotChunk{}, nil
}

// ApplySnapshotChunk returns the response to a snapshot chunk applying request.
func (*BaseApplication) ApplySnapshotChunk(context.Context,
	*abcitypes.RequestApplySnapshotChunk) (*abcitypes.ResponseApplySnapshotChunk, error) {
	return &abcitypes.ResponseApplySnapshotChunk{}, nil
}

// Query does nothing
func (*BaseApplication) Query(_ context.Context,
	_ *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	return &abcitypes.ResponseQuery{}, nil
}

// ExtendVote creates application specific vote extension
func (*BaseApplication) ExtendVote(_ context.Context,
	req *abcitypes.RequestExtendVote) (*abcitypes.ResponseExtendVote, error) {
	return &abcitypes.ResponseExtendVote{}, nil
}

// VerifyVoteExtension verifies application's vote extension data
func (*BaseApplication) VerifyVoteExtension(context.Context,
	*abcitypes.RequestVerifyVoteExtension) (*abcitypes.ResponseVerifyVoteExtension, error) {
	return &abcitypes.ResponseVerifyVoteExtension{}, nil
}
