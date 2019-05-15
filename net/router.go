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
			requestMap := msgMap["request"].(map[string]interface{})
			method := fmt.Sprintf("%v", requestMap["method"].(string))
			switch method {
			case "ping":
				reply := new(types.Message)
				reply.TimeStamp = time.Now()
				reply.Data = []byte("pong")
				reply.Context = msg.Context
				wsTransport.Send(*reply, errors)
			case "fetchFile":
				content, err := storage.Retrieve(fmt.Sprintf("%v", requestMap["uri"]))
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
