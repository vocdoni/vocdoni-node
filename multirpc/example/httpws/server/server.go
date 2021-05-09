package main

/*
Messages have the following structure:

{
  "request": {
    "error": "no signature provided",
    "request": "1234",
    "timestamp": 1602582404
  },
  "id": "1234",
  "signature": "6e1f5705f41c767d6d3ba516..."
}

You can test with curl:

curl -s 127.0.0.1:7788/main -X POST -d '{"request":{"method":"hello", "request":"1234"}, "id":"1234"}'
*/

import (
	"fmt"

	"go.vocdoni.io/dvote/multirpc/endpoint"
	"go.vocdoni.io/dvote/multirpc/example/httpws/message"
	"go.vocdoni.io/dvote/multirpc/router"
	"go.vocdoni.io/dvote/multirpc/transports"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
)

func main() {
	log.Init("debug", "stdout")

	// API configuration
	api := &endpoint.HTTPWSconfig{
		ListenHost: "0.0.0.0",
		ListenPort: 7788,
	}
	// Generate signing keys
	sig := ethereum.NewSignKeys()
	sig.Generate()

	// Create the channel for incoming messages and attach to transport
	listener := make(chan transports.Message)

	// Create HTTPWS endpoint (for HTTP(s) + Websockets(s) handling) using the endpoint interface
	ep := endpoint.HTTPWSendPoint{}

	// Configures the endpoint
	ep.SetOption(endpoint.OptionListenHost, api.ListenHost)
	ep.SetOption(endpoint.OptionListenPort, api.ListenPort)
	ep.SetOption(endpoint.OptionTLSdomain, api.TLSdomain)
	ep.SetOption(endpoint.OptionSetMode, endpoint.ModeHTTPWS)

	err := ep.Init(listener)
	if err != nil {
		panic(err)
	}

	// Create the transports map, this allows adding several transports on the same router
	transportMap := make(map[string]transports.Transport)
	transportMap[ep.ID()] = ep.Transport()

	// Create a new router and attach the transports
	r := router.NewRouter(listener, transportMap, sig, message.NewAPI)

	// Add namespace /main to the transport httpws
	r.Transports[ep.ID()].AddNamespace("/main")

	// And handler for namespace main and method hello
	if err := r.AddHandler("hello", "/main", hello, false, true); err != nil {
		log.Fatal(err)
	}

	if err := r.AddHandler("addkey", "/main", addKey, false, false); err != nil {
		log.Fatal(err)
	}

	// Add a private method
	if err := r.AddHandler("getsecret", "/main", getSecret, true, false); err != nil {
		log.Fatal(err)
	}

	// Start routing
	r.Route()
}

//////////////
// Handlers //
//////////////

func hello(rr router.RouterRequest) {
	msg := &message.MyAPI{}
	msg.ID = rr.Id
	msg.Reply = fmt.Sprintf("hello! got your message with ID %s", rr.Id)
	rr.Send(router.BuildReply(msg, rr))
}

func addKey(rr router.RouterRequest) {
	msg := &message.MyAPI{}

	if ok := rr.Signer.Authorized[rr.Address]; ok {
		msg.Error = fmt.Sprintf("address %s already authorized", rr.Address.Hex())
	} else {
		rr.Signer.AddAuthKey(rr.Address)
		log.Infof("adding pubKey %x", rr.SignaturePublicKey)
		msg.Reply = fmt.Sprintf("added new authorized address %s", rr.Address.Hex())
	}

	rr.Send(router.BuildReply(msg, rr))
}

func getSecret(rr router.RouterRequest) {
	msg := &message.MyAPI{Reply: "the secret is foobar123456"}
	rr.Send(router.BuildReply(msg, rr))
}
