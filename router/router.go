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
)

const (
	Private = true
	Public  = false
)

func buildReply(context types.MessageContext, data []byte) types.Message {
	reply := new(types.Message)
	reply.TimeStamp = int32(time.Now().Unix())
	reply.Context = context
	reply.Data = data
	return *reply
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
	census            *census.CensusManager
	tmclient          *voclient.HTTP
	PrivateCalls      int64
	PublicCalls       int64
	codec             *amino.Codec
}

func NewRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport,
	signer signature.SignKeys) *Router {
	privateReqMap := make(methodMap)
	publicReqMap := make(methodMap)
	cm := new(census.CensusManager)
	return &Router{
		privateRequestMap: privateReqMap,
		publicRequestMap:  publicReqMap,
		census:            cm,
		inbound:           inbound,
		storage:           storage,
		transport:         transport,
		signer:            signer,
		codec:             amino.NewCodec(),
	}
}

type routerRequest struct {
	method        string
	id            string
	structured    types.MetaRequest
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
	request.structured = msgStruct.Request
	request.method = msgStruct.Request.Method
	if request.method == "" {
		return request, errors.New("method is empty")
	}
	methodFunc := r.publicRequestMap[request.method]
	request.private = false
	if methodFunc == nil {
		methodFunc = r.privateRequestMap[request.method]
		if methodFunc != nil {
			// if method is Private
			request.private = true
			request.authenticated, request.address, err = r.signer.VerifyJSONsender(msgStruct.Request, msgStruct.Signature)
			// if no authrized keys, authenticate all requests
			if !request.authenticated && len(r.signer.Authorized) == 0 {
				request.authenticated = true
			}
		} else {
			// if method not found
			return request, fmt.Errorf("method not valid [%s]", request.method)
		}
	} else {
		// if method is Public
		request.authenticated = true
		request.address = "00000000000000000000"
	}
	request.id = msgStruct.ID
	request.context = context
	/*assign rawRequest by calling json.Marshal on the Request field. This works (tested against marshalling requestMap)
	because json.Marshal encodes in lexographic order for map objects.
	request.raw, err = json.Marshal(msgStruct.Request)
	*/
	return request, err
}

// InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport,
	signer *signature.SignKeys) *Router {
	log.Infof("using signer with address %s", signer.EthAddrString())
	return NewRouter(inbound, storage, transport, *signer)
}

func (r *Router) registerMethod(methodName string, methodCallback requestMethod, private bool) {
	if private {
		r.privateRequestMap[methodName] = methodCallback
	} else {
		r.publicRequestMap[methodName] = methodCallback
	}
}

// EnableFileAPI enables the FILE API in the Router
func (r *Router) EnableFileAPI() {
	r.registerMethod("fetchFile", fetchFile, Public) // false = public method
	r.registerMethod("addFile", addFile, Private)    // true = private method
	r.registerMethod("pinList", pinList, Private)
	r.registerMethod("pinFile", pinFile, Private)
	r.registerMethod("unpinFile", unpinFile, Private)
}

// EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.CensusManager) {
	r.census = cm
	cm.Data = &r.storage
	r.registerMethod("getRoot", censusLocal, Public) // false = public method
	r.registerMethod("dump", censusLocal, Private)   // true = private method
	r.registerMethod("dumpPlain", censusLocal, Private)
	r.registerMethod("getSize", censusLocal, Public)
	r.registerMethod("genProof", censusLocal, Public)
	r.registerMethod("checkProof", censusLocal, Public)
	r.registerMethod("addCensus", censusLocal, Private)
	r.registerMethod("addClaim", censusLocal, Private)
	r.registerMethod("addClaimBulk", censusLocal, Private)
	r.registerMethod("publish", censusLocal, Private)
	r.registerMethod("importRemote", censusLocal, Private)
}

// EnableVoteAPI enabled the Vote API in the Router
func (r *Router) EnableVoteAPI(rpcClient *voclient.HTTP) {
	r.tmclient = rpcClient
	r.registerMethod("submitEnvelope", submitEnvelope, Public)
	r.registerMethod("getEnvelopeStatus", getEnvelopeStatus, Public)
	r.registerMethod("getEnvelope", getEnvelope, Public)
	r.registerMethod("getEnvelopeHeight", getEnvelopeHeight, Public)
	r.registerMethod("getProcessList", getProcessList, Public)
	r.registerMethod("getEnvelopeList", getEnvelopeList, Public)
	r.registerMethod("getBlockHeight", getBlockHeight, Public)
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
			log.Warnf("error parsing request: %s", err)
			go sendError(r.transport, r.signer, request.context, request.id, "cannot parse request")
			break
		}
		var methodFunc requestMethod
		if !request.private {
			methodFunc = r.publicRequestMap[request.method]
		} else if request.private && request.authenticated {
			methodFunc = r.privateRequestMap[request.method]
		}
		if methodFunc == nil {
			errMsg := fmt.Sprintf("router has no method named %s or unauthorized", request.method)
			log.Warn(errMsg)
			go sendError(r.transport, r.signer, request.context, request.id, errMsg)
			continue
		}

		log.Infof("calling method %s", request.method)
		log.Debugf("data received: %+v", request.structured)

		if request.private {
			r.PrivateCalls++
			if r.PrivateCalls >= 2^64 {
				r.PrivateCalls = 0
			}
		} else {
			r.PublicCalls++
			if r.PublicCalls >= 2^64 {
				r.PublicCalls = 0
			}
		}
		go methodFunc(request, r)
	}
}

func sendError(transport net.Transport, signer signature.SignKeys, context types.MessageContext, requestID, errMsg string) {
	log.Warn(errMsg)
	var err error
	var response types.ErrorMessage
	response.ID = requestID
	response.Error.Request = requestID
	response.Error.Timestamp = int32(time.Now().Unix())
	response.Error.Message = errMsg
	response.Signature, err = signer.SignJSON(response.Error)
	if err != nil {
		log.Warn(err)
	}
	if context != nil {
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Warnf("error marshaling response body: %s", err)
		}
		transport.Send(buildReply(context, rawResponse))
	}
}
