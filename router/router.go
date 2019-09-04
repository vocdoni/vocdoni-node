package router

import (
	"fmt"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/types"

	"encoding/json"
)

func buildReply(context types.MessageContext, data []byte) types.Message {
	reply := new(types.Message)
	reply.TimeStamp = int32(time.Now().Unix())
	reply.Context = context
	reply.Data = data
	return *reply
}

func parseUrisContent(uris string) []string {
	out := make([]string, 0)
	urisSplit := strings.Split(uris, ",")
	for _, u := range urisSplit {
		out = append(out, u)
	}
	return out
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

//type methodMap map[string]func(msg types.Message, request routerRequest, r *Router)
type methodMap map[string]requestMethod

//Router holds a router object
type Router struct {
	privateRequestMap methodMap
	publicRequestMap  methodMap
	inbound           <-chan types.Message
	storage           data.Storage
	transport         net.Transport
	signer            signature.SignKeys
	census            *census.CensusManager
}

type routerRequest struct {
	method        string
	id            string
	structured    types.MetaRequest
	authenticated bool
	address       string
	context       types.MessageContext
}

//semi-unmarshalls message, returns method name
func (r *Router) getRequest(payload []byte, context types.MessageContext) (request routerRequest, err error) {
	var msgStruct types.RequestMessage
	err = json.Unmarshal(payload, &msgStruct)
	if err != nil {
		log.Errorf("could not unmarshall JSON, error: %s", err)
		return request, err
	}
	request.structured = msgStruct.Request
	request.authenticated, request.address, err = r.signer.VerifyJSONsender(msgStruct.Request, msgStruct.Signature)
	request.method = msgStruct.Request.Method
	request.id = msgStruct.ID
	request.context = context
	/*assign rawRequest by calling json.Marshal on the Request field. This works (tested against marshalling requestMap)
	because json.Marshal encodes in lexographic order for map objects.
	request.raw, err = json.Marshal(msgStruct.Request)
	*/
	return request, err
}

//InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport,
	signer signature.SignKeys) Router {
	privateReqMap := make(methodMap)
	publicReqMap := make(methodMap)
	cm := new(census.CensusManager)
	log.Infof("using signer with address %s", signer.EthAddrString())
	return Router{privateReqMap, publicReqMap, inbound, storage, transport, signer, cm}
}

func (r *Router) registerMethod(methodName string, methodCallback requestMethod, private bool) {
	if private {
		r.privateRequestMap[methodName] = methodCallback
	} else {
		r.publicRequestMap[methodName] = methodCallback
	}
}

//EnableFileAPI enables the FILE API in the Router
func (r *Router) EnableFileAPI() {
	r.registerMethod("fetchFile", fetchFile, false) // false = public method
	r.registerMethod("addFile", addFile, true)      // true = private method
	r.registerMethod("pinList", pinList, true)
	r.registerMethod("pinFile", pinFile, true)
	r.registerMethod("unpinFile", unpinFile, true)
}

//EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.CensusManager) {
	r.census = cm
	cm.Data = &r.storage
	r.registerMethod("getRoot", censusLocal, false) // false = public method
	r.registerMethod("dump", censusLocal, true)     // true = private method
	r.registerMethod("dumpPlain", censusLocal, true)
	r.registerMethod("genProof", censusLocal, false)
	r.registerMethod("checkProof", censusLocal, false)
	r.registerMethod("addCensus", censusLocal, true)
	r.registerMethod("addClaim", censusLocal, true)
	r.registerMethod("addClaimBulk", censusLocal, true)
	r.registerMethod("publish", censusLocal, true)
	r.registerMethod("importRemote", censusLocal, true)
}

//Route routes requests through the Router object
func (r *Router) Route() {
	if len(r.publicRequestMap) == 0 && len(r.privateRequestMap) == 0 {
		log.Warnf("router methods are not properly initialized: %v", r)
		return
	}
	for {
		select {
		case msg := <-r.inbound:
			request, err := r.getRequest(msg.Data, msg.Context)
			if request.method == "" {
				log.Warnf("couldn't extract method from JSON message %s", msg)
				break
			}
			if err != nil {
				log.Warnf("Error parsing request: %s", err.Error())
			}
			methodFunc := r.publicRequestMap[request.method]
			if methodFunc == nil && request.authenticated {
				methodFunc = r.privateRequestMap[request.method]
			}
			if methodFunc == nil {
				errMsg := fmt.Sprintf("router has no method named %s or unauthorized", request.method)
				log.Warn(errMsg)
				go sendError(r.transport, r.signer, request.context, request.id, errMsg)
			} else {
				log.Infof("calling method %s", request.method)
				log.Debugf("data received: %v+", request.structured)
				go methodFunc(request, r)
			}
		}
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
		log.Warn(err.Error())
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		log.Warnf("error marshaling response body: %s", err)
	}
	transport.Send(buildReply(context, rawResponse))
}
