package vochain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	crypto256k1 "github.com/cometbft/cometbft/crypto/secp256k1"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/genesis"
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
	var genesisAppState genesis.GenesisAppState
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
			Address:  genesisAppState.Validators[i].Address,
			PubKey:   genesisAppState.Validators[i].PubKey,
			Power:    genesisAppState.Validators[i].Power,
			KeyIndex: uint32(genesisAppState.Validators[i].KeyIndex),
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
	myValidator, err := app.State.Validator(ethcommon.Address(app.NodeAddress), false)
	if err != nil {
		return nil, fmt.Errorf("cannot get node validator: %w", err)
	}
	if myValidator != nil {
		log.Infow("node is a validator!", "power", myValidator.Power, "name", myValidator.Name)
	}

	// set treasurer address
	if genesisAppState.Treasurer != nil {
		log.Infof("adding genesis treasurer %x", genesisAppState.Treasurer)
		if err := app.State.SetTreasurer(ethcommon.BytesToAddress(genesisAppState.Treasurer), 0); err != nil {
			return nil, fmt.Errorf("could not set State.Treasurer from genesis file: %w", err)
		}
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
	txReference := vochaintx.TxKey(req.Tx)
	// store the initial height of the tx
	initialTTLheight, _ := app.txTTLReferences.LoadOrStore(txReference, app.Height())
	// check if the tx is referenced by a previous block and the TTL has expired
	if app.Height() > initialTTLheight.(uint32)+transactionBlocksTTL {
		// remove tx reference and return checkTx error
		log.Debugw("pruning expired tx from mempool", "height", app.Height(), "hash", fmt.Sprintf("%x", txReference))
		app.txTTLReferences.Delete(txReference)
		return &abcitypes.ResponseCheckTx{Code: 1, Data: []byte(fmt.Sprintf("tx expired %x", txReference))}, nil
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
	start := time.Now()
	height := uint32(req.GetHeight())

	resp, root, err := app.ExecuteBlock(req.Txs, height, req.GetTime())
	if err != nil {
		return nil, fmt.Errorf("cannot execute block: %w", err)
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
	log.Debugw("finalize block", "height", height,
		"txs", len(req.Txs), "hash", hex.EncodeToString(root),
		"totalElapsedSeconds", time.Since(req.GetTime()).Seconds(),
		"executionElapsedSeconds", time.Since(start).Seconds())

	return &abcitypes.ResponseFinalizeBlock{
		AppHash:   root,
		TxResults: txResults,
	}, nil
}

// Commit is the CometBFT implementation of the ABCI Commit method. We currently do nothing here.
func (app *BaseApplication) Commit(_ context.Context, _ *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	// save state and get hash
	h, err := app.CommitState()
	if err != nil {
		return nil, err
	}
	log.Debugw("commit block", "height", app.Height(), "hash", hex.EncodeToString(h))
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
	type txInfo struct {
		Data      []byte
		Addr      *ethcommon.Address
		Nonce     uint32
		DecodedTx *vochaintx.Tx
	}

	// ensure the pending state is clean
	if app.State.TxCounter() > 0 {
		panic("found existing pending transactions on prepare proposal")
	}

	validTxInfos := []txInfo{}
	for _, tx := range req.GetTxs() {
		vtx := new(vochaintx.Tx)
		if err := vtx.Unmarshal(tx, app.ChainID()); err != nil {
			// invalid transaction
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
			if validTxInfos[i].Addr.String() == validTxInfos[j].Addr.String() {
				return validTxInfos[i].Nonce < validTxInfos[j].Nonce
			}
			return validTxInfos[i].Addr.String() < validTxInfos[j].Addr.String()
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
			continue
		}
		validTxs = append(validTxs, txInfo.Data)
	}
	// Rollback the state to discard the changes made by CheckTx
	app.State.Rollback()
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
	// Check if the node is a validator, if not, just accept the proposal and return (nothing to say)
	validator, err := app.State.Validator(app.NodeAddress, true)
	if err != nil {
		return nil, fmt.Errorf("cannot get node validator info: %w", err)
	}
	if validator == nil {
		return &abcitypes.ResponseProcessProposal{Status: abcitypes.ResponseProcessProposal_ACCEPT}, nil
	}
	app.prepareProposalLock.Lock()
	defer app.prepareProposalLock.Unlock()
	// ensure the pending state is clean
	if app.State.TxCounter() > 0 {
		panic("found existing pending transactions on process proposal")
	}

	valid := true
	for _, tx := range req.Txs {
		vtx := new(vochaintx.Tx)
		if err := vtx.Unmarshal(tx, app.ChainID()); err != nil {
			// invalid transaction
			log.Warnw("could not unmarshal transaction", "err", err)
			valid = false
			break
		}
		// Check the validity of the transaction using forCommit true
		resp, err := app.TransactionHandler.CheckTx(vtx, true)
		if err != nil {
			log.Warnw("discard invalid tx on process proposal",
				"err", err,
				"data", func() string {
					if resp != nil {
						return string(resp.Data)
					}
					return ""
				}())
			valid = false
			break
		}
	}
	// Rollback the state to discard the changes made by CheckTx
	app.State.Rollback()

	if !valid {
		return &abcitypes.ResponseProcessProposal{
			Status: abcitypes.ResponseProcessProposal_REJECT,
		}, nil
	}
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
func (*BaseApplication) ExtendVote(context.Context,
	*abcitypes.RequestExtendVote) (*abcitypes.ResponseExtendVote, error) {
	return &abcitypes.ResponseExtendVote{}, nil
}

// VerifyVoteExtension verifies application's vote extension data
func (*BaseApplication) VerifyVoteExtension(context.Context,
	*abcitypes.RequestVerifyVoteExtension) (*abcitypes.ResponseVerifyVoteExtension, error) {
	return &abcitypes.ResponseVerifyVoteExtension{}, nil
}
