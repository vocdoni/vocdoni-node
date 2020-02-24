// Package router provides the routing and entry point for the go-dvote API
package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	amino "github.com/tendermint/go-amino"
	voclient "github.com/tendermint/tendermint/rpc/client"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

func (r *Router) buildReply(request routerRequest, response types.ResponseMessage) types.Message {
	response.ID = request.id
	response.Ok = true
	response.Request = request.id
	respData, err := json.Marshal(response)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.context,
			Data:      []byte(err.Error()),
		}
	}
	log.Debugf("response %s", respData)

	response.Signature, err = r.signer.SignJSON(response.MetaResponse)
	if err != nil {
		log.Warn(err)
		// continue without the signature
	}
	return types.Message{
		TimeStamp: int32(time.Now().Unix()),
		Context:   request.context,
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

type requestMethod func(request routerRequest, router *Router)

// type methodMap map[string]func(msg types.Message, request routerRequest, r *Router)
type methodMap map[string]requestMethod

// Router holds a router object
type Router struct {
	privateRequestMap methodMap
	publicRequestMap  methodMap
	inbound           <-chan types.Message
	storage           data.Storage
	transport         net.Transport
	signer            signature.SignKeys
	census            *census.Manager
	tmclient          *voclient.HTTP
	Scrutinizer       *scrutinizer.Scrutinizer
	PrivateCalls      uint64
	PublicCalls       uint64
	codec             *amino.Codec
	APIs              []string
}

func NewRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport,
	signer signature.SignKeys) *Router {
	privateReqMap := make(methodMap)
	publicReqMap := make(methodMap)
	cm := new(census.Manager)
	r := new(Router)
	r.privateRequestMap = privateReqMap
	r.publicRequestMap = publicReqMap
	r.census = cm
	r.inbound = inbound
	r.storage = storage
	r.transport = transport
	r.signer = signer
	r.codec = amino.NewCodec()
	r.registerPublic("getGatewayInfo", info)
	return r
}

type routerRequest struct {
	types.MetaRequest

	method        string
	id            string
	authenticated bool
	address       string
	context       types.MessageContext
	private       bool
}

// semi-unmarshalls message, returns method name
func (r *Router) getRequest(payload []byte, context types.MessageContext) (request routerRequest, err error) {
	var msgStruct types.RequestMessage
	err = json.Unmarshal(payload, &msgStruct)
	if err != nil {
		return request, err
	}
	request.MetaRequest = msgStruct.MetaRequest
	request.method = msgStruct.Method
	if request.method == "" {
		return request, errors.New("method is empty")
	}
	if fn := r.publicRequestMap[request.method]; fn != nil {
		// if method is Public
		request.private = false
		request.authenticated = true
		request.address = "00000000000000000000"
	} else if fn := r.privateRequestMap[request.method]; fn != nil {
		// if method is Private
		request.private = true
		request.authenticated, request.address, err = r.signer.VerifyJSONsender(msgStruct.MetaRequest, msgStruct.Signature)
		// if no authrized keys, authenticate all requests
		if !request.authenticated && len(r.signer.Authorized) == 0 {
			request.authenticated = true
		}
	} else {
		// if method not found
		return request, fmt.Errorf("method not valid [%s]", request.method)
	}
	request.id = msgStruct.ID
	request.context = context
	// assign rawRequest by calling json.Marshal on the Request field. This works (tested against marshalling requestMap)
	// because json.Marshal encodes in lexographic order for map objects.
	// request.raw, err = json.Marshal(msgStruct.MetaRequest)
	return request, err
}

// InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport,
	signer *signature.SignKeys) *Router {
	log.Infof("using signer with address %s", signer.EthAddrString())
	return NewRouter(inbound, storage, transport, *signer)
}

func (r *Router) registerPrivate(methodName string, methodCallback requestMethod) {
	r.privateRequestMap[methodName] = methodCallback
}

