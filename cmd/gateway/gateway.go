package main

import (
	flag "github.com/spf13/pflag"
	sig "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"

	"log"
	"strings"
	"time"

	"github.com/vocdoni/go-dvote/chain"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/router"
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

	allowPrivate := flag.Bool("allowPrivate", false, "allows authorized clients to call private methods")
	allowedAddrs := flag.String("allowedAddrs", "", "comma delimited list of allowed client eth addresses")
	signingKey := flag.String("signingKey", "", "request signing key for this node")

	chainType := flag.String("chain", "vctestnet", "Blockchain to connect")
	w3wsPort := flag.Int("w3wsPort", 0, "websockets port")
	w3wsHost := flag.String("w3wsHost", "0.0.0.0", "ws host to listen on")
	w3httpPort := flag.Int("w3httpPort", 9091, "http endpoint port, disabled if 0")
	w3httpHost := flag.String("w3httpHost", "0.0.0.0", "http host to listen on")
	w3Route := flag.String("w3Route", "/web3", "proxy endpoint exposing web3")

	ipfsDaemon := flag.String("ipfsDaemon", "ipfs", "ipfs daemon path")
	ipfsNoInit := flag.Bool("ipfsNoInit", false, "do not start ipfs daemon (if already started)")

	sslDomain := flag.String("sslDomain", "", "ssl secure domain")
	sslDirCert := flag.String("sslDirCert", "./", "path where the ssl files will be stored")

	flag.Parse()

	p := net.NewProxy()
	p.C = new(types.Connection)
	p.C.SSLDomain = *sslDomain
	p.C.SSLCertDir = *sslDirCert
	p.C.Address = *dvoteHost
	p.C.Port = *dvotePort
	p.Init()

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

		node, err = chain.Init(w3cfg)
		if err != nil {
			log.Panic(err)
		}

		node.Start()
		time.Sleep(1 * time.Second)
		p.AddHandler(*w3Route, p.AddEndpoint(*w3httpHost, *w3httpPort))
		log.Printf("web3 available at %s", *w3Route)
	}

	var signer *sig.SignKeys
	signer = new(sig.SignKeys)
	if *allowPrivate && *allowedAddrs != "" {
		keys := strings.Split(*allowedAddrs, ",")
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
	} else if *w3Enabled {
		acc := node.Keys.Accounts()
		if len(acc) > 0 {
			keyJSON, err := node.Keys.Export(acc[0], "", "")
			if err != nil {
				log.Fatal(err)
			}
			err = signer.AddKeyFromEncryptedJSON(keyJSON, "")
			if err != nil {
				log.Fatal(err)
			}
		}
	} else {
		err := signer.Generate()
		if err != nil {
			log.Fatal(err)
		}
	}

	listenerOutput := make(chan types.Message)

	if *dvoteEnabled {

		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(p)
		ws.AddProxyHandler(*dvoteRoute)
		log.Printf("ws available at %s", *dvoteRoute)

		ipfsConfig := data.IPFSNewConfig()
		ipfsConfig.Start = !*ipfsNoInit
		ipfsConfig.Binary = *ipfsDaemon
		storage, err := data.InitDefault(data.StorageIDFromString("IPFS"), ipfsConfig)
		if err != nil {
			log.Fatal(err)
		}

		go ws.Listen(listenerOutput)
		router := router.InitRouter(listenerOutput, storage, ws, *signer, *dvoteEnabled)
		go router.Route()

	}

	for {
		time.Sleep(1 * time.Second)
	}
}
