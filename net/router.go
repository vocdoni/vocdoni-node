package net

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

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
    "timestamp": %s
  }`

var addResponseFmt = `{
	"uri": "%s",
	"request": "%s",
	"timestamp": %s,
}`

var listPinsResponseFmt = `{
    "files": %s,
    "request": "%s",
    "timestamp": %s 
  }`

var boolResponseFmt = `{
    "ok": %s,
    "request": "%s",
    "timestamp": %s
  }`

func failBody(requestId string, failString string) []byte {
	return []byte(fmt.Sprintf(failBodyFmt, requestId, failString))
}

func successBody(requestId string, response string) []byte {
	//need to calculate signature over response!
	sig := []byte{0}
	return []byte(fmt.Sprintf(successBodyFmt, requestId, response, sig))
}

func buildReply(msg types.Message, data []byte) types.Message {
	reply := new(types.Message)
	reply.TimeStamp = time.Now()
	reply.Context = msg.Context
	reply.Data = data
	return *reply
}

func Route(inbound <-chan types.Message, outbound chan<- types.Message, errorChan chan<- error, storage data.Storage, wsTransport Transport) {
	for {
		select {
		case msg := <-inbound:

			//probably from here to the switch should be factored out
			var msgJSON interface{}
			err := json.Unmarshal(msg.Data, &msgJSON)
			if err != nil {
				log.Printf("Couldn't parse message JSON on message %v", msg)
			}
			msgMap := msgJSON.(map[string]interface{})
			if msgMap["Type"] == "zk-snarks-envelope" || msgMap["type"] == "lrs-envelope" {
				outbound <- msg
				break
			}
			requestId, ok := msgMap["id"].(string)
			if !ok {
				log.Printf("No ID field in message or malformed")
			}

			requestMap, ok := msgMap["request"].(map[string]interface{})
			if !ok {
				log.Printf("No request field in message or malformed")
			}
			method := fmt.Sprintf("%v", requestMap["method"].(string))

			switch method {
			case "ping":
				wsTransport.Send(buildReply(msg, []byte("pong")), errorChan)
			case "fetchFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in fetchFile request or malformed")
					//err to errchan, reply
				}
				content, err := storage.Retrieve(uri)
				if err != nil {
					failString := fmt.Sprintf("Error fetching file %s", requestMap["uri"])
					errorChan <- errors.New(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)), errorChan)
				}
				b64content := base64.StdEncoding.EncodeToString(content)
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(fetchResponseFmt, b64content, requestId, time.Now().String()))), errorChan)
				log.Printf("%v", content)
			case "addFile":
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
				name, ok := requestMap["name"].(string)
				if !ok {
					log.Printf("No name field in addFile request or malformed")
					//err to errchan, reply
					break
				}
				timestamp, errAtoi := strconv.ParseInt(requestMap["timestamp"].(string), 10, 64)
				if errAtoi != nil {
					log.Printf("timestamp wrong format")
					//err to errchan, reply
					break
				}
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
					log.Printf("added %s with name %s and with timestamp %s", cid, name, timestamp)
					wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(addResponseFmt, cid, requestId, time.Now().String()))), errorChan)
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
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(listPinsResponseFmt, pinsJsonArray, requestId, time.Now()))), errorChan)

				//data.Pins
			case "pinFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in pinFile request or malformed")
				}
				err := storage.Pin(uri)
				if err != nil {
					failString := fmt.Sprintf("Error pinning file %s", requestMap["uri"])
					errorChan <- errors.New(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)), errorChan)
				}
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(boolResponseFmt, "true", requestId, time.Now().String()))), errorChan)
			case "unpinFile":
				uri, ok := requestMap["uri"].(string)
				if !ok {
					log.Printf("No uri in unpinFile request or malformed")
				}
				err := storage.Pin(uri)
				if err != nil {
					failString := fmt.Sprintf("Error unpinning file %s", requestMap["uri"])
					errorChan <- errors.New(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)), errorChan)
				}
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(boolResponseFmt, "true", requestId, time.Now().String()))), errorChan)
			}
		}
	}
}