func (r *Router) registerPublic(methodName string, methodCallback requestMethod) {
	r.publicRequestMap[methodName] = methodCallback
}

// EnableFileAPI enables the FILE API in the Router
func (r *Router) EnableFileAPI() {
	r.APIs = append(r.APIs, "file")
	r.registerPublic("fetchFile", fetchFile)
	r.registerPrivate("addFile", addFile)
	r.registerPrivate("pinList", pinList)
	r.registerPrivate("pinFile", pinFile)
	r.registerPrivate("unpinFile", unpinFile)
}

// EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.Manager) {
	r.APIs = append(r.APIs, "census")
	r.census = cm
	cm.Storage = r.storage
	r.registerPublic("getRoot", censusLocal)
	r.registerPrivate("dump", censusLocal)
	r.registerPrivate("dumpPlain", censusLocal)
	r.registerPublic("getSize", censusLocal)
	r.registerPublic("genProof", censusLocal)
	r.registerPublic("checkProof", censusLocal)
	r.registerPrivate("addCensus", censusLocal)
	r.registerPrivate("addClaim", censusLocal)
	r.registerPrivate("addClaimBulk", censusLocal)
	r.registerPrivate("publish", censusLocal)
	r.registerPrivate("importRemote", censusLocal)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableVoteAPI(rpcClient *voclient.HTTP) {
	r.APIs = append(r.APIs, "vote")
	r.tmclient = rpcClient
	r.registerPublic("submitEnvelope", submitEnvelope)
	r.registerPublic("getEnvelopeStatus", getEnvelopeStatus)
	r.registerPublic("getEnvelope", getEnvelope)
	r.registerPublic("getEnvelopeHeight", getEnvelopeHeight)
	r.registerPublic("getProcessList", getProcessList)
	r.registerPublic("getEnvelopeList", getEnvelopeList)
	r.registerPublic("getBlockHeight", getBlockHeight)
	if r.Scrutinizer != nil {
		r.APIs = append(r.APIs, "results")
		r.registerPublic("getResults", getResults)
		r.registerPublic("getProcListResults", getProcListResults)
	}
}

// Route routes requests through the Router object
func (r *Router) Route() {
	if len(r.publicRequestMap) == 0 && len(r.privateRequestMap) == 0 {
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
		var methodFunc requestMethod
		if !request.private {
			methodFunc = r.publicRequestMap[request.method]
		} else if request.private && request.authenticated {
			methodFunc = r.privateRequestMap[request.method]
		}
		if methodFunc == nil {
			errMsg := fmt.Sprintf("router has no method named %s or unauthorized", request.method)
			go r.sendError(request, errMsg)
			continue
		}

		log.Infof("api method %s", request.method)
		log.Debugf("received: %+v", request.MetaRequest)

		if request.private {
			r.PrivateCalls++
		} else {
			r.PublicCalls++
		}
		go methodFunc(request, r)
	}
}

func (r *Router) sendError(request routerRequest, errMsg string) {
	log.Warn(errMsg)
	var err error
	var response types.ErrorMessage
	response.ID = request.id
	response.Error.Request = request.id
	response.Error.Timestamp = int32(time.Now().Unix())
	response.Error.SetError(errMsg)
	response.Signature, err = r.signer.SignJSON(response.Error)
	if err != nil {
		log.Warn(err)
	}
	if request.context != nil {
		// TODO(mvdan): consolidate with Router.buildReply once we
		// simplify the api types.
		data, err := json.Marshal(response)
		if err != nil {
			log.Warnf("error marshaling response body: %s", err)
		}
		msg := types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.context,
			Data:      data,
		}
		r.transport.Send(msg)
	}
}

func info(request routerRequest, router *Router) {
	var response types.ResponseMessage
	response.MetaResponse.Timestamp = int32(time.Now().Unix())
	response.MetaResponse.APIList = router.APIs
	response.MetaResponse.Request = request.id
	router.transport.Send(router.buildReply(request, response))
}
