package net

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	signature "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/types"

	"encoding/base64"
	"encoding/json"
)

func buildFailReply(requestId, message string) []byte {
	var response types.FailBody
	response.ID = requestId
	response.Error.Message = message
	response.Error.Request = requestId
	rawResponse, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response body: %s", err)
	}
	return rawResponse
}

func signMsg(message interface{}, signer signature.SignKeys) string {
	rawMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Unable to marshal message to sign: %s", err)
	}
	sig, err := signer.Sign(string(rawMsg))
	if err != nil {
		sig = "0x00"
		log.Printf("Error signing response body: %s", err)
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
		log.Printf("No method field in request or malformed")
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

type methodMap map[string]func(msg types.Message, rawRequest []byte, storage data.Storage, transport Transport, signer signature.SignKeys)

//Router holds a router object
type Router struct {
	requestMap methodMap
	inbound    <-chan types.Message
	storage    data.Storage
	transport  Transport
	signer     signature.SignKeys
	init       bool
}

//InitRouter sets up a Router object which can then be used to route requests
func InitRouter(inbound <-chan types.Message, storage data.Storage, transport Transport, signer signature.SignKeys,
	voteEnabled, censusEnabled, dvoteEnabled, w3Enabled bool) Router {
	requestMap := make(methodMap)

	if voteEnabled { //vote API methods go here
	}

	if censusEnabled { //census API methods go here
	}

	if dvoteEnabled {
		requestMap["fetchFile"] = func(msg types.Message, rawRequest []byte, storage data.Storage, transport Transport, signer signature.SignKeys) {
			var fileRequest types.FetchFileRequest
			if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
				log.Printf("Couldn't decode into FetchFileRequest type from request %v", msg.Data)
				return
			}
			log.Printf("Called method fetchFile, uri %s", fileRequest.Request.URI)
			go fetchFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
		}

		requestMap["addFile"] = func(msg types.Message, rawRequest []byte, storage data.Storage, transport Transport, signer signature.SignKeys) {
			var fileRequest types.AddFileRequest
			if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
				log.Printf("Couldn't decode into AddFileRequest type from request %s", msg.Data)
				return
			}
			authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
			if err != nil {
				log.Printf("Wrong authorization: %s", err)
				return
			}
			if authorized {
				content := fileRequest.Request.Content
				b64content, err := base64.StdEncoding.DecodeString(content)
				if err != nil {
					log.Printf("Couldn't decode content")
					return
				}
				reqType := fileRequest.Request.Type

				go addFile(reqType, fileRequest.ID, b64content, msg, storage, transport, signer)

			} else {
				transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
			}
		}

		requestMap["pinList"] = func(msg types.Message, rawRequest []byte, storage data.Storage, transport Transport, signer signature.SignKeys) {
			var fileRequest types.PinListRequest
			if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
				log.Printf("Couldn't decode into PinListRequest type from request %s", msg.Data)
				return
			}
			authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
			if err != nil {
				log.Printf("Error checking authorization: %s", err)
				return
			}
			if authorized {
				go pinList(fileRequest.ID, msg, storage, transport, signer)
			} else {
				transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
			}
		}

		requestMap["pinFile"] = func(msg types.Message, rawRequest []byte, storage data.Storage, transport Transport, signer signature.SignKeys) {
			var fileRequest types.PinFileRequest
			if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
				log.Printf("Couldn't decode into PinFileRequest type from request %s", msg.Data)
				return
			}
			authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
			if err != nil {
				log.Printf("Error checking authorization: %s", err)
				return
			}
			if authorized {
				go pinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
			} else {
				transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
			}
		}

		requestMap["unpinFile"] = func(msg types.Message, rawRequest []byte, storage data.Storage, transport Transport, signer signature.SignKeys) {
			var fileRequest types.UnpinFileRequest
			if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
				log.Printf("Couldn't decode into UnpinFileRequest type from request %s", msg.Data)
				return
			}
			authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
			if err != nil {
				log.Printf("Error checking authorization: %s", err)
				return
			}
			if authorized {

				go unPinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
			} else {
				transport.Send(buildReply(msg, buildFailReply(fileRequest.ID, "Unauthorized")))
			}
		}
	}

	if w3Enabled { //web3 API methods go here
	}

	routerObj := Router{requestMap, inbound, storage, transport, signer, true}
	return routerObj
}

