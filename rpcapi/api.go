package rpcapi

import (
	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/jsonrpcapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	census "go.vocdoni.io/dvote/rpccensus"
	api "go.vocdoni.io/dvote/rpctypes"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

const MaxListSize = 64

type Handler = func(*api.APIrequest) (*api.APIresponse, error)

type RPCAPI struct {
	PrivateCalls uint64
	PublicCalls  uint64
	APIs         []string

	router       *httprouter.HTTProuter
	rpcAPI       *jsonrpcapi.SignedJRPC
	indexer      *indexer.Indexer
	methods      map[string]Handler
	storage      data.Storage
	signer       *ethereum.SignKeys
	census       *census.Manager
	vocapp       *vochain.BaseApplication
	metricsagent *metrics.Agent
	vocinfo      *vochaininfo.VochainInfo
	allowPrivate bool
}

func NewAPI(signer *ethereum.SignKeys, router *httprouter.HTTProuter, endpoint string,
	metricsagent *metrics.Agent, allowPrivate bool) (*RPCAPI, error) {
	cm := new(census.Manager)
	api := new(RPCAPI)
	api.census = cm
	api.signer = signer
	api.metricsagent = metricsagent
	api.allowPrivate = allowPrivate
	api.methods = make(map[string]Handler, 128)
	api.rpcAPI = jsonrpcapi.NewSignedJRPC(signer, NewApiRequest, NewApiResponse, allowPrivate)
	api.router = router
	router.AddNamespace("rpcAPI", api.rpcAPI)

	api.RegisterPublic("getInfo", false, api.info)
	if metricsagent != nil {
		api.registerMetrics(metricsagent)
	}

	router.AddPrivateHandler("rpcAPI", endpoint, "POST", api.route)

	return api, nil
}

func (a *RPCAPI) RegisterPrivate(method string, h Handler) {
	a.rpcAPI.RegisterMethod(method, true, false)
	a.methods[method] = h
}

func (a *RPCAPI) RegisterPublic(method string, requireSignature bool, h Handler) {
	a.rpcAPI.RegisterMethod(method, false, !requireSignature)
	a.methods[method] = h
}

func (a *RPCAPI) SetIndexer(sc *indexer.Indexer) {
	a.indexer = sc
}

func (a *RPCAPI) SetVocdoniApp(app *vochain.BaseApplication) {
	a.vocapp = app
}

func (a *RPCAPI) SetVocdoniInfo(info *vochaininfo.VochainInfo) {
	a.vocinfo = info
}

func (a *RPCAPI) SetStorage(stg data.Storage) {
	a.storage = stg
}

func (a *RPCAPI) AuthorizedAddress(addr *common.Address) bool {
	if addr == nil {
		return false
	}
	if !a.allowPrivate {
		return false
	}
	// Warning: if allowPrivate is true but no authorized addresses,
	// we allow any address
	if len(a.signer.Authorized) == 0 {
		return true
	}
	return a.signer.Authorized[*addr]
}

func (a *RPCAPI) route(msg httprouter.Message) {
	request := msg.Data.(*jsonrpcapi.SignedJRPCdata)
	apiMsg := request.Message.(*api.APIrequest)
	apiMsg.SetAddress(&request.Address)
	method := a.methods[apiMsg.GetMethod()]
	apiMsgResponse, err := method(apiMsg)
	if err != nil {
		a.rpcAPI.SendError(
			request.ID,
			err.Error(),
			msg.Context,
		)
		return
	}
	apiMsgResponse.Ok = true
	data, err := jsonrpcapi.BuildReply(a.signer, apiMsgResponse, request.ID)
	if err != nil {
		log.Errorf("cannot build reply for method %s: %v", apiMsg.GetMethod(), err)
		return
	}
	if err := msg.Context.Send(data, 200); err != nil {
		log.Warnf("cannot send api response: %v", err)
	}
}

func NewApiRequest() jsonrpcapi.MessageAPI {
	return &api.APIrequest{}
}

func NewApiResponse() jsonrpcapi.MessageAPI {
	return &api.APIresponse{}
}
