package router

import (
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

//semi-unmarshalls message, returns method name
func getMethod(payload []byte) (string, []byte, error) {
	var msgStruct types.MessageRequest
	err := json.Unmarshal(payload, &msgStruct)
	if err != nil {
		log.Errorf("Could not unmarshall JSON, error: %s", err)
		return "", nil, err
	}
	method, ok := msgStruct.Request["method"].(string)
	if !ok {
		log.Warnf("no method field in request or malformed")
	}
	/*assign rawRequest by calling json.Marshal on the Request field. This works (tested against marshalling requestMap)
	because json.Marshal encodes in lexographic order for map objects. */
	rawRequest, err := json.Marshal(msgStruct.Request)
	if err != nil {
		return "", nil, err
	}
	return method, rawRequest, err
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

type methodMap map[string]func(msg types.Message, rawRequest []byte, r *Router)

//Router holds a router object
type Router struct {
	requestMap methodMap
	inbound    <-chan types.Message
	storage    data.Storage
	transport  net.Transport
	signer     signature.SignKeys
	census     *census.CensusManager
}

//InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport,
	signer signature.SignKeys) Router {
	requestMap := make(methodMap)
	cm := new(census.CensusManager)
	log.Infof("using signer with address %s", signer.EthAddrString())
	return Router{requestMap, inbound, storage, transport, signer, cm}
}

func (r *Router) registerMethod(methodName string, methodCallback requestMethod) {
	r.requestMap[methodName] = methodCallback
}

//EnableFileAPI enables the FILE API in the Router
func (r *Router) EnableFileAPI() {
	r.registerMethod("fetchFile", fetchFileMethod)
	r.registerMethod("addFile", addFileMethod)
	r.registerMethod("pinList", pinListMethod)
	r.registerMethod("pinFile", pinFileMethod)
	r.registerMethod("unpinFile", unpinFileMethod)
}

//EnableCensusAPI enables the Census API in the Router
func (r *Router) EnableCensusAPI(cm *census.CensusManager) {
	r.census = cm
	cm.Data = &r.storage
	r.registerMethod("getRoot", censusLocalMethod)
	r.registerMethod("dump", censusLocalMethod)
	r.registerMethod("dumpPlain", censusLocalMethod)
	r.registerMethod("genProof", censusLocalMethod)
	r.registerMethod("checkProof", censusLocalMethod)
	r.registerMethod("addCensus", censusLocalMethod)
	r.registerMethod("addClaim", censusLocalMethod)
	r.registerMethod("addClaimBulk", censusLocalMethod)
	r.registerMethod("publish", censusLocalMethod)
	r.registerMethod("importRemote", censusLocalMethod)
}

//Route routes requests through the Router object
func (r *Router) Route() {
	if len(r.requestMap) == 0 {
		log.Warnf("router methods are not properly initialized: %v", r)
		return
	}
	for {
		select {
		case msg := <-r.inbound:

			/*getMethod pulls method name and rawRequest from msg.Data*/
			method, rawRequest, err := getMethod(msg.Data)
			if err != nil {
				log.Warnf("couldn't extract method from JSON message %s", msg)
				break
			}
			methodFunc := r.requestMap[method]
			if methodFunc == nil {
				log.Warnf("router has no method named %s", method)
			} else {
				log.Infof("calling method %s", method)
				log.Debugf("data received: %+v", msg)
				methodFunc(msg, rawRequest, r)
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
