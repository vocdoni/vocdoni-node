package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/gorilla/websocket"
	signature "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

// APIConnection holds an API websocket connection
type APIConnection struct {
	Conn *websocket.Conn
}

// NewAPIConnection starts a connection with the given endpoint address. The
// connection is closed automatically when the test or benchmark finishes.
func NewAPIConnection(addr string) *APIConnection {
	r := &APIConnection{}
	var err error
	r.Conn, _, err = websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
	return r
}

// Request makes a request to the previously connected endpoint
func (r *APIConnection) Request(req types.MetaRequest, signer *signature.SignKeys) *types.MetaResponse {
	method := req.Method
	var cmReq types.RequestMessage
	cmReq.MetaRequest = req
	cmReq.ID = fmt.Sprintf("%d", rand.Intn(1000))
	cmReq.Timestamp = int32(time.Now().Unix())
	if signer != nil {
		var err error
		cmReq.Signature, err = signer.SignJSON(cmReq.MetaRequest)
		if err != nil {
			log.Fatalf("%s: %v", method, err)
		}
	}
	rawReq, err := json.Marshal(cmReq)
	if err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	if err := r.Conn.WriteMessage(websocket.TextMessage, rawReq); err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	_, message, err := r.Conn.ReadMessage()
	log.Debugf("%s", message)
	if err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	var cmRes types.ResponseMessage
	if err := json.Unmarshal(message, &cmRes); err != nil {
		log.Fatalf("%s: %v", method, err)
	}
	if cmRes.ID != cmReq.ID {
		log.Fatalf("%s: %v", method, "request ID doesn'tb match")
	}
	if cmRes.Signature == "" {
		log.Fatalf("%s: empty signature in response: %s", method, message)
	}
	return &cmRes.MetaResponse
}

func printNice(resp *types.MetaResponse) {
	v := reflect.ValueOf(*resp)
	typeOfS := v.Type()
	output := "\n\n"
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Type().Name() == "bool" || v.Field(i).Type().Name() == "int64" || !v.Field(i).IsZero() {
			output += fmt.Sprintf("%v: %v\n", typeOfS.Field(i).Name, v.Field(i))
		}
	}
	log.Info(output)
}

func main() {
	host := flag.String("host", "ws://0.0.0.0:9090/dvote", "host to connect to")
	method := flag.String("method", "getGatewayInfo", "method to call")
	flag.Parse()
	log.Init("info", "stdout")

	signer := new(signature.SignKeys)
	signer.Generate()

	c := NewAPIConnection(*host)
	defer c.Conn.Close()

	var req types.MetaRequest
	req.Method = *method
	resp := c.Request(req, signer)
	if !resp.Ok {
		printNice(resp)
	} else {
		printNice(resp)
	}
}
