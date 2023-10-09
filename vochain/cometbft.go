package vochain

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	lastHeight, err := app.State.LastHeight()
	if err != nil {
		return nil, fmt.Errorf("cannot get State.LastHeight: %w", err)
	}
	app.State.SetHeight(lastHeight)
	appHash := app.State.WorkingHash()
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
			Address:  genesisAppState.Validators[i].Address,
			PubKey:   genesisAppState.Validators[i].PubKey,
			Power:    genesisAppState.Validators[i].Power,
			KeyIndex: uint32(genesisAppState.Validators[i].KeyIndex),
		}
		if err = app.State.AddValidator(v); err != nil {
			return nil, fmt.Errorf("cannot add validator %s: %w", log.FormatProto(v), err)
		}
		tendermintValidators = append(tendermintValidators,
			abcitypes.UpdateValidator(
				genesisAppState.Validators[i].PubKey,
				int64(genesisAppState.Validators[i].Power),
				crypto256k1.KeyType,
			))
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
	hash, err := app.State.Save()
	if err != nil {
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
	if req.Type == abcitypes.CheckTxType_Recheck {
		if app.Height()%recheckTxHeightInterval != 0 {
			return &abcitypes.ResponseCheckTx{Code: 0}, nil
		}
	}
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

// FinalizeBlock It delivers a decided block to the Application. The Application must execute
// the transactions in the block deterministically and update its state accordingly.
// Cryptographic commitments to the block and transaction results, returned via the corresponding
// parameters in ResponseFinalizeBlock, are included in the header of the next block.
// CometBFT calls it when a new block is decided.
func (app *BaseApplication) FinalizeBlock(_ context.Context,
	req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	height := uint32(req.GetHeight())
	app.beginBlock(req.GetTime(), height)
	txResults := make([]*abcitypes.ExecTxResult, len(req.Txs))
	for i, tx := range req.Txs {
		resp := app.deliverTx(tx)
		if resp.Code != 0 {
			log.Warnw("deliverTx failed",
				"code", resp.Code,
				"data", string(resp.Data),
				"info", resp.Info,
				"log", resp.Log)
		}
		txResults[i] = &abcitypes.ExecTxResult{
			Code: resp.Code,
			Data: resp.Data,
			Log:  resp.Log,
		}
	}
	// execute internal state transition commit
	if err := app.Istc.Commit(height); err != nil {
		return nil, fmt.Errorf("cannot execute ISTC commit: %w", err)
	}
	app.endBlock(req.GetTime(), height)
	return &abcitypes.ResponseFinalizeBlock{
		AppHash:   app.State.WorkingHash(),
		TxResults: txResults, // TODO: check if we can remove this
	}, nil
}

// Commit saves the current vochain state and returns a commit hash
func (app *BaseApplication) Commit(_ context.Context, _ *abcitypes.RequestCommit) (*abcitypes.ResponseCommit, error) {
	// save state
	_, err := app.State.Save()
	if err != nil {
		return nil, fmt.Errorf("cannot save state: %w", err)
	}
	// perform state snapshot (DISABLED)
	if false && app.Height()%50000 == 0 && !app.IsSynchronizing() { // DISABLED
		startTime := time.Now()
		log.Infof("performing a state snapshot on block %d", app.Height())
		if _, err := app.State.Snapshot(); err != nil {
			return nil, fmt.Errorf("cannot make state snapshot: %w", err)
		}
		log.Infof("snapshot created successfully, took %s", time.Since(startTime))
		log.Debugf("%+v", app.State.ListSnapshots())
	}
	if app.State.TxCounter() > 0 {
		log.Infow("commit block", "height", app.Height(), "txs", app.State.TxCounter())
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
	// TODO: Prepare Proposal should check the validity of the transactions for the next block.
	// Currently they are executed by CheckTx, but it does not allow height to be passed in.
	validTxs := [][]byte{}
	for _, tx := range req.GetTxs() {
		resp, err := app.CheckTx(ctx, &abcitypes.RequestCheckTx{
			Tx: tx, Type: abcitypes.CheckTxType_New,
		})
		if err != nil || resp.Code != 0 {
			log.Warnw("discard invalid tx on prepare proposal",
				"err", err,
				"code", resp.Code,
				"data", string(resp.Data),
				"info", resp.Info,
				"log", resp.Log)
			continue
		}
		validTxs = append(validTxs, tx)
	}

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
func (*BaseApplication) ProcessProposal(_ context.Context,
	_ *abcitypes.RequestProcessProposal) (*abcitypes.ResponseProcessProposal, error) {
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
