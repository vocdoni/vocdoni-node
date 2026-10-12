package vochain

import (
	"context"
	"errors"
	"fmt"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/blocktypes"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// CheckTxRequest is the input of CheckTx.
type CheckTxRequest struct {
	Tx []byte
}

// CheckTxResponse is the result of CheckTx. A non-zero Code means the transaction is not valid.
type CheckTxResponse struct {
	Code uint32
	Data []byte
	Log  string
	Info string
}

// SendTxResponse is the result of sending a transaction to the mempool.
// A non-zero Code means the transaction was rejected.
type SendTxResponse struct {
	Code uint32
	Data blocktypes.HexBytes
	Log  string
	Hash blocktypes.HexBytes
}

// InitChain initializes the state of a new chain from the genesis app state.
// It must only be called on an empty state, and returns the resulting state root.
func (app *BaseApplication) InitChain(appState *genesis.AppState, genesisTime time.Time) ([]byte, error) {
	lastHeight, err := app.State.LastHeight()
	if err != nil {
		return nil, fmt.Errorf("cannot get State.LastHeight: %w", err)
	}
	if lastHeight > 0 {
		return nil, fmt.Errorf("cannot init chain: state already initialized at height %d", lastHeight)
	}

	// create accounts
	for _, acc := range appState.Accounts {
		addr := ethcommon.BytesToAddress(acc.Address)
		if err := app.State.CreateAccount(addr, "", nil, acc.Balance); err != nil {
			if !errors.Is(err, state.ErrAccountAlreadyExists) {
				return nil, fmt.Errorf("cannot create acount %x: %w", addr, err)
			}
			if err := app.State.InitChainMintBalance(addr, acc.Balance); err != nil {
				return nil, fmt.Errorf("cannot initialize chain minting balance: %w", err)
			}
		}
		log.Infow("created account", "addr", addr.Hex(), "tokens", acc.Balance)
	}

	// add validators, which hold the keykeeper key indexes
	for _, gv := range appState.Validators {
		log.Infow("add genesis validator",
			"signingAddress", gv.Address.String(),
			"power", gv.Power,
			"name", gv.Name,
			"keyIndex", gv.KeyIndex,
		)
		v := &models.Validator{
			Address:  gv.Address,
			PubKey:   gv.PubKey,
			Power:    gv.Power,
			KeyIndex: uint32(gv.KeyIndex),
			Name:     gv.Name,
		}
		if err := app.State.AddValidator(v); err != nil {
			return nil, fmt.Errorf("cannot add validator %s: %w", log.FormatProto(v), err)
		}
		addr := ethcommon.BytesToAddress(v.Address)
		if err := app.State.CreateAccount(addr, "", nil, 0); err != nil {
			if !errors.Is(err, state.ErrAccountAlreadyExists) {
				return nil, fmt.Errorf("cannot create validator acount %x: %w", addr, err)
			}
		}
	}

	// add tx costs
	for k, v := range appState.TxCost.AsMap() {
		if err := app.State.SetTxBaseCost(k, v); err != nil {
			return nil, fmt.Errorf("could not set tx cost %q to value %d from genesis file to the State", k, v)
		}
	}

	// create burn account
	if err := app.State.SetAccount(state.BurnAddress, &state.Account{}); err != nil {
		return nil, fmt.Errorf("unable to set burn address")
	}

	// set max election size
	if err := app.State.SetMaxProcessSize(appState.MaxElectionSize); err != nil {
		return nil, fmt.Errorf("unable to set max election size")
	}

	// set network capacity
	if err := app.State.SetNetworkCapacity(appState.NetworkCapacity); err != nil {
		return nil, fmt.Errorf("unable to set network capacity")
	}

	// initialize election price calc
	if err := app.State.SetElectionPriceCalc(); err != nil {
		return nil, fmt.Errorf("cannot set election price calc: %w", err)
	}

	// set initial timestamp
	if err := app.State.SetTimestamp(uint32(genesisTime.Unix())); err != nil {
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
	return hash, nil
}

// CheckTx unmarshals req.Tx and checks its validity against the current state,
// without applying it.
func (app *BaseApplication) CheckTx(_ context.Context, req *CheckTxRequest) (*CheckTxResponse, error) {
	if req == nil || req.Tx == nil {
		return &CheckTxResponse{
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
			return &CheckTxResponse{Code: 1, Data: []byte(fmt.Sprintf("tx expired %x", txReference))}, nil
		}
	}
	// unmarshal tx and check it
	tx := new(vochaintx.Tx)
	if err := tx.Unmarshal(req.Tx, app.ChainID()); err != nil {
		return &CheckTxResponse{Code: 1, Data: []byte(err.Error()), Log: err.Error()}, nil
	}
	response, err := app.TransactionHandler.CheckTx(tx, false)
	if err != nil {
		if errors.Is(err, transaction.ErrorAlreadyExistInCache) {
			return &CheckTxResponse{Code: 0}, nil
		}
		log.Errorw(err, "checkTx")
		return &CheckTxResponse{Code: 1, Data: []byte(err.Error()), Log: err.Error()}, nil
	}
	return &CheckTxResponse{
		Code: 0,
		Data: response.Data,
		Info: fmt.Sprintf("%x", response.TxHash),
		Log:  response.Log,
	}, nil
}
