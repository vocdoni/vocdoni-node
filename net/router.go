package net

import (
	"errors"
	"fmt"
	"log"
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
  "signature": "%x"
}`

//content file must be b64 encoded
var fetchResponseFmt = `{
    "content": "%s",
    "request": "%s",
    "timestamp": %d
  }`

var addResponseFmt = `{
	"uri": "%s",
	"request": "%s",
	"timestamp": %d
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
	sig := signer.Sign(response)
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

func checkSig

func Route(inbound <-chan types.Message, storage data.Storage, wsTransport Transport, signer signature.SignKeys) {
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

			requestMap, ok := msgMap["request"].(map[string]interface{})
			if !ok {
				log.Printf("No request field in message or malformed")
			}
			method, ok := requestMap["method"].(string)
			if !ok {
				log.Printf("No method field in request or malformed")
			}

			switch method {
			case "fetchFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in fetchFile request or malformed")
					//err to errchan, reply
				}
				content, err := storage.Retrieve(uri)
				if err != nil {
					failString := fmt.Sprintf("Error fetching file %s", requestMap["uri"])
					fmt.Printf(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)))
				}
				b64content := base64.StdEncoding.EncodeToString(content)
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(fetchResponseFmt, b64content, requestId, time.Now().UnixNano()), signer)))
				log.Printf("%v", content)
			case "addFile":
				//check auth
				content, ok := requestMap["content"].(string)
				if !ok {
					log.Printf("No content field in addFile request or malformed")
					//err to errchan, reply
					break
				}
				b64content, err := base64.StdEncoding.DecodeString(content)
				if err != nil {
					log.Printf("Couldn't decode content")
					//err to errchan, reply
					break
				}
				/*
					name, ok := requestMap["name"].(string)
					if !ok {
						log.Printf("No name field in addFile request or malformed")
						//err to errchan, reply
						break
					}
						timestamp, errAtoi := strconv.ParseInt(requestMap["timestamp"].(), 10, 64)
						if errAtoi != nil {
							log.Printf("timestamp wrong format")
							//err to errchan, reply
							break
						}
				*/
				reqType, ok := requestMap["type"].(string)
				if !ok {
					log.Printf("No type field in addFile request or malformed")
					//err to errchan, reply
					break
				}
				switch reqType {
				case "swarm":
					// TODO: Only need IPFS for now
					//err to errchan, reply
					break
				case "ipfs":
					cid, err := storage.Publish(b64content)
					if err != nil {
						log.Printf("cannot add file")
					}
					//log.Printf("added %s with name %s and with timestamp %s", cid, name, timestamp)
					wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(addResponseFmt, cid, requestId, time.Now().UnixNano()), signer)))
				}
				//data.Publish
			case "pinList":
				pins, err := storage.ListPins()
				if err != nil {
					log.Printf("Internal error fetching pins")
				}
				pinsJsonArray, err := json.Marshal(pins)
				if err != nil {
					log.Printf("Internal error parsing pins")
				}
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(listPinsResponseFmt, pinsJsonArray, requestId, time.Now().UnixNano()), signer)))

				//data.Pins
			case "pinFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in pinFile request or malformed")
				}
				err := storage.Pin(uri)
				if err != nil {
					failString := fmt.Sprintf("Error pinning file %s", requestMap["uri"])
					log.Printf(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)))
				}
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(boolResponseFmt, "true", requestId, time.Now().UnixNano()), signer)))
			case "unpinFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in unpinFile request or malformed")
				}
				err := storage.Pin(uri)
				if err != nil {
					failString := fmt.Sprintf("Error unpinning file %s", requestMap["uri"])
					log.Printf(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)))
				}
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(boolResponseFmt, "true", requestId, time.Now().UnixNano()), signer)))
			}
		}
	}
}
