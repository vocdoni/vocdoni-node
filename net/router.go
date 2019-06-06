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

var failBodyFmt = `{
"id": "%[1]s", 
"error": {
  "request": "%[1]s",
  "message": "%s"
}
  }`

var successBodyFmt = `{
  "id": "%s",
  "response": %s,
  "signature": "0x%s"
}`

//content file must be b64 encoded
var fetchResponseFmt = `{
    "content": "%s",
    "request": "%s",
    "timestamp": %d
  }`

var addResponseFmt = `{
"request": "%s",
"timestamp": %d,
"uri": "%s"
}`

var listPinsResponseFmt = `{
    "files": %s,
    "request": "%s",
    "timestamp": %d 
  }`

var boolResponseFmt = `{
    "ok": %s,
    "request": "%s",
    "timestamp": %d
  }`

func failBody(requestId string, failString string) []byte {
	return []byte(fmt.Sprintf(failBodyFmt, requestId, failString))
}

func successBody(requestId string, response string, signer signature.SignKeys) []byte {
	sig, err := signer.Sign(response)
	if err != nil {
		sig = "0x00"
		log.Printf("Error signing response body: %s", err)
	}
	return []byte(fmt.Sprintf(successBodyFmt, requestId, response, sig))
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
	/*assign rawRequest by calling json.Marshal on the Request field. This is
	  assumed to work because json.Marshal encodes in lexographic order for map objects. */
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

func Route(inbound <-chan types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	for {
		select {
		case msg := <-inbound:

			/*getMethod pulls method name and rawRequest from msg.Data*/
			method, rawRequest, err := getMethod(msg.Data)
			if err != nil {
				log.Printf("Couldn't extract method from JSON message %v", msg)
				break
			}

			switch method {
			case "fetchFile":
				var fileRequest types.FetchFileRequest
				if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
					log.Printf("Couldn't decode into FetchFileRequest type from request %v", msg.Data)
					break
				}
				log.Printf("Called method fetchFile, uri %s", fileRequest.Request.URI)
				go fetchFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)

			case "addFile":
				var fileRequest types.AddFileRequest
				if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
					log.Printf("Couldn't decode into AddFileRequest type from request %s", msg.Data)
					break
				}
				authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
				if err != nil {
					log.Printf("Wrong authorization: %s", err)
					break
				}
				if authorized {
					content := fileRequest.Request.Content
					b64content, err := base64.StdEncoding.DecodeString(content)
					if err != nil {
						log.Printf("Couldn't decode content")
						break
					}
					reqType := fileRequest.Request.Type

					go addFile(reqType, fileRequest.ID, b64content, msg, storage, transport, signer)

				} else {
					transport.Send(buildReply(msg, failBody(fileRequest.ID, "Unauthorized")))
				}
				break
			case "pinList":
				var fileRequest types.PinListRequest
				if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
					log.Printf("Couldn't decode into PinListRequest type from request %s", msg.Data)
					break
				}
				authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
				if err != nil {
					log.Printf("Error checking authorization: %s", err)
					break
				}
				if authorized {
					go pinList(fileRequest.ID, msg, storage, transport, signer)
				} else {
					transport.Send(buildReply(msg, failBody(fileRequest.ID, "Unauthorized")))
				}
				break
			case "pinFile":
				var fileRequest types.PinFileRequest
				if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
					log.Printf("Couldn't decode into PinFileRequest type from request %s", msg.Data)
					break
				}
				authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
				if err != nil {
					log.Printf("Error checking authorization: %s", err)
					break
				}
				if authorized {
					go pinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
				} else {
					transport.Send(buildReply(msg, failBody(fileRequest.ID, "Unauthorized")))
				}
				break
			case "unpinFile":
				var fileRequest types.UnpinFileRequest
				if err := json.Unmarshal(msg.Data, &fileRequest); err != nil {
					log.Printf("Couldn't decode into UnpinFileRequest type from request %s", msg.Data)
					break
				}
				authorized, err := signer.VerifySender(string(rawRequest), fileRequest.Signature)
				if err != nil {
					log.Printf("Error checking authorization: %s", err)
					break
				}
				if authorized {

					go unPinFile(fileRequest.Request.URI, fileRequest.ID, msg, storage, transport, signer)
				} else {
					transport.Send(buildReply(msg, failBody(fileRequest.ID, "Unauthorized")))
				}
				break
			}
		}
	}
}

func unPinFile(uri, requestId string, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Printf("Calling UnPinFile %s", uri)
	err := storage.Unpin(uri)
	if err != nil {
		failString := fmt.Sprintf("Error unpinning file %s", uri)
		log.Printf(failString)
		transport.Send(buildReply(msg, failBody(requestId, failString)))
	} else {
		transport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(boolResponseFmt, "true", requestId, time.Now().UnixNano()), signer)))
	}
}

func pinFile(uri, requestId string, msg types.Message, storage data.Storage, transport Transport, signer signature.SignKeys) {
	log.Printf("Calling PinFile %s", uri)
	err := storage.Pin(uri)
	if err != nil {
		failString := fmt.Sprintf("Error pinning file %s", uri)
		log.Printf(failString)
		transport.Send(buildReply(msg, failBody(requestId, failString)))
	} else {
		transport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(boolResponseFmt, "true", requestId, time.Now().UnixNano()), signer)))
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
		transport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(listPinsResponseFmt, pinsJsonArray, requestId, time.Now().UnixNano()), signer)))
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
		transport.Send(buildReply(msg,
			successBody(requestId, fmt.Sprintf(addResponseFmt, requestId, time.Now().UnixNano(), ipfsRouteBaseURL+cid), signer)))
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
		failString := fmt.Sprintf("Error fetching uri %s", uri)
		fmt.Printf(failString)
		transport.Send(buildReply(msg, failBody(requestId, failString)))
	} else {
		b64content := base64.StdEncoding.EncodeToString(content)
		log.Printf("File fetched, b64 size %d", len(b64content))
		transport.Send(buildReply(msg, successBody(requestId,
			fmt.Sprintf(fetchResponseFmt, b64content, requestId, time.Now().UnixNano()),
			signer)))
	}
}
