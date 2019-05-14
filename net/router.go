package net

import (
	"github.com/vocdoni/go-dvote/types"
	"encoding/json"
)


func Route(inbound <-types.Message, outbound <-types.Message) {
	for {
		select {
		case msg <- inbound:
			var jsonMap map[string]interface{}
			err := json.Unmarshal(msg.Data, jsonMap)
			if err != nil {
				log.Error("Couldn't parse request JSON")
			}
			if (jsonMap["type"] == "zk-snarks-envelope" || jsonMap["type"] == "lrs-envelope") {
				outbound <- msg
				break
			}
			method := jsonMap["request"]["method"]
			swtich method {
			case "fetchFile":
				//data.Retrieve
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