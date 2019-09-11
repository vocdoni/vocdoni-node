package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	//abcicli "github.com/tendermint/tendermint/abci/client"
	dbm "github.com/tendermint/tm-db"

	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"

	"encoding/hex"
	goneturl "net/url"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"time"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"
	"gitlab.com/vocdoni/go-dvote/types"
	vochain "gitlab.com/vocdoni/go-dvote/vochain"
	oracle "gitlab.com/vocdoni/go-dvote/vochain/oracle"
)

func newConfig() (config.GWCfg, error) {
	var globalCfg config.GWCfg
	//setup flags
	usr, err := user.Current()
	if err != nil {
		return globalCfg, err
	}
	userDir := usr.HomeDir + "/.dvote"
	path := flag.String("cfgpath", userDir+"/config.yaml", "filepath for custom gateway config")
	flag.Bool("fileApi", true, "enable file API")
	flag.Bool("censusApi", true, "enable census API")
	flag.Bool("voteApi", true, "enable vote API")
	flag.Bool("web3Api", true, "enable web3 API")
	flag.String("listenHost", "0.0.0.0", "API endpoint listen address")
	flag.Int("listenPort", 9090, "API endpoint http port")
	flag.String("apiRoute", "/dvote", "dvote API route")
	flag.Bool("allowPrivate", false, "allows private methods over the APIs")
	flag.String("allowedAddrs", "", "comma delimited list of allowed client ETH addresses for private methods")
	flag.String("signingKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	flag.String("chain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	flag.Bool("chainLightMode", false, "synchronize Ethereum blockchain in light mode")
	flag.Int("w3nodePort", 30303, "Ethereum p2p node port to use")
	flag.String("w3route", "/web3", "web3 endpoint API route")
	flag.String("w3external", "", "use external WEB3 endpoint. Local Ethereum node won't be initialized.")
	flag.Bool("ipfsNoInit", false, "disables inter planetary file system support")
	flag.String("ipfsClusterKey", "", "enables ipfs cluster using a shared key")
	flag.StringSlice("ipfsClusterPeers", []string{}, "ipfs cluster peer bootstrap addresses in multiaddr format")
	flag.String("sslDomain", "", "enable SSL secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")
	flag.String("dataDir", userDir, "directory where data is stored")
	flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	flag.String("clusterLogLevel", "ERROR", "Log level for ipfs cluster (debug, info, warning, error)")
	flag.String("vochainConfigFilePath", "/home/jordi/vocdoni/go-dvote/vochain/config/config.toml", "vochain config file path")
	flag.String("vochainAppDBName", "vochaindb", "sets the name of the application database for the vochain")
	flag.String("vochainAppDBPath", "/home/jordi/vocdoni/go-dvote/vochain/data", "sets the path indicating where to store the vochain related data")

	flag.Parse()

	viper := viper.New()
	viper.SetDefault("listenHost", "0.0.0.0")
	viper.SetDefault("listenPort", 9090)
	viper.SetDefault("api.file.enabled", true)
	viper.SetDefault("api.census.enabled", true)
	viper.SetDefault("api.vote.enabled", true)
	viper.SetDefault("api.route", "/dvote")
	viper.SetDefault("client.allowPrivate", false)
	viper.SetDefault("client.allowedAddrs", "")
	viper.SetDefault("client.signingKey", "")
	viper.SetDefault("w3.enabled", true)
	viper.SetDefault("w3.chainType", "goerli")
	viper.SetDefault("w3.lightMode", false)
	viper.SetDefault("w3.nodePort", 32000)
	viper.SetDefault("w3.route", "/web3")
	viper.SetDefault("w3.external", "")
	viper.SetDefault("w3.httpPort", "9091")
	viper.SetDefault("w3.httpHost", "127.0.0.1")
	viper.SetDefault("ssl.domain", "")
	viper.SetDefault("dataDir", userDir)
	viper.SetDefault("logLevel", "warn")
	viper.SetDefault("ipfs.noInit", false)
	viper.SetDefault("ipfs.configPath", userDir+"/.ipfs")
	viper.SetDefault("cluster.secret", "")
	viper.SetDefault("cluster.bootstraps", []string{})
	viper.SetDefault("cluster.stats", false)
	viper.SetDefault("cluster.tracing", false)
	viper.SetDefault("cluster.consensus", "raft")
	viper.SetDefault("cluster.pintracker", "map")
	viper.SetDefault("cluster.leave", true)
	viper.SetDefault("cluster.alloc", "disk")
	viper.SetDefault("cluster.clusterLogLevel", "ERROR")
	viper.SetDefault("vochain.configFilePath", "/home/jordi/vocdoni/go-dvote/vochain/config/config.toml")
	viper.SetDefault("vochain.appDBName", "vochaindb")
	viper.SetDefault("vochain.appDBPath", "/home/jordi/vocdoni/go-dvote/vochain/data")

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
	viper.BindPFlag("api.file.enabled", flag.Lookup("fileApi"))
	viper.BindPFlag("api.census.enabled", flag.Lookup("censusApi"))
	viper.BindPFlag("api.vote.enabled", flag.Lookup("voteApi"))
	viper.BindPFlag("api.route", flag.Lookup("apiRoute"))
	viper.BindPFlag("listenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("listenPort", flag.Lookup("listenPort"))
	viper.BindPFlag("client.allowPrivate", flag.Lookup("allowPrivate"))
	viper.BindPFlag("client.allowedAddrs", flag.Lookup("allowedAddrs"))
	viper.BindPFlag("client.signingKey", flag.Lookup("signingKey"))
	viper.BindPFlag("w3.enabled", flag.Lookup("web3Api"))
	viper.BindPFlag("w3.chainType", flag.Lookup("chain"))
	viper.BindPFlag("w3.lightMode", flag.Lookup("chainLightMode"))
	viper.BindPFlag("w3.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("w3.route", flag.Lookup("w3route"))
	viper.BindPFlag("w3external", flag.Lookup("w3external"))
	viper.BindPFlag("ssl.domain", flag.Lookup("sslDomain"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("cluster.secret", flag.Lookup("ipfsClusterKey"))
	viper.BindPFlag("cluster.bootstraps", flag.Lookup("ipfsClusterPeers"))
	viper.BindPFlag("cluster.clusterloglevel", flag.Lookup("clusterLogLevel"))

	viper.BindPFlag("vochain.configFilePath", flag.Lookup("vochainConfigFilePath"))
	viper.BindPFlag("vochain.appDBName", flag.Lookup("vochainAppDBName"))
	viper.BindPFlag("vochain.appDBPath", flag.Lookup("vochainAppDBPath"))

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

func main() {
	//setup config
	globalCfg, err := newConfig()
	log.Infof("using datadir %s", globalCfg.DataDir)
	globalCfg.Ipfs.ConfigPath = globalCfg.DataDir + "/ipfs"

	//setup logger
	log.InitLoggerAtLevel(globalCfg.LogLevel)
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	//setup listener
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
	// Add Authorized keys for private methods
	if globalCfg.Client.AllowPrivate && globalCfg.Client.AllowedAddrs != "" {
		keys := strings.Split(globalCfg.Client.AllowedAddrs, ",")
		for _, key := range keys {
			err := signer.AddAuthKey(key)
			if err != nil {
				log.Error(err.Error())
			}
		}
	}

	// Set Ethereum node context
	globalCfg.W3.DataDir = globalCfg.DataDir
	w3cfg, err := chain.NewConfig(globalCfg.W3)
	if err != nil {
		log.Fatal(err.Error())
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		log.Panic(err.Error())
	}

	// Add signing private key if exist in configuration or flags
	if globalCfg.Client.SigningKey != "" {
		log.Infof("adding custom signing key")
		err := signer.AddHexKey(globalCfg.Client.SigningKey)
		if err != nil {
			log.Fatalf("Fatal error adding hex key: %v", err.Error())
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
		os.RemoveAll(globalCfg.DataDir + "/.keyStore.tmp")
		node.Keys = keystore.NewPlaintextKeyStore(globalCfg.DataDir + "/.keyStore.tmp")
		node.Keys.ImportECDSA(signer.Private, "")
	} else {
		// Get stored keys from Ethereum node context
		acc := node.Keys.Accounts()
		if len(acc) > 0 {
			keyJSON, err := node.Keys.Export(acc[0], "", "")
			if err != nil {
				log.Fatal(err.Error())
			}
			err = addKeyFromEncryptedJSON(keyJSON, "", signer)
			pub, _ := signer.HexString()
			log.Infof("using pubKey %s from keystore", pub)
			if err != nil {
				log.Fatalf(err.Error())
			}
		}
	}

	// Start Ethereum Web3 native node
	if globalCfg.W3.Enabled && len(globalCfg.W3external) == 0 {
		log.Info("starting Ethereum node")
		node.Start()
		for i := 0; i < len(node.Keys.Accounts()); i++ {
			log.Debugf("got ethereum address: %x", node.Keys.Accounts()[i].Address)
		}
		time.Sleep(1 * time.Second)
		log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
		pxy.AddHandler(globalCfg.W3.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
		log.Infof("web3 available at %s", globalCfg.W3.Route)
		go func() {
			for {
				time.Sleep(60 * time.Second)
				if &node.Eth != nil {
					log.Debugf("[ethereum info] peers:%d synced:%t block:%s",
						node.Node.Server().PeerCount(),
						node.Eth.Synced(),
						node.Eth.BlockChain().CurrentBlock().Number())
				}
			}
		}()
	}

	if globalCfg.W3.Enabled && len(globalCfg.W3external) > 0 {
		url, err := goneturl.Parse(globalCfg.W3external)
		if err != nil {
			log.Fatalf("cannot parse w3external URL")
		}

		log.Debugf("testing web3 endpoint %s", url.String())
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
			log.Fatal("cannot connect to web3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err.Error())
		}
		log.Infof("successfuly connected to web3 endpoint at external url: %s", globalCfg.W3external)
		log.Infof("web3 available at %s", globalCfg.W3.Route)
	}

	// Storage
	var storage data.Storage
	if !globalCfg.Ipfs.NoInit {
		ipfsStore := data.IPFSNewConfig(globalCfg.Ipfs.ConfigPath)
		ipfsStore.ClusterCfg = globalCfg.Cluster
		if len(globalCfg.Cluster.Secret) > 0 {
			if len(globalCfg.Cluster.Secret) > 16 {
				log.Fatal("ipfsClusterKey too long")
			}
			encodedSecret := hex.EncodeToString([]byte(globalCfg.Cluster.Secret))
			for i := len(encodedSecret); i < 32; i++ {
				encodedSecret += "0"
			}
			ipfsStore.ClusterCfg.Secret = encodedSecret
			log.Infof("ipfs cluster encoded secret: %s", encodedSecret)
		}
		storage, err = data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	var censusManager census.CensusManager

	if globalCfg.Api.Census.Enabled {
		log.Info("starting census manager")
		if _, err := os.Stat(globalCfg.DataDir + "/census"); os.IsNotExist(err) {
			err = os.MkdirAll(globalCfg.DataDir+"/census", os.ModePerm)
			if err != nil {
				log.Fatal(err.Error())
			}
		}
		censusManager.Init(globalCfg.DataDir+"/census", "")
	}

	// Vochain Initialization

	// app layer db
	db, err := dbm.NewGoLevelDBWithOpts(globalCfg.Vochain.AppDBName, globalCfg.Vochain.AppDBPath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// node + app layer
	app, vnode := vochain.Start(globalCfg.Vochain.ConfigFilePath, db)
	defer func() {
		vnode.Stop()
		vnode.Wait()
	}()

	oracle_eth_connection := node
	_ = oracle.NewOracle(oracle_eth_connection, app, &storage)
	//_ = orc.ReadEthereumEventLogs(0, 0, "")

	// API Initialization
	// Dvote API
	if globalCfg.Api.File.Enabled || globalCfg.Api.Census.Enabled || globalCfg.Api.Vote.Enabled {
		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(pxy)

		listenerOutput := make(chan types.Message)
		go ws.Listen(listenerOutput)

		routerApi := router.InitRouter(listenerOutput, storage, ws, *signer)
		if globalCfg.Api.File.Enabled {
			routerApi.EnableFileAPI()
		}
		if globalCfg.Api.Census.Enabled {
			routerApi.EnableCensusAPI(&censusManager)
		}
		if globalCfg.Api.Vote.Enabled {
			// todo: client params as cli flags
			routerApi.EnableVoteAPI(app)
		}

		go routerApi.Route()
		log.Debug("Setting up API router")
		ws.AddProxyHandler(globalCfg.Api.Route)
		log.Infof("websockets API available at %s", globalCfg.Api.Route)
	}

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)

	for {
		time.Sleep(1 * time.Second)
	}

}

/*



	// TESTING

	//n := 0
	//for n < 10 {
	//	n++
	time.Sleep(5 * time.Second)
	//txbytes := []byte(`{"method": "voteTx", "args": {"nullifier": "0", "payload": "0", "censusProof": "0"}}`)
	//txbytes := []byte(test.HARDCODED_NEW_VOTE_TX)
	//req := abci.RequestCheckTx{Tx: txbytes}
	//go vlog.Infof("%+v", app.CheckTx(req))
	//req2 := abci.RequestDeliverTx{Tx: txbytes}
	//time.Sleep(10 * time.Second)
	//time.Sleep(5 * time.Second)
	//go vlog.Infof("%+v", app.DeliverTx(req2))
	//txbytes2 := []byte(`{"method": "voteTx", "args": { "processId": "0", "nullifier": "0", "censusProof": "0", "payload": "0" }}`)
	//req3 := abci.RequestDeliverTx{Tx: txbytes}
	//go vlog.Infof("%+v", app.DeliverTx(req3))
	//}

	// END TESTING



*/
