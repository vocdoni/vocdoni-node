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
	tmnode "github.com/tendermint/tendermint/node"
	dbm "github.com/tendermint/tm-db"

	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"

	goneturl "net/url"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"time"

	voclient "github.com/tendermint/tendermint/rpc/client"
	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/ipfssync"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/router"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
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
	dataDir := flag.String("dataDir", userDir, "directory where data is stored")
	flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
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
	flag.String("ipfsSyncKey", "", "enable IPFS cluster synchronization using the given secret key")
	flag.StringArray("ipfsSyncPeers", []string{}, "use custom ipfsSync peers/bootnodes for accessing the DHT")
	flag.String("sslDomain", "", "enable SSL secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")

	flag.String("vochainListen", "0.0.0.0:26656", "p2p host and port to listent for the voting chain")
	flag.String("vochainAddress", "", "external addrress:port to announce to other peers (automatically guessed if empty)")
	flag.String("vochainRPClisten", "127.0.0.1:26657", "rpc host and port to listent for the voting chain")
	flag.Bool("vochainCreateGenesis", false, "create own/testing genesis file on vochain")
	flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	flag.String("vochainLogLevel", "error", "voting chain node log level")
	flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	flag.String("vochainContract", "0xb99F60f7a651589022c9495d3e555a46e3625A42", "voting smart contract where the oracle will listen")

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
	viper.SetDefault("logOutput", "stdout")
	viper.SetDefault("ipfs.noInit", false)
	viper.SetDefault("ipfs.configPath", userDir+"/.ipfs")
	viper.SetDefault("ipfs.syncKey", "")
	viper.SetDefault("ipfs.syncPeers", []string{})

	viper.SetDefault("vochain.p2pListen", "0.0.0.0:26656")
	viper.SetDefault("vochain.address", "")
	viper.SetDefault("vochain.rpcListen", "0.0.0.0:26657")
	viper.SetDefault("vochain.logLevel", "error")
	viper.SetDefault("vochain.createGenesis", false)
	viper.SetDefault("vochain.genesis", "")
	viper.SetDefault("vochain.peers", []string{})
	viper.SetDefault("vochain.seeds", []string{})
	viper.SetDefault("vochain.dataDir", *dataDir+"/vochain")
	viper.SetDefault("vochain.contract", "0xb99F60f7a651589022c9495d3e555a46e3625A42")

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
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ipfs.syncKey", flag.Lookup("ipfsSyncKey"))
	viper.BindPFlag("ipfs.syncPeers", flag.Lookup("ipfsSyncPeers"))

	viper.BindPFlag("vochain.p2pListen", flag.Lookup("vochainListen"))
	viper.BindPFlag("vochain.publicAddress", flag.Lookup("vochainAddress"))
	viper.BindPFlag("vochain.rpcListen", flag.Lookup("vochainRPClisten"))
	viper.BindPFlag("vochain.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochain.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochain.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochain.createGenesis", flag.Lookup("vochainCreateGenesis"))
	viper.BindPFlag("vochain.genesis", flag.Lookup("vochainGenesis"))
	viper.Set("vochain.dataDir", *dataDir+"/vochain")
	viper.BindPFlag("vochain.contract", flag.Lookup("vochainContract"))

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
		return err // Storage
	}
	signKeys.Private = key.PrivateKey
	signKeys.Public = &key.PrivateKey.PublicKey
	return nil
}

