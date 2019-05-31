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

func parseMsg(payload []byte) (map[string]interface{}, error) {
	var msgJSON interface{}
	err := json.Unmarshal(payload, &msgJSON)
	if err != nil {
		return nil, err
	}
	msgMap, ok := msgJSON.(map[string]interface{})
	if !ok {
		return nil, errors.New("Could not parse request JSON")
	}
	return msgMap, nil
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

			msgMap, err := parseMsg(msg.Data)
			if err != nil {
				log.Printf("Couldn't parse message JSON on message %v", msg)
			}

			requestId, ok := msgMap["id"].(string)
			if !ok {
				log.Printf("No ID field in message or malformed")
			}

			signature, ok := msgMap["signature"].(string)
			if !ok {
				log.Printf("No signature in request: %s", msg)
			}
			requestMap, ok := msgMap["request"].(map[string]interface{})
			if !ok {
				log.Printf("No request field in message or malformed")
			}

			rawRequest, err := json.Marshal(requestMap)

			method, ok := requestMap["method"].(string)
			if !ok {
				log.Printf("No method field in request or malformed")
			}

			switch method {

			case "fetchFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in fetchFile request or malformed: %s", uri)
					break
				}
				log.Printf("Called method fetchFile, uri %s", uri)
				go fetchFile(uri, requestId, msg, storage, transport, signer)

			case "addFile":

				authorized, err := signer.VerifySender(string(rawRequest), signature)
				if err != nil {
					log.Printf("Wrong authorization: %s", err)
					break
				}
				if authorized {
					content, ok := requestMap["content"].(string)
					if !ok {
						log.Printf("No content field in addFile request or malformed")
						break
					}
					b64content, err := base64.StdEncoding.DecodeString(content)
					if err != nil {
						log.Printf("Couldn't decode content")
						break
					}
					reqType, ok := requestMap["type"].(string)
					if !ok {
						log.Printf("No type field in addFile request or malformed")
						break
					}
					go addFile(reqType, requestId, b64content, msg, storage, transport, signer)

				} else {
					transport.Send(buildReply(msg, failBody(requestId, "Unauthorized")))
				}
				break
			case "pinList":
				authorized, err := signer.VerifySender(string(rawRequest), signature)
				if err != nil {
					log.Printf("Error checking authorization: %s", err)
					break
				}
				if authorized {
					go pinList(requestId, msg, storage, transport, signer)

				} else {
					transport.Send(buildReply(msg, failBody(requestId, "Unauthorized")))
				}
				break
			case "pinFile":
				authorized, err := signer.VerifySender(string(rawRequest), signature)
				if err != nil {
					log.Printf("Error checking authorization: %s", err)
					break
				}
				if authorized {
					uri, ok := requestMap["uri"].(string)
					if !ok {
						log.Printf("No uri in pinFile request or malformed")
						break
					}
					go pinFile(uri, requestId, msg, storage, transport, signer)
				} else {
					transport.Send(buildReply(msg, failBody(requestId, "Unauthorized")))
				}
				break
			case "unpinFile":
				authorized, err := signer.VerifySender(string(rawRequest), signature)
				if err != nil {
					log.Printf("Error checking authorization: %s", err)
					break
				}
				if authorized {
					uri, ok := requestMap["uri"].(string)
					if !ok {
						log.Printf("No uri in unpinFile request or malformed")
					}
					go unPinFile(uri, requestId, msg, storage, transport, signer)
				} else {
					transport.Send(buildReply(msg, failBody(requestId, "Unauthorized")))
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