//Route routes requests through the Router object
func (r Router) Route() {
	if !r.init {
		log.Printf("Router is not properly initialized: %v", r)
	}
	for {
		select {
		case msg := <-r.inbound:

			/*getMethod pulls method name and rawRequest from msg.Data*/
			method, rawRequest, err := getMethod(msg.Data)
			if err != nil {
				log.Printf("Couldn't extract method from JSON message %v", msg)
				break
			}
			methodFunc := r.requestMap[method]
			if methodFunc == nil {
				log.Printf("Router has no method named $s", method)
			} else {
				methodFunc(msg, rawRequest, r.storage, r.transport, r.signer)
			}
		}
	}
}

func unPinFile(uri, requestId string, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Printf("Calling UnPinFile %s", uri)
	err := storage.Unpin(uri)
	if err != nil {
		log.Printf(fmt.Sprintf("Error unpinning file %s", uri))
		transport.Send(buildReply(msg, buildFailReply(requestId, "Error unpinning file")))
	} else {
		var response types.BoolResponse
		response.ID = requestId
		response.Response.OK = true
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func pinFile(uri, requestId string, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Printf("Calling PinFile %s", uri)
	err := storage.Pin(uri)
	if err != nil {
		log.Printf(fmt.Sprintf("Error pinning file %s", uri))
		transport.Send(buildReply(msg, buildFailReply(requestId, "Error pinning file")))
	} else {
		var response types.BoolResponse
		response.ID = requestId
		response.Response.OK = true
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func pinList(requestId string, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Println("Calling PinList")
	pins, err := storage.ListPins()
	if err != nil {
		log.Printf("Internal error fetching pins")
	}
	pinsJsonArray, err := json.Marshal(pins)
	if err != nil {
		log.Printf("Internal error parsing pins")
	} else {
		var response types.ListPinsResponse
		response.ID = requestId
		response.Response.Files = pinsJsonArray
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}

func addFile(reqType, requestId string, b64content []byte, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Println("Calling addFile")
	switch reqType {
	case "swarm":
		// TODO
		break
	case "ipfs":
		cid, err := storage.Publish(b64content)
		if err != nil {
			log.Printf("Cannot add file")
		}
		log.Printf("Added file %s, b64 size of %d", cid, len(b64content))
		ipfsRouteBaseURL := "ipfs://"
		var response types.AddResponse
		response.ID = requestId
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Response.URI = ipfsRouteBaseURL + cid
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}

}

func fetchFile(uri, requestId string, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Printf("Calling FetchFile %s", uri)
	parsedURIs := parseUrisContent(uri)
	transportTypes := parseTransportFromUri(parsedURIs)
	var resp *http.Response
	var content []byte
	var err error
	found := false
	for idx, t := range transportTypes {
		if found {
			break
		}
		switch t {
		case "http:", "https:":
			resp, err = http.Get(parsedURIs[idx])
			defer resp.Body.Close()
			content, err = ioutil.ReadAll(resp.Body)
			if content != nil {
				found = true
			}
			break
		case "ipfs:":
			splt := strings.Split(parsedURIs[idx], "/")
			hash := splt[len(splt)-1]
			content, err = storage.Retrieve(hash)
			if content != nil {
				found = true
			}
			break
		case "bzz:", "bzz-feed":
			err = errors.New("Bzz and Bzz-feed not implemented yet")
			break
		}
	}

	if err != nil {
		fmt.Printf(fmt.Sprintf("Error fetching uri %s", uri))
		transport.Send(buildReply(msg, buildFailReply(requestId, "Error fetching uri")))
	} else {
		b64content := base64.StdEncoding.EncodeToString(content)
		log.Printf("File fetched, b64 size %d", len(b64content))
		var response types.FetchResponse
		response.ID = requestId
		response.Response.Content = b64content
		response.Response.Request = requestId
		response.Response.Timestamp = time.Now().UnixNano()
		response.Signature = signMsg(response.Response, signer)
		rawResponse, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response body: %s", err)
		}
		transport.Send(buildReply(msg, rawResponse))
	}
}
