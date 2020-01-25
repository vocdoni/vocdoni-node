package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	// abcicli "github.com/tendermint/tendermint/abci/client"

	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"

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
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

func newConfig() (*config.GWCfg, config.Error) {
	var err error
	var cfgError config.Error
	// create base config
	globalCfg := config.NewGatewayConfig()
	// get current user home dir
	home, err := os.UserHomeDir()
	if err != nil {
		cfgError = config.Error{
			Critical: true,
			Message:  fmt.Sprintf("cannot get user home directory with error: %s", err),
		}
		return nil, cfgError
	}

	// CLI flags will be used if something fails from this point
	// CLI flags have preference over the config file

	userDir := home + "/.dvote"
	// gateway
	globalCfg.DataDir = *flag.String("dataDir", userDir+"/gateway/", "directory where data is stored")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.ListenHost = *flag.String("listenHost", "0.0.0.0", "API endpoint listen address")
	globalCfg.ListenPort = *flag.Int("listenPort", 9090, "API endpoint http port")
	globalCfg.ConfigFilePath = *flag.String("configFilePath", userDir+"/gateway/", "path where the gateway config file is located")
	globalCfg.Contract = *flag.String("contract", "0x6f55bAE05cd2C88e792d4179C051359d02C6b34f", "smart contract to follow for synchronization and coordination with other nodes")
	globalCfg.CensusSync = *flag.Bool("censusSync", true, "automatically import new census published on smart contract")
	// api
	globalCfg.API.File = *flag.Bool("fileApi", true, "enable file API")
	globalCfg.API.Census = *flag.Bool("censusApi", true, "enable census API")
	globalCfg.API.Vote = *flag.Bool("voteApi", true, "enable vote API")
	globalCfg.API.Results = *flag.Bool("resultsApi", true, "enable results API")
	globalCfg.API.Route = *flag.String("apiRoute", "/dvote", "dvote API route")
	// ethereum client
	globalCfg.Client.AllowPrivate = *flag.Bool("allowPrivate", false, "allows private methods over the APIs")
	globalCfg.Client.AllowedAddrs = *flag.String("allowedAddrs", "", "comma delimited list of allowed client ETH addresses for private methods")
	globalCfg.Client.SigningKey = *flag.String("signingKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	// ethereum node
	globalCfg.W3.Enabled = *flag.Bool("w3enabled", true, "if true web3 will be enabled")
	globalCfg.W3.ChainType = *flag.String("w3chain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	globalCfg.W3.LightMode = *flag.Bool("w3chainLightMode", true, "synchronize Ethereum blockchain in light mode")
	globalCfg.W3.NodePort = *flag.Int("w3nodePort", 30303, "Ethereum p2p node port to use")
	globalCfg.W3.Route = *flag.String("w3route", "/web3", "web3 endpoint API route")
	globalCfg.W3.WsPort = *flag.Int("w3WsPort", 9092, "web3 websocket port")
	globalCfg.W3.WsHost = *flag.String("w3WsHost", "0.0.0.0", "web3 websocket host")
	globalCfg.W3.HTTPPort = *flag.Int("w3HTTPPort", 9091, "ethereum http server port")
	globalCfg.W3.HTTPHost = *flag.String("w3HTTPHost", "0.0.0.0", "ethereum http server host")
	globalCfg.W3.W3External = *flag.String("w3external", "", "use external WEB3 endpoint. Local Ethereum node won't be initialized.")
	globalCfg.W3.DataDir = *flag.String("w3dataDir", userDir, "sets the path indicating where to store the ethereum related data")
	// ipfs
	globalCfg.Ipfs.NoInit = *flag.Bool("ipfsNoInit", false, "disables inter planetary file system support")
	globalCfg.Ipfs.SyncKey = *flag.String("ipfsSyncKey", "", "enable IPFS cluster synchronization using the given secret key")
	globalCfg.Ipfs.SyncPeers = *flag.StringArray("ipfsSyncPeers", []string{}, "use custom ipfsSync peers/bootnodes for accessing the DHT")
	globalCfg.Ipfs.ConfigPath = *flag.String("ipfsConfigPath", userDir+"/ipfs/", "path indicating where to store ipfs related data")
	// ssl
	globalCfg.Ssl.Domain = *flag.String("sslDomain", "", "enable SSL secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")
	globalCfg.Ssl.DirCert = *flag.String("sslDirCert", userDir+"/tls/", "path where to store ssl related data")
	// vochain
	globalCfg.Vochain.DataDir = *flag.String("vochainDataDir", userDir+"/vochain/", "sets the path indicating where to store the Vochain related data")
	globalCfg.Vochain.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656", "p2p host and port to listent for the voting chain")
	globalCfg.Vochain.PublicAddr = *flag.String("vochainPublicAddr", "", "external addrress:port to announce to other peers (automatically guessed if empty)")
	globalCfg.Vochain.RPCListen = *flag.String("vochainRPCListen", "0.0.0.0:26657", "rpc host and port to listent for the voting chain")
	globalCfg.Vochain.CreateGenesis = *flag.Bool("vochainCreateGenesis", false, "create own/testing genesis file on vochain")
	globalCfg.Vochain.Genesis = *flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	globalCfg.Vochain.LogLevel = *flag.String("vochainLogLevel", "info", "voting chain node log level")
	globalCfg.Vochain.Peers = *flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	globalCfg.Vochain.Seeds = *flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.Vochain.KeyFile = *flag.String("vochainKeyFile", "", "user alternative vochain p2p node key file")
	// parse flags
	flag.Parse()
	// setting up viper
	viper := viper.New()
	viper.AddConfigPath(globalCfg.ConfigFilePath)
	viper.SetConfigName("gateway")
	viper.SetConfigType("yml")
	// binding flags to viper
	// gateway
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("listenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("listenPort", flag.Lookup("listenPort"))
	viper.BindPFlag("configFilePath", flag.Lookup("configFilePath"))
	viper.BindPFlag("censusSync", flag.Lookup("censusSync"))
	viper.BindPFlag("contract", flag.Lookup("contract"))
	// api
	viper.BindPFlag("api.file", flag.Lookup("fileApi"))
	viper.BindPFlag("api.census", flag.Lookup("censusApi"))
	viper.BindPFlag("api.vote", flag.Lookup("voteApi"))
	viper.BindPFlag("api.results", flag.Lookup("resultsApi"))
	viper.BindPFlag("api.route", flag.Lookup("apiRoute"))
	// ethereum client
	viper.BindPFlag("client.allowPrivate", flag.Lookup("allowPrivate"))
	viper.BindPFlag("client.allowedAddrs", flag.Lookup("allowedAddrs"))
	viper.BindPFlag("client.signingKey", flag.Lookup("signingKey"))
	// w3
	viper.BindPFlag("w3.enabled", flag.Lookup("w3enabled"))
	viper.BindPFlag("w3.chainType", flag.Lookup("w3chain"))
	viper.BindPFlag("w3.lightMode", flag.Lookup("w3chainLightMode"))
	viper.BindPFlag("w3.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("w3.route", flag.Lookup("w3route"))
	viper.BindPFlag("w3.wsPort", flag.Lookup("w3WsPort"))
	viper.BindPFlag("w3.wsHost", flag.Lookup("w3WsHost"))
	viper.BindPFlag("w3.httpPort", flag.Lookup("w3HTTPPort"))
	viper.BindPFlag("w3.httpHost", flag.Lookup("w3HTTPHost"))
	viper.BindPFlag("w3.w3external", flag.Lookup("w3external"))
	viper.BindPFlag("w3.dataDir", flag.Lookup("w3dataDir"))
	// ssl
	viper.BindPFlag("ssl.domain", flag.Lookup("sslDomain"))
	viper.BindPFlag("ssl.dirCert", flag.Lookup("sslDirCert"))
	// ipfs
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ipfs.syncKey", flag.Lookup("ipfsSyncKey"))
	viper.BindPFlag("ipfs.syncPeers", flag.Lookup("ipfsSyncPeers"))
	viper.BindPFlag("ipfs.configPath", flag.Lookup("ipfsConfigPath"))
	// vochain
	viper.BindPFlag("vochain.p2pListen", flag.Lookup("vochainP2PListen"))
	viper.BindPFlag("vochain.publicAddr", flag.Lookup("vochainPublicAddr"))
	viper.BindPFlag("vochain.rpcListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("vochain.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochain.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochain.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochain.createGenesis", flag.Lookup("vochainCreateGenesis"))
	viper.BindPFlag("vochain.genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochain.keyFile", flag.Lookup("vochainKeyFile"))
	viper.BindPFlag("vochain.dataDir", flag.Lookup("vochainDataDir"))

	// check if config file exists
	_, err = os.Stat(globalCfg.ConfigFilePath + "gateway.yml")
	if os.IsNotExist(err) {
		cfgError = config.Error{
			Critical: false,
			Message:  fmt.Sprintf("cannot read config file in: %s, with error: %s. A new one will be created", globalCfg.ConfigFilePath, err),
		}
		// creting config folder if not exists
		err = os.MkdirAll(globalCfg.ConfigFilePath, os.ModePerm)
		if err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot create config dir, with error: %s", err),
			}
		}
		// create config file if not exists
		if err = viper.SafeWriteConfig(); err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot write config file into config dir with error: %s", err),
			}
		}
	} else {
		// read config file
		err = viper.ReadInConfig()
		if err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot read loaded config file in: %s, with error: %s", globalCfg.ConfigFilePath, err),
			}
		}
		//err = viper.Unmarshal(&globalCfg)
		if err != nil {
			cfgError = config.Error{
				Critical: false,
				Message:  fmt.Sprintf("cannot unmarshal loaded config file in: %s, with error: %s", globalCfg.ConfigFilePath, err),
			}
		}
	}
	return globalCfg, cfgError
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
	// setup config
	// creating config and init logger
	globalCfg, cfgErr := newConfig()
	if globalCfg != nil {
		log.InitLogger(globalCfg.LogLevel, "stdout")
	}

	log.Infof("initializing vochain with tendermint config %+v", globalCfg)

	// check if errors during config creation and determine if Critical
	if cfgErr.Critical && cfgErr.Message != "" {
		log.Fatalf("Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message != "" {
		log.Warnf("non Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message == "" {
		log.Infof("config file loaded successfully, remember CLI flags have preference")
	}

	log.Infof("using datadir %s", globalCfg.DataDir)

	// Ensure we can have at least 8k open files. This is necessary, since
	// many components like IPFS and Tendermint require keeping many active
	// connections. Some systems have low defaults like 1024, which can make
	// the gateway crash after it's been running for a bit.
	if err := ensureNumberFiles(8000); err != nil {
		log.Fatalf("could not ensure we can have enough open files: %v", err)
	}

	// setup listener
	pxy := net.NewProxy()
	pxy.C.SSLDomain = globalCfg.Ssl.Domain
	pxy.C.SSLCertDir = globalCfg.Ssl.DirCert
	if globalCfg.Ssl.Domain != "" {
		log.Infof("storing SSL certificate in %s", pxy.C.SSLCertDir)
	}
	pxy.C.Address = globalCfg.ListenHost
	pxy.C.Port = globalCfg.ListenPort
	if _, err := pxy.Init(); err != nil {
		log.Fatal(err)
	}

	// Signing key
	signer := new(sig.SignKeys)
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
	w3cfg, err := chain.NewConfig(*globalCfg.W3)
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
		os.RemoveAll(globalCfg.W3.DataDir + "/.keyStore.tmp")
		node.Keys = keystore.NewPlaintextKeyStore(globalCfg.W3.DataDir + "/.keyStore.tmp")
		node.Keys.ImportECDSA(signer.Private, "")

	}

	// Start Ethereum Web3 node
	if globalCfg.W3.Enabled {
		log.Info("starting Ethereum node")
		node.Start()
		for i := 0; i < len(node.Keys.Accounts()); i++ {
			log.Debugf("got ethereum address: %x", node.Keys.Accounts()[i].Address)
		}
		if len(globalCfg.W3.W3External) == 0 {
			time.Sleep(1 * time.Second)
			log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
			pxy.AddHandler(globalCfg.W3.Route, pxy.AddEndpoint(fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort)))
			log.Infof("web3 available at %s", globalCfg.W3.Route)
		}
		go node.PrintInfo(time.Second * 15)
	}

	if globalCfg.W3.Enabled && len(globalCfg.W3.W3External) > 0 {
		// TO-DO create signing key since node.Start() is not executed and the ethereum account is not created on first run
		url, err := neturl.Parse(globalCfg.W3.W3External)
		if err != nil {
			log.Fatal("cannot parse w3external URL")
		}

		log.Debugf("testing web3 endpoint %s", url)
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
		resp, err := http.Post(globalCfg.W3.W3External,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatal("cannot connect to web3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("successfuly connected to web3 endpoint at external url: %s", globalCfg.W3.W3External)
		log.Infof("web3 available at %s", globalCfg.W3.Route)
	}

	if globalCfg.Client.SigningKey == "" {
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

	// Storage
	var storage data.Storage
	var storageSync ipfssync.IPFSsync
	if !globalCfg.Ipfs.NoInit {
		os.Setenv("IPFS_FD_MAX", "1024")
		ipfsStore := data.IPFSNewConfig(globalCfg.Ipfs.ConfigPath)
		storage, err = data.Init(data.StorageIDFromString("IPFS"), ipfsStore)
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			for {
				time.Sleep(time.Second * 20)
				stats, err := storage.Stats()
				if err != nil {
					log.Warnf("IPFS node returned an error: %s", err)
				}
				log.Infof("[ipfs info] %s", stats)
			}
		}()
		if len(globalCfg.Ipfs.SyncKey) > 0 {
			log.Info("enabling ipfs synchronization")
			storageSync = ipfssync.NewIPFSsync(globalCfg.Ipfs.ConfigPath+"/.ipfsSync", globalCfg.Ipfs.SyncKey, storage)
			if len(globalCfg.Ipfs.SyncPeers) > 0 {
				log.Debugf("using custom ipfs sync bootnodes %s", globalCfg.Ipfs.SyncPeers)
				storageSync.Transport.BootNodes = globalCfg.Ipfs.SyncPeers
			}
			go storageSync.Start()
		}
	}

	// Census Manager
	var censusManager census.Manager
	if globalCfg.API.Census {
		log.Info("starting census manager")
		if _, err := os.Stat(globalCfg.DataDir + "/census"); os.IsNotExist(err) {
			err = os.MkdirAll(globalCfg.DataDir+"/census", os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}
		censusManager.Init(globalCfg.DataDir+"/census", "")

	}

	// Initialize and start Vochain
	var vnode *vochain.BaseApplication
	var sc *scrutinizer.Scrutinizer

	if globalCfg.API.Vote {
		log.Info("initializing vochain")
		// node + app layer
		if len(globalCfg.Vochain.PublicAddr) == 0 {
			ip, err := util.PublicIP()
			if err != nil {
				log.Warn(err)
			} else {
				addrport := strings.Split(globalCfg.Vochain.P2PListen, ":")
				if len(addrport) > 0 {
					globalCfg.Vochain.PublicAddr = fmt.Sprintf("%s:%s", ip, addrport[len(addrport)-1])
				}
			}
		} else {
			addrport := strings.Split(globalCfg.Vochain.P2PListen, ":")
			if len(addrport) > 0 {
				globalCfg.Vochain.PublicAddr = fmt.Sprintf("%s:%s", addrport[0], addrport[1])
			}
		}
		if globalCfg.Vochain.PublicAddr != "" {
			log.Infof("public IP address: %s", globalCfg.Vochain.PublicAddr)
		}
		vnode = vochain.NewVochain(globalCfg.Vochain)
		if globalCfg.API.Results {
			log.Info("starting vochain scrutinizer")
			sc, err = scrutinizer.NewScrutinizer(globalCfg.DataDir+"/scrutinizer", vnode.State)
			if err != nil {
				log.Fatal(err)
			}
		}
		go func() {
			for {
				if vnode.Node != nil {
					log.Infof("[vochain info] height:%d mempool:%d appTree:%d processTree:%d voteTree:%d",
						vnode.Node.BlockStore().Height(),
						vnode.Node.Mempool().Size(),
						vnode.State.AppTree.Size(),
						vnode.State.ProcessTree.Size(),
						vnode.State.VoteTree.Size(),
					)
				}
				time.Sleep(20 * time.Second)
			}
		}()
		defer func() {
			vnode.Node.Stop()
			vnode.Node.Wait()
		}()
	}

	// Wait for Vochain to be ready
	if globalCfg.API.Vote {
		var h, hPrev int64
		for {
			if vnode.Node != nil {
				log.Infof("replay of vochain local blocks finished")
				break
			}
			hPrev = h
			time.Sleep(time.Second * 5)
			h = vnode.State.Height()
			log.Infof("[vochain info] replaying block %d at %d b/s",
				h, int64((h-hPrev)/5))
		}
	}

	// Wait for Ethereum to be ready
	if globalCfg.W3.Enabled {
		for {
			if _, synced, peers, _ := node.SyncInfo(); synced && peers > 1 {
				log.Infof("ethereum blockchain synchronized")
				break
			}
			time.Sleep(time.Second * 5)
		}
	}

	// API Endpoint initialization
	if globalCfg.API.File || globalCfg.API.Census || globalCfg.API.Vote {
		ws := new(net.WebsocketHandle)
		ws.Init(new(types.Connection))
		ws.SetProxy(pxy)

		listenerOutput := make(chan types.Message)
		go ws.Listen(listenerOutput)

		routerAPI := router.InitRouter(listenerOutput, storage, ws, signer)
		if globalCfg.API.File {
			log.Info("enabling file API")
			routerAPI.EnableFileAPI()
		}
		if globalCfg.API.Census {
			log.Info("enabling census API")
			routerAPI.EnableCensusAPI(&censusManager)
		}
		if globalCfg.API.Vote {
			// creating the RPC calls client
			rpcClient := voclient.NewHTTP(globalCfg.Vochain.RPCListen, "/websocket")
			// todo: client params as cli flags
			log.Info("enabling vote API")
			routerAPI.Scrutinizer = sc
			routerAPI.EnableVoteAPI(rpcClient)
		}

		go routerAPI.Route()
		ws.AddProxyHandler(globalCfg.API.Route)
		log.Infof("websockets API available at %s", globalCfg.API.Route)
		go func() {
			for {
				time.Sleep(60 * time.Second)
				log.Infof("[router info] privateReqs:%d publicReqs:%d", routerAPI.PrivateCalls, routerAPI.PublicCalls)
			}
		}()
	}
	log.Infof("gateway startup complete")

	// Census Oracle
	if globalCfg.CensusSync && globalCfg.API.Census {
		log.Infof("starting census import oracle")
		ev, err := ethevents.NewEthEvents(globalCfg.Contract, nil, fmt.Sprintf("ws://%s:%d", globalCfg.W3.WsHost, globalCfg.W3.WsPort), &censusManager)
		if err != nil {
			log.Fatalf("couldn't create ethereum events listener: %s", err)
		}
		ev.AddEventHandler(ethevents.HandleCensus)

		go func() {
			for _, synced, _, _ := node.SyncInfo(); !synced; {
				time.Sleep(time.Second * 2)
			}
			height, _, _, _ := node.SyncInfo()
			log.Infof("searching for census from block 0 to %s", height)
			ev.ReadEthereumEventLogs(0, util.Hex2int64(height))
			// Wait until having some peers
			for _, _, peers, _ := node.SyncInfo(); peers > 0; {
				time.Sleep(time.Second * 2)
			}
			log.Info("subscribing to new ethereum census")
			ev.SubscribeEthereumEventLogs()
		}()
	}

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)
}

func ensureNumberFiles(min uint64) error {
	// Note that this function should work on Unix-y systems, but not on
	// others like Windows.

	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		return err
	}
	if rlim.Max < min {
		return fmt.Errorf("hard limit is %d, but we require a minimum of %d", rlim.Max, min)
	}
	if rlim.Cur >= rlim.Max {
		return nil // nothing to do
	}
	log.Infof("raising file descriptor soft limit from %d to %d", rlim.Cur, rlim.Max)
	rlim.Cur = rlim.Max
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlim)
}
