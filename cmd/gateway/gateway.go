package main

import (
	"flag"
	"log"
	"time"

	"github.com/vocdoni/go-dvote/chain"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/types"
)

/*
Example code for using web3 implementation

Testing the RPC can be performed with curl and/or websocat
 curl -X POST -H "Content-Type:application/json" --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' localhost:9091
 echo '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' | websocat ws://127.0.0.1:9092
*/
func main() {
	dvoteEnabled := flag.Bool("fileApi", true, "enable file API")
	w3Enabled := flag.Bool("web3Api", true, "enable web3 API")

	dvoteHost := flag.String("dvoteHost", "0.0.0.0", "dvote API host")
	dvotePort := flag.Int("dvotePort", 9090, "dvote API port")
	dvoteRoute := flag.String("dvoteRoute", "/dvote", "dvote API route")

	chainType := flag.String("chain", "vctestnet", "Blockchain to connect")
	w3wsPort := flag.Int("w3wsPort", 0, "websockets port")
	w3wsHost := flag.String("w3wsHost", "0.0.0.0", "ws host to listen on")
	w3httpPort := flag.Int("w3httpPort", 9091, "http endpoint port, disabled if 0")
	w3httpHost := flag.String("w3httpHost", "0.0.0.0", "http host to listen on")

	flag.Parse()

	var websockets *net.Transport
	_ = websockets
	var storage *data.Storage
	_ = storage

	listenerOutput := make(chan types.Message)

	if *dvoteEnabled {
		wsCfg := new(types.Connection)
		wsCfg.Address = *dvoteHost
		wsCfg.Port = *dvotePort
		wsCfg.Path = *dvoteRoute

		websockets, err := net.Init(net.TransportIDFromString("Websocket"), wsCfg)
		if err != nil {
			log.Fatal(err)
		}

		storage, err := data.InitDefault(data.StorageIDFromString("IPFS"))
		if err != nil {
			log.Fatal(err)
		}

		go websockets.Listen(listenerOutput)
		go net.Route(listenerOutput, storage, websockets)
	}

	var node *chain.EthChainContext
	_ = node
	if *w3Enabled {
		w3cfg, err := chain.NewConfig(*chainType)
		if err != nil {
			log.Fatal(err)
		}
		w3cfg.WSPort = *w3wsPort
		w3cfg.WSHost = *w3wsHost
		w3cfg.HTTPPort = *w3httpPort
		w3cfg.HTTPHost = *w3httpHost

		node, err := chain.Init(w3cfg)
		if err != nil {
			log.Panic(err)
		}

		node.Start()
	}

	for {
		time.Sleep(1 * time.Second)
	}

}
