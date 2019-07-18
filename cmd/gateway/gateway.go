package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"

	goneturl "net/url"
	"os"
	"os/user"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"
	"gitlab.com/vocdoni/go-dvote/types"
)

func newConfig() (config.GWCfg, error) {
	var globalCfg config.GWCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	userDir := usr.HomeDir + "/.dvote"
	path := flag.String("cfgpath", userDir+"/config.yaml", "cfgpath. Specify filepath for gateway config")
	flag.Bool("dvoteApi", true, "enable dvote API")
	flag.Bool("web3Api", true, "enable web3 API")
	flag.String("listenHost", "0.0.0.0", "http host endpoint")
	flag.Int("listenPort", 9090, "http port endpoint")
	flag.String("dvoteRoute", "/dvote", "dvote endpoint API route")
	flag.Bool("allowPrivate", false, "allows authorized clients to call private methods")
	flag.String("allowedAddrs", "", "comma delimited list of allowed client ETH addresses")
	flag.String("signingKey", "", "request signing key for this node")
	flag.String("chain", "goerli", "Ethereum blockchain to use")
	flag.Bool("chainLightMode", false, "synchronize Ethereum blockchain in light mode")
	flag.Int("w3nodePort", 32000, "node port")
	flag.String("w3route", "/web3", "web3 endpoint API route")
	flag.String("w3external", "", "use external WEB3 endpoint. Local Ethereum node won't be initialized.")
	flag.String("ipfsDaemon", "ipfs", "ipfs daemon path")
	flag.Bool("ipfsNoInit", false, "do not start ipfs daemon (if already started)")
	flag.String("sslDomain", "", "enable SSL secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")
	flag.String("dataDir", userDir, "directory where data is stored")
	flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")

	flag.Parse()

	viper := viper.New()
	viper.SetDefault("api.dvoteApi", true)
	viper.SetDefault("api.web3Api", true)
	viper.SetDefault("listenHost", "0.0.0.0")
	viper.SetDefault("listenPort", 9090)
	viper.SetDefault("dvote.route", "/dvote")
	viper.SetDefault("client.allowPrivate", false)
	viper.SetDefault("client.allowedAddrs", "")
	viper.SetDefault("client.signingKey", "")
	viper.SetDefault("w3.chainType", "goerli")
	viper.SetDefault("w3.lightMode", false)
	viper.SetDefault("w3.nodePort", 32000)
	viper.SetDefault("w3.route", "/web3")
	viper.SetDefault("w3.external", "")
	viper.SetDefault("w3.httpPort", "9091")
	viper.SetDefault("w3.httpHost", "127.0.0.1")
	viper.SetDefault("ipfs.daemon", "ipfs")
	viper.SetDefault("ipfs.noInit", false)
	viper.SetDefault("ssl.domain", "")
	viper.SetDefault("dataDir", userDir)
	viper.SetDefault("logLevel", "warn")

	viper.SetConfigType("yaml")
	if *path == userDir+"/config.yaml" { //if path left default, write new cfg file if empty or if file doesn't exist.
		if err = viper.SafeWriteConfigAs(*path); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(userDir, os.ModePerm)
				if err != nil {
					return globalCfg, err
				}
				err = viper.WriteConfigAs(*path)
				if err != nil {
					return globalCfg, err
				}
			}
		}
	}

	//bind flags after writing default config so flag use
	//does not write config on first program run
	viper.BindPFlag("api.dvoteApi", flag.Lookup("dvoteApi"))
	viper.BindPFlag("api.web3Api", flag.Lookup("web3Api"))
	viper.BindPFlag("listenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("listenPort", flag.Lookup("listenPort"))
	viper.BindPFlag("dvote.route", flag.Lookup("dvoteRoute"))
	viper.BindPFlag("client.allowPrivate", flag.Lookup("allowPrivate"))
	viper.BindPFlag("client.allowedAddrs", flag.Lookup("allowedAddrs"))
	viper.BindPFlag("client.signingKey", flag.Lookup("signingKey"))
	viper.BindPFlag("w3.chainType", flag.Lookup("chain"))
	viper.BindPFlag("w3.lightMode", flag.Lookup("chainLightMode"))
	viper.BindPFlag("w3.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("w3.route", flag.Lookup("w3route"))
	viper.BindPFlag("w3external", flag.Lookup("w3external"))
	viper.BindPFlag("ipfs.daemon", flag.Lookup("ipfsDaemon"))
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ssl.domain", flag.Lookup("sslDomain"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))

	viper.SetConfigFile(*path)
	err = viper.ReadInConfig()
	if err != nil {
		return globalCfg, err
	}

	err = viper.Unmarshal(&globalCfg)
	return globalCfg, err
}

func addKeyFromEncryptedJSON(keyJSON []byte, passphrase string, signKeys *sig.SignKeys) error {
	key, err := keystore.DecryptKey(keyJSON, passphrase)
	if err != nil {
		return err
	}
	signKeys.Private = key.PrivateKey
	signKeys.Public = &key.PrivateKey.PublicKey
	return nil
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
	log.Infof("using datadir %s", globalCfg.DataDir)
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	var node *chain.EthChainContext
	pxy := net.NewProxy()
	pxy.C.SSLDomain = globalCfg.Ssl.Domain
	pxy.C.SSLCertDir = globalCfg.DataDir
	log.Infof("storing SSL certificate in %s", pxy.C.SSLCertDir)
	pxy.C.Address = globalCfg.ListenHost
	pxy.C.Port = globalCfg.ListenPort
	err = pxy.Init()
	if err != nil {
		log.Warn("letsEncrypt SSL certificate cannot be obtained, probably port 443 is not accessible or domain provided is not correct")
		log.Info("disabling SSL!")
		// Probably SSL has failed
		pxy.C.SSLDomain = ""
		globalCfg.Ssl.Domain = ""
		err = pxy.Init()
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	// Signing key
	var signer *sig.SignKeys
	signer = new(sig.SignKeys)
	if globalCfg.Client.AllowPrivate && globalCfg.Client.AllowedAddrs != "" {
		keys := strings.Split(globalCfg.Client.AllowedAddrs, ",")
		for _, key := range keys {
			err := signer.AddAuthKey(key)
			if err != nil {
				log.Error(err.Error())
			}
		}
	}

	if globalCfg.Client.SigningKey != "" {
		err := signer.AddHexKey(globalCfg.Client.SigningKey)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		err := signer.Generate()
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	// Web3 and chain
	globalCfg.W3.DataDir = globalCfg.DataDir
	if globalCfg.Api.Web3Api && globalCfg.W3external == "" {
		w3cfg, err := chain.NewConfig(globalCfg.W3)
		if err != nil {
			log.Fatal(err.Error())
		}
		node, err = chain.Init(w3cfg)
		if err != nil {
			log.Panic(err.Error())
		}
		node.Start()
		time.Sleep(1 * time.Second)
		pxy.AddHandler(globalCfg.W3.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
		log.Infof("web3 available at %s", globalCfg.W3.Route)

		acc := node.Keys.Accounts()
		if len(acc) > 0 {
			keyJSON, err := node.Keys.Export(acc[0], "", "")
			if err != nil {
				log.Fatal(err.Error())
			}
			err = addKeyFromEncryptedJSON(keyJSON, "", signer)
			pub, _ := signer.HexString()
			log.Infof("added pubKey %s from keystore", pub)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
		}
	}

	if globalCfg.Api.Web3Api && globalCfg.W3external != "" {
		url, err := goneturl.Parse(globalCfg.W3external)
		if err != nil {
			log.Fatalf("cannot parse w3external URL")
		}

		log.Debugf("%s", fmt.Sprintf("%s", url.String()))
		pxy.AddHandler(globalCfg.W3.Route, pxy.AddEndpoint(url.String()))
		data, err := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "net_peerCount",
			"id":      74,
			"params":  []interface{}{},
		})
		if err != nil {
			log.Fatal(err.Error())
		}
		resp, err := http.Post(globalCfg.W3external,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatal("cannot connect to w3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err.Error())
		}
		log.Infof("successfuly connected to w3 endpoint at external url: %s", globalCfg.W3external)
		log.Infof("web3 available at %s", globalCfg.W3.Route)
	}

	listenerOutput := make(chan types.Message)

	if globalCfg.Api.DvoteApi {
		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(pxy)
		ws.AddProxyHandler(globalCfg.Dvote.Route)
		log.Infof("ws file api available at %s", globalCfg.Dvote.Route)

		ipfsConfig := data.IPFSNewConfig()
		ipfsConfig.Start = !globalCfg.Ipfs.NoInit
		ipfsConfig.Binary = globalCfg.Ipfs.Daemon
		storage, err := data.InitDefault(data.StorageIDFromString("IPFS"), ipfsConfig)
		if err != nil {
			log.Fatalf(err.Error())
		}

		go ws.Listen(listenerOutput)
		router := router.InitRouter(listenerOutput, storage, ws, *signer, globalCfg.Api.DvoteApi)
		go router.Route()

	}

	for {
		time.Sleep(1 * time.Second)
	}
}
