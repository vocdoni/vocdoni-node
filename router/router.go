// Package router provides the routing and entry point for the go-dvote API
package router

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	psload "github.com/shirou/gopsutil/load"
	psmem "github.com/shirou/gopsutil/mem"
	psnet "github.com/shirou/gopsutil/net"
	amino "github.com/tendermint/go-amino"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/crypto"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
)

const (
	healthMemMax   = 100
	healthLoadMax  = 10
	healthSocksMax = 10000
)

func (r *Router) buildReply(request routerRequest, resp *types.MetaResponse) types.Message {
	// Add any last fields to the inner response, and marshal it with sorted
	// fields for signing.
	resp.Ok = true
	resp.Request = request.id
	resp.Timestamp = int32(time.Now().Unix())
	respInner, err := crypto.SortedMarshalJSON(resp)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      []byte(err.Error()),
		}
	}

	// Sign the marshaled inner response.
	signature, err := r.signer.Sign(respInner)
	if err != nil {
		log.Error(err)
		// continue without the signature
	}

	// Build the outer response with the already-marshaled inner response
	// and its signature.
	respOuter := types.ResponseMessage{
		ID:           request.id,
		Signature:    signature,
		MetaResponse: respInner,
	}
	// We don't need to use crypto.SortedMarshalJSON here, since we don't
	// sign these bytes.
	respData, err := json.Marshal(respOuter)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      []byte(err.Error()),
		}
	}
	log.Debugf("api response %s", respOuter.MetaResponse)
	return types.Message{
		TimeStamp: int32(time.Now().Unix()),
		Context:   request.MessageContext,
		Data:      respData,
	}
}

func parseTransportFromURI(uris []string) []string {
	out := make([]string, 0)
	for _, u := range uris {
		splt := strings.Split(u, "/")
		out = append(out, splt[0])
	}
	return out
}

type registeredMethod struct {
	public  bool
	handler func(routerRequest)
}

// Router holds a router object
type Router struct {
	methods      map[string]registeredMethod
	inbound      <-chan types.Message
	storage      data.Storage
	signer       *ethereum.SignKeys
	census       *census.Manager
	vocapp       *vochain.BaseApplication
	metricsagent *metrics.Agent
	vocinfo      *vochaininfo.VochainInfo
	allowPrivate bool
	Scrutinizer  *scrutinizer.Scrutinizer
	PrivateCalls uint64
	PublicCalls  uint64
	codec        *amino.Codec
	APIs         []string
}

func NewRouter(inbound <-chan types.Message, storage data.Storage,
	signer *ethereum.SignKeys, metricsagent *metrics.Agent, allowPrivate bool) *Router {
	cm := new(census.Manager)
	r := new(Router)
	r.methods = make(map[string]registeredMethod)
	r.census = cm
	r.inbound = inbound
	r.storage = storage
	r.signer = signer
	r.codec = amino.NewCodec()
	r.metricsagent = metricsagent
	r.allowPrivate = allowPrivate
	r.registerPublic("getGatewayInfo", r.info)
	if metricsagent != nil {
		r.registerMetrics(metricsagent)
	}
	return r
}

type routerRequest struct {
	types.MetaRequest
	types.MessageContext

	method        string
	id            string
	authenticated bool
	address       ethcommon.Address
	private       bool
}

// semi-unmarshalls message, returns method name
func (r *Router) getRequest(payload []byte, context types.MessageContext) (request routerRequest, err error) {
	// First unmarshal the outer layer, to obtain the request ID, the signed
	// request, and the signature.
	var reqOuter types.RequestMessage
	if err := json.Unmarshal(payload, &reqOuter); err != nil {
		return request, err
	}
	request.id = reqOuter.ID
	request.MessageContext = context

	var reqInner types.MetaRequest
	if err := json.Unmarshal(reqOuter.MetaRequest, &reqInner); err != nil {
		return request, err
	}
	request.MetaRequest = reqInner
	request.method = reqInner.Method
	if request.method == "" {
		return request, fmt.Errorf("method is empty")
	}

	method, ok := r.methods[request.method]
	if !ok {
		return request, fmt.Errorf("method not valid [%s]", request.method)
	}
	if method.public {
		request.private = false
		request.authenticated = true
		request.address = ethcommon.Address{}
	} else {
		request.private = true
		request.authenticated, request.address, err = r.signer.VerifySender(reqOuter.MetaRequest, reqOuter.Signature)
		// if no authrized keys, authenticate all requests if allowPrivate=true
		if r.allowPrivate && !request.authenticated && len(r.signer.Authorized) == 0 {
			request.authenticated = true
		}
	}
	return request, err
}

// InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage,
	signer *ethereum.SignKeys, metricsagent *metrics.Agent, allowPrivate bool) *Router {
	log.Infof("using signer with address %s", signer.AddressString())
	if allowPrivate {
		log.Warn("allowing API private methods")
	}

	return NewRouter(inbound, storage, signer, metricsagent, allowPrivate)
}

func (r *Router) registerPrivate(name string, handler func(routerRequest)) {
	if _, ok := r.methods[name]; ok {
		log.Fatalf("duplicate method: %q", name)
	}
	r.methods[name] = registeredMethod{handler: handler}
}

func (r *Router) registerPublic(name string, handler func(routerRequest)) {
	if _, ok := r.methods[name]; ok {
		log.Fatalf("duplicate method: %q", name)
	}
	r.methods[name] = registeredMethod{public: true, handler: handler}
}

// EnableFileAPI enables the FILE API in the Router
func (r *Router) EnableFileAPI() {
	r.APIs = append(r.APIs, "file")
	r.registerPublic("fetchFile", r.fetchFile)
	r.registerPrivate("addFile", r.addFile)
	r.registerPrivate("pinList", r.pinList)
	r.registerPrivate("pinFile", r.pinFile)
	r.registerPrivate("unpinFile", r.unpinFile)
}

// EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.Manager) {
	r.APIs = append(r.APIs, "census")
	r.census = cm
	if cm.RemoteStorage == nil {
		cm.RemoteStorage = r.storage
	}
	r.registerPublic("getRoot", r.censusLocal)
	r.registerPrivate("dump", r.censusLocal)
	r.registerPrivate("dumpPlain", r.censusLocal)
	r.registerPublic("getSize", r.censusLocal)
	r.registerPublic("genProof", r.censusLocal)
	r.registerPublic("checkProof", r.censusLocal)
	r.registerPrivate("addCensus", r.censusLocal)
	r.registerPrivate("addClaim", r.censusLocal)
	r.registerPrivate("addClaimBulk", r.censusLocal)
	r.registerPrivate("publish", r.censusLocal)
	r.registerPrivate("importRemote", r.censusLocal)
	r.registerPrivate("getCensusList", r.censusLocal)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableVoteAPI(vocapp *vochain.BaseApplication, vocInfo *vochaininfo.VochainInfo) {
	r.APIs = append(r.APIs, "vote")
	r.vocapp = vocapp
	r.vocinfo = vocInfo
	r.registerPrivate("submitRawTx", r.submitRawTx)
	r.registerPublic("submitEnvelope", r.submitEnvelope)
	r.registerPublic("getEnvelopeStatus", r.getEnvelopeStatus)
	r.registerPublic("getEnvelope", r.getEnvelope)
	r.registerPublic("getEnvelopeHeight", r.getEnvelopeHeight)
	r.registerPublic("getProcessList", r.getProcessList)
	r.registerPublic("getEnvelopeList", r.getEnvelopeList)
	r.registerPublic("getBlockHeight", r.getBlockHeight)
	r.registerPublic("getProcessKeys", r.getProcessKeys)
	r.registerPublic("getBlockStatus", r.getBlockStatus)
	r.registerPublic("getProcessCount", r.getProcessCount)
	if r.Scrutinizer != nil {
		r.APIs = append(r.APIs, "results")
		r.registerPublic("getResults", r.getResults)
		r.registerPublic("getProcListResults", r.getProcListResults)
		r.registerPublic("getProcListLiveResults", r.getProcListLiveResults)
		r.registerPublic("getScrutinizerEntities", r.getScrutinizerEntities)
		r.registerPublic("getScrutinizerEntityCount", r.getScrutinizerEntityCount)
	}
}

