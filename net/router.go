package net

import (
	"errors"
	"fmt"
	"log"
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
			//probably from here to the switch should be factored out, error checking added
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
			requestId := msgMap["id"].(string)
			requestMap := msgMap["request"].(map[string]interface{})
			method := fmt.Sprintf("%v", requestMap["method"].(string))
			switch method {
			case "ping":
				wsTransport.Send(buildReply(msg, []byte("pong")), errorChan)
			case "fetchFile":
				content, err := storage.Retrieve(fmt.Sprintf("%v", requestMap["uri"]))
				if err != nil {
					failString := fmt.Sprintf("Error fetching file %s", requestMap["uri"])
					errorChan <- errors.New(failString)
					wsTransport.Send(buildReply(msg, failBody(requestId, failString)), errorChan)
				}
				b64content := base64.StdEncoding.EncodeToString(content)
				wsTransport.Send(buildReply(msg, successBody(requestId, fmt.Sprintf(fetchResponseFmt, b64content, requestId, time.Now().String()))), errorChan)
				log.Printf("%v", content)
			case "addFile":
				//data.Publish
			case "pinList":
				//data.Pins
			case "pinFile":
				//data.Pin
			case "unpinFile":
				//data.Unpin
			}
		}
	}
}
