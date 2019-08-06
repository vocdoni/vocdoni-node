package types

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
)

// Context is an immutable object containing all the info needed to process a request
// Do not over-use the internal Context, trying to keep all data structured and standard additions here
// would be better than to add to the Context struct
type Context struct {
	ctx        context.Context
	store      Store
	header     abci.Header
	chainID    string
	txBytes    []byte
	checkTx    bool
	consParams *abci.ConsensusParams
}

// Context returns Context the internal context
func (c Context) Context() context.Context { return c.ctx }

// Store returns the Context store
func (c Context) Store() Store { return c.store }

// BlockHeight returns the Context block height
func (c Context) BlockHeight() int64 { return c.header.Height }

// BlockTime returns the Context block time
func (c Context) BlockTime() time.Time { return c.header.Time }

// ChainID return the Context chainId
func (c Context) ChainID() string { return c.chainID }

// TxBytes returns the Context TxBytes
func (c Context) TxBytes() []byte { return c.txBytes }

// IsCheckTx returns the Context checkTx boolean
func (c Context) IsCheckTx() bool { return c.checkTx }

// BlockHeader clones the header before returning
func (c Context) BlockHeader() abci.Header {
	var msg = proto.Clone(&c.header).(*abci.Header)
	return *msg
}

// ConsensusParams clones the consensus params before returning
func (c Context) ConsensusParams() *abci.ConsensusParams {
	return proto.Clone(c.consParams).(*abci.ConsensusParams)
}

// NewContext creates a new context
func NewContext(store Store, header abci.Header, isCheckTx bool) Context {
	// https://github.com/gogo/protobuf/issues/519
	header.Time = header.Time.UTC()
	return Context{
		store:   store,
		ctx:     context.Background(),
		header:  header,
		chainID: header.ChainID,
		checkTx: isCheckTx,
	}
}
