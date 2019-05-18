package main

import (
	"flag"
	"log"
	"time"

	"github.com/vocdoni/go-dvote/chain"
)

/*
Example code for using web3 implementation

Testing the RPC can be performed with curl and/or websocat
 curl -X POST -H "Content-Type:application/json" --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' localhost:9091
 echo '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' | websocat ws://127.0.0.1:9092
*/
func main() {
	chainType := flag.String("chain", "vctestnet", "Blockchain to connect")
	wsPort := flag.Int("wsPort", 0, "websockets port")
	wsHost := flag.String("wsHost", "0.0.0.0", "ws host to listen on")
	httpPort := flag.Int("httpPort", 9091, "http endpoint port, disabled if 0")
	httpHost := flag.String("httpHost", "0.0.0.0", "http host to listen on")

	flag.Parse()

	cfg, err := chain.NewConfig(*chainType)
	if err != nil {
		panic(err)
	}
	cfg.WSPort = *wsPort
	cfg.WSHost = *wsHost
	cfg.HTTPPort = *httpPort
	cfg.HTTPHost = *httpHost

	node, err := chain.Init(cfg)
	if err != nil {
		log.Panic(err)
	}

	node.Start()

	for {
		time.Sleep(1 * time.Second)
	}

}
