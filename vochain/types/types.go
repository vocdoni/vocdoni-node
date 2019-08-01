package vochain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-cmn/db"
)

// GlobalState represents the Tendermint blockchain global state
type GlobalState struct {
	ProcessList        *ProcessState     `json:"processList"`
	ValidatorsPubK     []tmtypes.Address `json:"minerspubk"`
	CensusManagersPubK []tmtypes.Address `json:"censusmanagerpubk"`
}

// ProcessState represents a state per process
type ProcessState struct {
	ID             string              `json:"id"`
	MkRoot         string              `json:"mkroot"`
	InitBlock      tmtypes.Block       `json:"initblock"`
	EndBlock       tmtypes.Block       `json:"endblock"`
	EncryptionKeys []tmtypes.Address   `json:"encryptionkeys"`
	CurrentState   CurrentProcessState `json:"currentstate"`
	Votes          []Vote              `json:"votes"`
}

// Vote represents a single vote
type Vote struct {
	Nullifier   string `json:"nullifier"`
	Payload     string `json:"payload"`
	CensusProof string `json:"censusproof"`
	VoteProof   string `json:"voteproof"`
}

// CurrentProcessState represents the current phase of process state
type CurrentProcessState string

const (
	processScheduled  CurrentProcessState = "processScheduled"
	processInProgress CurrentProcessState = "processInProgress"
	processPaused     CurrentProcessState = "processPaused"
	processResumed    CurrentProcessState = "processResumed"
	rocessFinished    CurrentProcessState = "processFinished"
)

// String returns the CurrentProcessState as string
func (c CurrentProcessState) String() string {
	return fmt.Sprintf("%s", c)
}

type TxMethod string

const (
	NewProcessTx          TxMethod = "newProcessTx"
	VoteTx                TxMethod = "voteTx"
	AddCensusManagerTx    TxMethod = "addCensusManagerTx"
	RemoveCensusManagerTx TxMethod = "removeCensusManagerTx"
	AddValidatorTx        TxMethod = "addValidatorTx"
	RemoveValidatorTx     TxMethod = "removeValidatorTx"
	GetProcessState       TxMethod = "getProcessState"
)

// String returns the CurrentProcessState as string
func (m TxMethod) String() string {
	return fmt.Sprintf("%s", m)
}

type Tx interface {
	GetMethod() string
	GetArgs() []string
}

type ValidTx struct {
	Method TxMethod `json:"method"`
	Args   []string `json:"args"`
}

func (tx ValidTx) String() string {
	var buffer bytes.Buffer
	for _, i := range tx.Args {
		buffer.WriteString(i)
	}
	return fmt.Sprintf(`{ "method": %s, args: %s }`, tx.Method.String(), buffer.String())
}

func (tx ValidTx) GetMethod() (m TxMethod, err error) {
	validmethod := tx.validateMethod()
	if validmethod {
		return tx.Method, nil
	}
	return "", errors.New("INVALID METHOD")
}

func (tx ValidTx) GetArgs() []string {
	return tx.Args
}

func (tx ValidTx) validateMethod() bool {
	m := tx.Method
	if m == NewProcessTx || m == VoteTx || m == AddCensusManagerTx || m == RemoveCensusManagerTx || m == AddValidatorTx || m == RemoveValidatorTx || m == GetProcessState {
		return true
	}
	return false
}

// ______________________ CONTEXT ______________________

// Context is an immutable object containing all the info needed to process a request
type Context struct {
	ctx        context.Context
	header     abci.Header
	chainID    string
	txBytes    []byte
	checkTx    bool
	voteInfo   []abci.VoteInfo
	consParams *abci.ConsensusParams
}

// Read-only accessors
func (c Context) Context() context.Context   { return c.ctx }
func (c Context) BlockHeight() int64         { return c.header.Height }
func (c Context) BlockTime() time.Time       { return c.header.Time }
func (c Context) ChainID() string            { return c.chainID }
func (c Context) TxBytes() []byte            { return c.txBytes }
func (c Context) VoteInfos() []abci.VoteInfo { return c.voteInfo }
func (c Context) IsCheckTx() bool            { return c.checkTx }

// clone the header before returning
func (c Context) BlockHeader() abci.Header {
	var msg = proto.Clone(&c.header).(*abci.Header)
	return *msg
}

func (c Context) ConsensusParams() *abci.ConsensusParams {
	return proto.Clone(c.consParams).(*abci.ConsensusParams)
}

// create a new context
func NewContext(header abci.Header, isCheckTx bool) Context {
	// https://github.com/gogo/protobuf/issues/519
	header.Time = header.Time.UTC()
	return Context{
		ctx:     context.Background(),
		header:  header,
		chainID: header.ChainID,
		checkTx: isCheckTx,
	}
}

// ______________________ STORE ______________________

type Store struct {
	db dbm.DB
}

func NewStore(db dbm.DB) *Store {
	return &Store{
		db: db,
	}
}

// ______________________ STATE ______________________

type State struct {
	store Store
	ctx   Context
}

func (st *State) Context() Context {
	return st.ctx
}

func (st *State) Store() Store {
	return st.store
}
