package router

import (
	"strings"
	"time"

	signature "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/types"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/log"

	"encoding/json"
)

func buildFailReply(requestId, message string) []byte {
	var response types.FailBody
	response.ID = requestId
	response.Error.Message = message
	response.Error.Request = requestId
	rawResponse, err := json.Marshal(response)
	if err != nil {
		log.Warnf("Error marshaling response body: %s", err)
	}
	return rawResponse
}

func signMsg(message interface{}, signer signature.SignKeys) string {
	rawMsg, err := json.Marshal(message)
	if err != nil {
		log.Warnf("Unable to marshal message to sign: %s", err)
	}
	sig, err := signer.Sign(string(rawMsg))
	if err != nil {
		sig = "0x00"
		log.Warnf("Error signing response body: %s", err)
	}
	return sig
}

func buildReply(msg types.Message, data []byte) types.Message {
	reply := new(types.Message)
	reply.TimeStamp = time.Now()
	reply.Context = msg.Context
	reply.Data = data
	return *reply
}

//semi-unmarshalls message, returns method name
func getMethod(payload []byte) (string, []byte, error) {
	var msgStruct types.MessageRequest
	err := json.Unmarshal(payload, &msgStruct)
	if err != nil {
		return "", nil, err
	}
	method, ok := msgStruct.Request["method"].(string)
	if !ok {
		log.Warnf("No method field in request or malformed")
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

type methodMap map[string]func(msg types.Message, rawRequest []byte, storage data.Storage, transport net.Transport, signer signature.SignKeys)

//Router holds a router object
type Router struct {
	requestMap methodMap
	inbound    <-chan types.Message
	storage    data.Storage
	transport  net.Transport
	signer     signature.SignKeys
}

//InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage, transport net.Transport, signer signature.SignKeys, dvoteEnabled bool) Router {
	requestMap := make(methodMap)
	routerObj := Router{requestMap, inbound, storage, transport, signer}
	if dvoteEnabled {
		routerObj.registerMethod("fetchFile", fetchFileMethod)
		routerObj.registerMethod("addFile", addFileMethod)
		routerObj.registerMethod("pinList", pinListMethod)
		routerObj.registerMethod("pinFile", pinFileMethod)
		routerObj.registerMethod("unpinFile", unpinFileMethod)
	}
	return routerObj
}

func (r *Router) registerMethod(methodName string, 
	methodCallback requestMethod) {
	r.requestMap[methodName] = methodCallback
}


//Route routes requests through the Router object
func (r *Router) Route() {
	if len(r.requestMap) == 0 {
		log.Warnf("Router methods are not properly initialized: %v", r)
		return
	}
	for {
		select {
		case msg := <-r.inbound:

			/*getMethod pulls method name and rawRequest from msg.Data*/
			method, rawRequest, err := getMethod(msg.Data)
			if err != nil {
				log.Warnf("Couldn't extract method from JSON message %v", msg)
				break
			}
			methodFunc := r.requestMap[method]
			if methodFunc == nil {
				log.Warnf("Router has no method named %s", method)
			} else {
				methodFunc(msg, rawRequest, r.storage, r.transport, r.signer)
			}
		}
	}
}