func main() {
	//setup config
	globalCfg, err := newConfig()
	globalCfg.Ipfs.ConfigPath = globalCfg.DataDir + "/ipfs"

	//setup logger
	log.InitLogger(globalCfg.LogLevel, globalCfg.LogOutput)
	if err != nil {
		log.Fatalf("could not load config: %s", err)
	}
	log.Infof("using datadir %s", globalCfg.DataDir)

	//setup listener
	pxy := net.NewProxy()
	pxy.C.SSLDomain = globalCfg.Ssl.Domain
	pxy.C.SSLCertDir = globalCfg.DataDir
	log.Infof("storing SSL certificate in %s", pxy.C.SSLCertDir)
	pxy.C.Address = globalCfg.ListenHost
	pxy.C.Port = globalCfg.ListenPort
	err = pxy.Init()
	if err != nil {
		// letsencrypt SSL has failed
		log.Warn("letsEncrypt SSL certificate cannot be obtained")
		log.Warn("probably port 443 is not accessible or domain provided is not correct")
		log.Info("disabling SSL")
		pxy.C.SSLDomain = ""
		globalCfg.Ssl.Domain = ""
		err = pxy.Init()
		if err != nil {
			log.Fatal(err)
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
				log.Error(err)
			}
		}
	}

	// Set Ethereum node context
	globalCfg.W3.DataDir = globalCfg.DataDir
	w3cfg, err := chain.NewConfig(globalCfg.W3)
	if err != nil {
		log.Fatal(err)
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		log.Panic(err)
	}

	// Add signing private key if exist in configuration or flags
	if globalCfg.Client.SigningKey != "" {
		log.Infof("adding custom signing key")
		err := signer.AddHexKey(globalCfg.Client.SigningKey)
		if err != nil {
			log.Fatalf("Fatal error adding hex key: %v", err)
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
				log.Fatal(err)
			}
			err = addKeyFromEncryptedJSON(keyJSON, "", signer)
			pub, _ := signer.HexString()
			log.Infof("using pubKey %s from keystore", pub)
			if err != nil {
				log.Fatal(err)
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
				time.Sleep(15 * time.Second)
				if node.Eth != nil {
					log.Infof("[ethereum info] peers:%d synced:%t block:%s",
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
			log.Fatal("cannot parse w3external URL")
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
			log.Fatal(err)
		}
		resp, err := http.Post(globalCfg.W3external,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatal("cannot connect to web3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("successfuly connected to web3 endpoint at external url: %s", globalCfg.W3external)
		log.Infof("web3 available at %s", globalCfg.W3.Route)
	}

	// Storage
	var storage data.Storage
	var storageSync ipfssync.IPFSsync
	if !globalCfg.Ipfs.NoInit {
		ipfsStore := data.IPFSNewConfig(globalCfg.Ipfs.ConfigPath)
		storage, err = data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			for {
				time.Sleep(time.Second * 60)
				stats, err := storage.Stats()
				if err != nil {
					log.Warnf("IPFS node returned an error: %s", err)
				}
				log.Infof("[ipfs info] %s", stats)
			}
		}()
		if len(globalCfg.Ipfs.SyncKey) > 0 {
			log.Info("enabling ipfs cluster synchronization")
			storageSync = ipfssync.NewIPFSsync(globalCfg.DataDir+"/.ipfsSync", globalCfg.Ipfs.SyncKey, storage)
			if len(globalCfg.Ipfs.SyncPeers) > 0 {
				log.Debugf("using custom ipfs sync bootnodes %s", globalCfg.Ipfs.SyncPeers)
				storageSync.Transport.BootNodes = globalCfg.Ipfs.SyncPeers
			}
			go storageSync.Start()
		}
	}

	// Census Manager
	var censusManager census.CensusManager
	if globalCfg.Api.Census.Enabled {
		log.Info("starting census manager")
		if _, err := os.Stat(globalCfg.DataDir + "/census"); os.IsNotExist(err) {
			err = os.MkdirAll(globalCfg.DataDir+"/census", os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}
		censusManager.Init(globalCfg.DataDir+"/census", "")

	}

	var vnode *tmnode.Node
	if globalCfg.Api.Vote.Enabled {
		log.Info("initializing vochain")
		// app layer db
		db, err := dbm.NewGoLevelDBWithOpts("vochain", globalCfg.Vochain.DataDir+"/data", nil)
		if err != nil {
			log.Fatalf("failed to open db: %s", err)
		}
		defer db.Close()

		// node + app layer
		if len(globalCfg.Vochain.PublicAddr) == 0 {
			ip, err := util.GetPublicIP()
			if err != nil || len(ip.String()) < 8 {
				log.Warnf("public IP discovery failed: %s", err)
			} else {
				addrport := strings.Split(globalCfg.Vochain.P2pListen, ":")
				if len(addrport) > 0 {
					globalCfg.Vochain.PublicAddr = fmt.Sprintf("%s:%s", ip.String(), addrport[len(addrport)-1])
				}
			}
		}
		if globalCfg.Vochain.PublicAddr != "" {
			log.Infof("public IP address: %s", globalCfg.Vochain.PublicAddr)
		}
		_, vnode = vochain.Start(globalCfg.Vochain, db)
		defer func() {
			vnode.Stop()
			vnode.Wait()
		}()

	}

	// API Endpoint initialization
	if globalCfg.Api.File.Enabled || globalCfg.Api.Census.Enabled || globalCfg.Api.Vote.Enabled {
		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(pxy)

		listenerOutput := make(chan types.Message)
		go ws.Listen(listenerOutput)

		routerAPI := router.InitRouter(listenerOutput, storage, ws, *signer)
		if globalCfg.Api.File.Enabled {
			log.Info("enabling file API")
			routerAPI.EnableFileAPI()
		}
		if globalCfg.Api.Census.Enabled {
			log.Info("enabling census API")
			routerAPI.EnableCensusAPI(&censusManager)
		}
		if globalCfg.Api.Vote.Enabled {
			// creating the RPC calls client
			rpcClient := voclient.NewHTTP(globalCfg.Vochain.RpcListen, "/websocket")
			// todo: client params as cli flags
			log.Info("enabling vote API")
			routerAPI.EnableVoteAPI(rpcClient)
		}

		go routerAPI.Route()
		ws.AddProxyHandler(globalCfg.Api.Route)
		log.Infof("websockets API available at %s", globalCfg.Api.Route)
		go func() {
			for {
				time.Sleep(60 * time.Second)
				log.Infof("[router info] privateReqs:%d publicReqs:%d", routerAPI.PrivateCalls, routerAPI.PublicCalls)
			}
		}()
	}

	log.Infof("Gateway startup complete")

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)
}
