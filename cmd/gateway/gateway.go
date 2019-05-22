package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/vocdoni/go-dvote/chain"
	sig "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"
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

	genesis := flag.String("genesis", "genesis.json", "Ethereum genesis file")
	netID := flag.Int("netID", 1714, "network ID for the Ethereum blockchain")
	w3wsPort := flag.Int("w3wsPort", 9091, "websockets port")
	w3wsHost := flag.String("w3wsHost", "0.0.0.0", "ws host to listen on")
	w3httpPort := flag.Int("w3httpPort", 0, "http endpoint port, disabled if 0")
	w3httpHost := flag.String("w3httpHost", "0.0.0.0", "http host to listen on")

	allowPrivate := flag.Bool("allowPrivate", true, "allows authorized clients to call private methods")
	allowedAddrs := flag.String("allowedAddrs", "", "comma delimited list of allowed client eth addresses")
	signingKey := flag.String("signingKey", "", "request signing key for this node")

	flag.Parse()

	var signer *sig.SignKeys
	if *allowPrivate {
		keys := strings.Split(*authKeys, ",")
		for _, key := range keys {
			err := signer.AddAuthKey(key)
			if err != nil {
				log.Printf("Error adding allowed key: %s", err)
			}
		}
	}

	if *signingKey != "" {
		err := signer.AddHexKey(*signingKey)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err := signer.Generate()
		if err != nil {
			log.Fatal(err)
		}
	}

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
		w3cfg := chain.NewConfig()
		w3cfg.WSPort = *w3wsPort
		w3cfg.WSHost = *w3wsHost
		w3cfg.HTTPPort = *w3httpPort
		w3cfg.HTTPHost = *w3httpHost
		w3cfg.NetworkGenesisFile = *genesis
		w3cfg.NetworkId = *netID

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
