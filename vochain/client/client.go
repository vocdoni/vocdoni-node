package vochaincli

import (
	"sync"

	"github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

// Client defines an interface for an ABCI client.
// All `Async` methods return a `ReqRes` object.
// All `Sync` methods return the appropriate protobuf ResponseXxx struct and an error.
// Note these are client errors, eg. ABCI socket connectivity issues.
// Application-related errors are reflected in response via ABCI error codes and logs.
type Client interface {
	cmn.Service

	SetResponseCallback(Callback)
	Error() error

	DeliverTxAsync(types.RequestDeliverTx) *ReqRes
	CheckTxAsync(types.RequestCheckTx) *ReqRes

	FlushSync() error
	EchoSync(msg string) (*types.ResponseEcho, error)
	InfoSync(types.RequestInfo) (*types.ResponseInfo, error)
	SetOptionSync(types.RequestSetOption) (*types.ResponseSetOption, error)
	QuerySync(types.RequestQuery) (*types.ResponseQuery, error)
	CommitSync() (*types.ResponseCommit, error)
	InitChainSync(types.RequestInitChain) (*types.ResponseInitChain, error)
	BeginBlockSync(types.RequestBeginBlock) (*types.ResponseBeginBlock, error)
	EndBlockSync(types.RequestEndBlock) (*types.ResponseEndBlock, error)
}

//----------------------------------------

type Callback func(*types.Request, *types.Response)

//----------------------------------------

type ReqRes struct {
	*types.Request
	*sync.WaitGroup
	*types.Response // Not set atomically, so be sure to use WaitGroup.

	mtx  sync.Mutex
	done bool                  // Gets set to true once *after* WaitGroup.Done().
	cb   func(*types.Response) // A single callback that may be set.
}

func NewReqRes(req *types.Request) *ReqRes {
	return &ReqRes{
		Request:   req,
		WaitGroup: waitGroup1(),
		Response:  nil,

		done: false,
		cb:   nil,
	}
}

// Sets the callback for this ReqRes atomically.
// If reqRes is already done, calls cb immediately.
// NOTE: reqRes.cb should not change if reqRes.done.
// NOTE: only one callback is supported.
func (reqRes *ReqRes) SetCallback(cb func(res *types.Response)) {
	reqRes.mtx.Lock()

	if reqRes.done {
		reqRes.mtx.Unlock()
		cb(reqRes.Response)
		return
	}

	reqRes.cb = cb
	reqRes.mtx.Unlock()
}

func (reqRes *ReqRes) GetCallback() func(*types.Response) {
	reqRes.mtx.Lock()
	defer reqRes.mtx.Unlock()
	return reqRes.cb
}

// NOTE: it should be safe to read reqRes.cb without locks after this.
func (reqRes *ReqRes) SetDone() {
	reqRes.mtx.Lock()
	reqRes.done = true
	reqRes.mtx.Unlock()
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
