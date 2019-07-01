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
	flag.String("loglevel", "info", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")
	flag.Parse()
	viper := viper.New()
	var globalCfg config.GWCfg
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/go-dvote/cmd/gateway/") // path to look for the config file in
	viper.AddConfigPath(".")                      // optionally look for config in the working directory
	err := viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	viper.BindPFlags(flag.CommandLine)
	
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
