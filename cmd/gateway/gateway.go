package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

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

	flag.Bool("fileApi", true, "enable file API")
	flag.Bool("web3Api", true, "enable web3 API")

	flag.String("listenHost", "0.0.0.0", "http host endpoint")
	flag.Int("listenPort", 9090, "http port endpoint")

	flag.String("dvoteRoute", "/dvote", "dvote API route")

	flag.Bool("allowPrivate", false, "allows authorized clients to call private methods")
	flag.String("allowedAddrs", "", "comma delimited list of allowed client eth addresses")
	flag.String("signingKey", "", "request signing key for this node")

	flag.String("chain", "vctestnet", "Blockchain to connect")
	flag.Int("w3nodePort", 32000, "node port")
	flag.String("w3Route", "/web3", "proxy endpoint exposing web3")
	flag.String("w3External", "", "URL where the gateway will be connected for WEB3 related. Local blockchain node won't be initialized.")

	flag.String("ipfsDaemon", "ipfs", "ipfs daemon path")
	flag.Bool("ipfsNoInit", false, "do not start ipfs daemon (if already started)")

	flag.String("sslDomain", "", "ssl secure domain")
	flag.String("dataDir", userDir, "directory where data is stored")

	flag.String("loglevel", "warn", "Log level. Valid values are: debug, info, warn, error, dpanic, panic, fatal.")

	flag.Parse()

	viper := viper.New()
	viper.SetDefault("api.fileApi", true)
	viper.SetDefault("api.web3Api", true)
	viper.SetDefault("listenHost", "0.0.0.0")
	viper.SetDefault("listenPort", 9090)
	viper.SetDefault("dvote.route", "/dvote")
	viper.SetDefault("client.allowPrivate", false)
	viper.SetDefault("client.allowedAddrs", "")
	viper.SetDefault("client.signingKey", "")
	viper.SetDefault("w3.chainType", "goerli")
	viper.SetDefault("w3.nodePort", 32000)
	viper.SetDefault("w3.route", "/web3")
	viper.SetDefault("w3External", "")
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
	viper.BindPFlag("api.fileApi", flag.Lookup("fileApi"))
	viper.BindPFlag("api.web3Api", flag.Lookup("web3Api"))
	viper.BindPFlag("listenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("listenPort", flag.Lookup("listenPort"))
	viper.BindPFlag("dvote.route", flag.Lookup("dvoteRoute"))
	viper.BindPFlag("client.allowPrivate", flag.Lookup("allowPrivate"))
	viper.BindPFlag("client.allowedAddrs", flag.Lookup("allowedAddrs"))
	viper.BindPFlag("client.signingKey", flag.Lookup("signingKey"))
	viper.BindPFlag("w3.chainType", flag.Lookup("chain"))
	viper.BindPFlag("w3.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("w3.route", flag.Lookup("w3Route"))
	viper.BindPFlag("w3External", flag.Lookup("w3External"))
	viper.BindPFlag("ipfs.daemon", flag.Lookup("ipfsDaemon"))
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ssl.domain", flag.Lookup("sslDomain"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("loglevel"))

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
	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	var node *chain.EthChainContext
	_ = node

	p := net.NewProxy()
	p.C.SSLDomain = globalCfg.Ssl.Domain
	p.C.SSLCertDir = globalCfg.DataDir
	log.Infof("Storing SSL certificate in %s", p.C.SSLCertDir)
	p.C.Address = globalCfg.ListenHost
	p.C.Port = globalCfg.ListenPort
	err = p.Init()
	if err != nil {
		log.Warn("LetsEncrypt SSL certificate cannot be obtained, probably port 443 is not accessible or domain provided is not correct")
		log.Warn("Disabling SSL!")
		// Probably SSL has failed
		p.C.SSLDomain = ""
		globalCfg.Ssl.Domain = ""
		err = p.Init()
		if err != nil {
			log.Fatal(err)
		}
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
	} else {
		err := signer.Generate()
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
	}

	if globalCfg.Api.Web3Api && globalCfg.W3External == "" {
		w3cfg, err := chain.NewConfig(globalCfg.W3)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		node, err = chain.Init(w3cfg)
		if err != nil {
			log.Panicf("Error: %v", err)
		}
		node.Start()
		time.Sleep(1 * time.Second)
		p.AddHandler(globalCfg.W3.Route, p.AddEndpoint(fmt.Sprintf("http://%s:%s", w3cfg.HTTPHost, strconv.Itoa(w3cfg.HTTPPort))))
		log.Infof("web3 available at %s", globalCfg.W3.Route)

		acc := node.Keys.Accounts()
		if len(acc) > 0 {
			keyJSON, err := node.Keys.Export(acc[0], "", "")
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			err = addKeyFromEncryptedJSON(keyJSON, "", signer)
			pub, _ := signer.HexString()
			log.Infof("Added pubKey %s from keystore", pub)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
		}
	}

	if globalCfg.Api.Web3Api && globalCfg.W3External != "" {
		u, err := goneturl.Parse(globalCfg.W3External)
		if err != nil {
			log.Fatalf("Cannot parse W3External URL")
		}

		log.Debugf("%s", fmt.Sprintf("%s", u.String()))
		p.AddHandler(globalCfg.W3.Route, p.AddEndpoint(u.String()))
		data, err := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "net_peerCount",
			"id":      74,
			"params":  []interface{}{},
		})
		if err != nil {
			log.Fatalf("Marshal: %v", err)
		}
		resp, err := http.Post(globalCfg.W3External,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatalf("Cannot connect to w3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("ReadAll: %v", err)
		}
		log.Infof("successfuly connected to w3 endpoint at external url: %s", globalCfg.W3External)
		log.Infof("web3 available at %s", globalCfg.W3.Route)
	}

	listenerOutput := make(chan types.Message)

	if globalCfg.Api.FileApi {
		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(p)
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
		router := router.InitRouter(listenerOutput, storage, ws, *signer, globalCfg.Api.FileApi)
		go router.Route()

	}

	for {
		time.Sleep(1 * time.Second)
	}
}
