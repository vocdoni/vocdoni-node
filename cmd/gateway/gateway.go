package main

import (
	viper "github.com/spf13/viper"
	flag "github.com/spf13/pflag"
	sig "github.com/vocdoni/go-dvote/crypto/signature_ecdsa"

	"strings"
	"time"

	"github.com/vocdoni/go-dvote/chain"
	"github.com/vocdoni/go-dvote/data"
	"github.com/vocdoni/go-dvote/net"
	"github.com/vocdoni/go-dvote/router"
	"github.com/vocdoni/go-dvote/types"
	"github.com/vocdoni/go-dvote/config"
	"github.com/vocdoni/go-dvote/log"
)

func newConfig() (config.GWCfg, error) {
	//setup flags
	path := flag.String("cfgpath", "./", "cfgpath. Specify filepath for gateway config file")

	flag.Bool("fileApi", true, "enable file API")
	flag.Bool("web3Api", true, "enable web3 API")

	flag.String("dvoteHost", "0.0.0.0", "dvote API host")
	flag.Int("dvotePort", 9090, "dvote API port")
	flag.String("dvoteRoute", "/dvote", "dvote API route")

	flag.Bool("allowPrivate", false, "allows authorized clients to call private methods")
	flag.String("allowedAddrs", "", "comma delimited list of allowed client eth addresses")
	flag.String("signingKey", "", "request signing key for this node")

	flag.String("chain", "vctestnet", "Blockchain to connect")
	flag.Int("w3wsPort", 0, "websockets port")
	flag.String("w3wsHost", "0.0.0.0", "ws host to listen on")
	flag.Int("w3httpPort", 9091, "http endpoint port, disabled if 0")
	flag.String("w3httpHost", "0.0.0.0", "http host to listen on")
	flag.Int("w3nodePort", 32000, "node port")

	flag.String("ipfsDaemon", "ipfs", "ipfs daemon path")
	flag.Bool("ipfsNoInit", false, "do not start ipfs daemon (if already started)")

	flag.String("sslDomain", "", "ssl secure domain")
	flag.String("sslDirCert", "./", "path where the ssl files will be stored")

	flag.String("loglevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")

	flag.Parse()
	viper := viper.New()
	var globalCfg config.GWCfg
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(*path) // path to look for the config file in
	viper.AddConfigPath(".")                      // optionally look for config in the working directory
	err := viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	viper.BindPFlag("api.fileApi", flag.Lookup("fileApi"))
	viper.BindPFlag("api.web3Api", flag.Lookup("web3Api"))
	viper.BindPFlag("dvote.host", flag.Lookup("dvoteHost"))
	viper.BindPFlag("dvote.port", flag.Lookup("dvotePort"))
	viper.BindPFlag("dvote.route", flag.Lookup("dvoteRoute"))
	viper.BindPFlag("client.allowPrivate", flag.Lookup("allowPrivate"))
	viper.BindPFlag("client.allowedAddrs", flag.Lookup("allowedAddrs"))
	viper.BindPFlag("client.signingKey", flag.Lookup("signingKey"))
	viper.BindPFlag("w3.chainType", flag.Lookup("chain"))
	viper.BindPFlag("w3.wsPort", flag.Lookup("w3wsPort"))
	viper.BindPFlag("w3.wsHost", flag.Lookup("w3wsHost"))
	viper.BindPFlag("w3.httpPort", flag.Lookup("w3httpPort"))
	viper.BindPFlag("w3.httpHost", flag.Lookup("w3httpHost"))
	viper.BindPFlag("w3.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("ipfs.daemon", flag.Lookup("ipfsDaemon"))
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ssl.domain", flag.Lookup("sslDomain"))
	viper.BindPFlag("ssl.dirCert", flag.Lookup("sslDirCert"))
	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

/*
Example code for using web3 implementation

Testing the RPC can be performed with curl and/or websocat
 curl -X POST -H "Content-Type:application/json" --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' localhost:9091
 echo '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":74}' | websocat ws://127.0.0.1:9092
*/
func main() {
	//setup config
	globalCfg, err := newConfig()
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	var node *chain.EthChainContext
	_ = node
	if globalCfg.Api.Web3Api {
		w3cfg, err := chain.NewConfig(globalCfg.W3)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		node, err = chain.Init(w3cfg)
		if err != nil {
			log.Panicf("Error: %v", err)
		}
		node.Start()
	}

	var signer *sig.SignKeys
	signer = new(sig.SignKeys)
	if globalCfg.Client.AllowPrivate && globalCfg.Client.AllowedAddrs != "" {
		keys := strings.Split(globalCfg.Client.AllowedAddrs, ",")
		for _, key := range keys {
			err := signer.AddAuthKey(key)
			if err != nil {
				log.Errorf("Error adding allowed key: %s", err)
			}
		}
	}

	if globalCfg.Client.SigningKey != "" {
		err := signer.AddHexKey(globalCfg.Client.SigningKey)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
	} else if globalCfg.Api.Web3Api {
		acc := node.Keys.Accounts()
		if len(acc) > 0 {
			keyJson, err := node.Keys.Export(acc[0], "", "")
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			err = signer.AddKeyFromEncryptedJSON(keyJson, "")
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
		}
	} else {
		err := signer.Generate()
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
	}

	listenerOutput := make(chan types.Message)

	if globalCfg.Api.FileApi {
		p := net.NewProxy()
		p.SSLDomain = globalCfg.Ssl.Domain
		p.SSLCertDir = globalCfg.Ssl.DirCert
		p.Address = globalCfg.Dvote.Host
		p.Port = globalCfg.Dvote.Port
		p.Init()
		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(p)
		ws.AddProxyHandler(globalCfg.Dvote.Route)

		ipfsConfig := data.IPFSNewConfig()
		ipfsConfig.Start = !globalCfg.Ipfs.NoInit
		ipfsConfig.Binary = globalCfg.Ipfs.Daemon
		storage, err := data.InitDefault(data.StorageIDFromString("IPFS"), ipfsConfig)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		go ws.Listen(listenerOutput)
		router := router.InitRouter(listenerOutput, storage, ws, *signer, globalCfg.Api.FileApi)
		go router.Route()

	}

	for {
		time.Sleep(1 * time.Second)
	}
}