// Route routes requests through the Router object
func (r *Router) Route() {
	log.Infof("starting router mux")
	if len(r.methods) == 0 {
		log.Warnf("router methods are not properly initialized: %+v", r)
		return
	}
	for {
		msg := <-r.inbound
		request, err := r.getRequest(msg.Data, msg.Context)
		if !request.authenticated && err != nil {
			go r.sendError(request, err.Error())
			continue
		}
		method, ok := r.methods[request.method]
		if !ok {
			errMsg := fmt.Sprintf("router has no method %q", request.method)
			go r.sendError(request, errMsg)
			continue
		}
		if !method.public && !request.authenticated {
			errMsg := fmt.Sprintf("authentication is required for %q", request.method)
			go r.sendError(request, errMsg)
			continue
		}
		log.Debugf("api query %s", request.MetaRequest.String())
		if request.private {
			r.PrivateCalls++
		} else {
			r.PublicCalls++
		}

		if r.metricsagent != nil {
			if request.private {
				RouterPrivateReqs.With(prometheus.Labels{"method": request.method}).Inc()
			} else {
				RouterPublicReqs.With(prometheus.Labels{"method": request.method}).Inc()
			}
		}

		go method.handler(request)
	}
}

func (r *Router) sendError(request routerRequest, errMsg string) {
	log.Warn(errMsg)

	// Add any last fields to the inner response, and marshal it with sorted
	// fields for signing.
	response := types.MetaResponse{
		Request:   request.id,
		Timestamp: int32(time.Now().Unix()),
	}
	response.SetError(errMsg)
	respInner, err := crypto.SortedMarshalJSON(response)
	if err != nil {
		log.Error(err)
		return
	}

	// Sign the marshaled inner response.
	signature, err := r.signer.Sign(respInner)
	if err != nil {
		log.Error(err)
		// continue without the signature
	}

	respOuter := types.ResponseMessage{
		ID:           request.id,
		Signature:    signature,
		MetaResponse: respInner,
	}
	if request.MessageContext != nil {
		data, err := json.Marshal(respOuter)
		if err != nil {
			log.Warnf("error marshaling response body: %s", err)
		}
		msg := types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      data,
		}
		request.Send(msg)
	}
}

func (r *Router) info(request routerRequest) {
	var response types.MetaResponse
	response.APIList = r.APIs
	response.Request = request.id
	if health, err := getHealth(); err == nil {
		response.Health = health
	} else {
		response.Health = -1
		log.Errorf("cannot get health status: (%s)", err)
	}
	request.Send(r.buildReply(request, &response))
}

// Health is a number between 0 and 99 that represents the status of the node, as bigger the better
// The formula ued to calculate health is: 100* (1- ( Sum(weight[0..1] * value/value_max) ))
// Weight is a number between 0 and 1 used to give a specific weight to a value. The sum of all weights used must be equals to 1
//  so 0.2*value1 + 0.8*value2 would give 20% of weight to value1 and 80% of weight to value2
// Each value must be represented as a number between 0 and 1. To this aim the value might be divided by its maximum value
//  so if the mettered value is cpuLoad, a maximum must be defined in order to give a normalized value between 0 and 1
//   i.e cpuLoad=2 and maxCpuLoad=10. The value is: 2/10 (where cpuLoad<10) = 0.2
// The last operation includes the reverse of the values, so 1- (result).
//   And its *100 multiplication and trunking in order to provide a natural number between 0 and 99
func getHealth() (int32, error) {
	v, err := psmem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	memUsed := v.UsedPercent
	l, err := psload.Avg()
	if err != nil {
		return 0, err
	}
	load15 := l.Load15
	n, err := psnet.Connections("tcp")
	if err != nil {
		return 0, err
	}
	sockets := float64(len(n))

	// ensure maximums are not overflow
	if memUsed > healthMemMax {
		memUsed = healthMemMax
	}
	if load15 > healthLoadMax {
		load15 = healthLoadMax
	}
	if sockets > healthSocksMax {
		sockets = healthSocksMax
	}
	result := int32((1 - (0.33*(memUsed/healthMemMax) +
		0.33*(load15/healthLoadMax) +
		0.33*(sockets/healthSocksMax))) * 100)
	if result < 0 || result >= 100 {
		return 0, fmt.Errorf("expected health to be between 0 and 99: %d", result)
	}
	return result, nil
}
