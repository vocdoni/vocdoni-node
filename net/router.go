package net

import (
	"fmt"
	"log"
	"time"

	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/types"

	"encoding/json"
)

func Route(inbound <-chan types.Message, outbound chan<- types.Message, errors chan<- error, storage data.Storage, wsTransport Transport) {
	for {
		select {
		case msg := <-inbound:
			var genericJSON interface{}
			err := json.Unmarshal(msg.Data, &genericJSON)
			if err != nil {
				log.Printf("Couldn't parse message JSON on message %v", msg)
			}
			jsonMap := genericJSON.(map[string]interface{})
			if jsonMap["Type"] == "zk-snarks-envelope" || jsonMap["type"] == "lrs-envelope" {
				outbound <- msg
				break
			}
			var requestMap map[string]interface{}
			err = json.Unmarshal([]byte(jsonMap["request"].(string)), requestMap)
			if err != nil {
				log.Printf("Couldn't parse request JSON on request %v", jsonMap["request"])
			}
			method := fmt.Sprintf("%v", requestMap["method"])
			switch method {
			case "ping":
				reply := new(types.Message)
				reply.TimeStamp = time.Now()
				reply.Data = []byte("pong")
				reply.Context = msg.Context
				wsTransport.Send(*reply, errors)
			case "fetchFile":
				content, err := storage.Retrieve(fmt.Sprintf("%v", jsonMap["uri"]))
				if err != nil {
					log.Printf("Error fetching file on request %v", msg)
					//send error reply, and also send to error channel?
				}
				//send success
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
