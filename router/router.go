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

func buildReply(msg types.Message, data []byte) types.Message {
	reply := new(types.Message)
	reply.TimeStamp = int32(time.Now().Unix())
	reply.Context = msg.Context
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

func parseTransportFromUri(uris []string) []string {
	out := make([]string, 0)
	for _, u := range uris {
		splt := strings.Split(u, "/")
		out = append(out, splt[0])
	}
	return out
}

type requestMethod func(msg types.Message, request routerRequest, router *Router)

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
	rawRequest    []byte
	authenticated bool
	address       string
}

//semi-unmarshalls message, returns method name
func (r *Router) getRequest(payload []byte) (request routerRequest, err error) {
	var msgStruct types.MessageRequest
	err = json.Unmarshal(payload, &msgStruct)
	if err != nil {
		log.Errorf("could not unmarshall JSON, error: %s", err)
		return request, err
	}
	request.authenticated, request.address, err = r.signer.VerifyJSONsender(msgStruct.Request, msgStruct.Signature)
	request.method = msgStruct.Request.Method
	request.id = msgStruct.ID
	/*assign rawRequest by calling json.Marshal on the Request field. This works (tested against marshalling requestMap)
	because json.Marshal encodes in lexographic order for map objects. */
	request.rawRequest, err = json.Marshal(msgStruct.Request)
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
	r.registerMethod("fetchFile", fetchFileMethod, false) // false = public method
	r.registerMethod("addFile", addFileMethod, true)      // true = private method
	r.registerMethod("pinList", pinListMethod, true)
	r.registerMethod("pinFile", pinFileMethod, true)
	r.registerMethod("unpinFile", unpinFileMethod, true)
}

//EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.CensusManager) {
	r.census = cm
	cm.Data = &r.storage
	r.registerMethod("getRoot", censusLocalMethod, false) // false = public method
	r.registerMethod("dump", censusLocalMethod, true)     // true = private method
	r.registerMethod("dumpPlain", censusLocalMethod, true)
	r.registerMethod("genProof", censusLocalMethod, false)
	r.registerMethod("checkProof", censusLocalMethod, false)
	r.registerMethod("addCensus", censusLocalMethod, true)
	r.registerMethod("addClaim", censusLocalMethod, true)
	r.registerMethod("addClaimBulk", censusLocalMethod, true)
	r.registerMethod("publish", censusLocalMethod, true)
	r.registerMethod("importRemote", censusLocalMethod, true)
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
			request, err := r.getRequest(msg.Data)
			if err != nil || request.method == "" {
				log.Warnf("couldn't extract method from JSON message %s", msg)
				break
			}
			methodFunc := r.publicRequestMap[request.method]
			if methodFunc == nil && request.authenticated {
				methodFunc = r.privateRequestMap[request.method]
			}
			if methodFunc == nil {
				errMsg := fmt.Sprintf("router has no method named %s or unauthorized", request.method)
				log.Warn(errMsg)
				go sendError(r.transport, r.signer, msg, request.id, errMsg)
			} else {
				log.Infof("calling method %s", request.method)
				log.Debugf("data received: %s", request.rawRequest)
				go methodFunc(msg, request, r)
			}
		}
	}
}

func sendError(transport net.Transport, signer signature.SignKeys, msg types.Message, requestID, errMsg string) {
	log.Warn(errMsg)
	var err error
	var response types.ErrorResponse
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
	transport.Send(buildReply(msg, rawResponse))
}
