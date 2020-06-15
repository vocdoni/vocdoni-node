package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/config"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/metrics"
	vnet "gitlab.com/vocdoni/go-dvote/net"
	"gitlab.com/vocdoni/go-dvote/service"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/keykeeper"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
	"gitlab.com/vocdoni/go-dvote/vochain/vochaininfo"
)

func newConfig() (*config.DvoteCfg, config.Error) {
	var err error
	var cfgError config.Error
	// create base config
	globalCfg := config.NewConfig()
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
	// Booleans should be passed to the CLI as: var=True/false

	// global
	flag.StringVar(&globalCfg.DataDir, "dataDir", home+"/.dvote", "directory where data is stored")
	flag.BoolVar(&globalCfg.Dev, "dev", false, "run and connect to the development network")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.LogErrorFile = *flag.String("logErrorFile", "", "Log errors and warnings to a file")
	globalCfg.SaveConfig = *flag.Bool("saveConfig", false, "overwrites an existing config file with the CLI provided flags")
	// TODO(mvdan): turn this into an enum to avoid human error
	globalCfg.Mode = *flag.String("mode", "gateway", "global operation mode. Available options: [gateway,web3,oracle,miner]")
	// api
	globalCfg.API.File = *flag.Bool("fileApi", true, "enable file API")
	globalCfg.API.Census = *flag.Bool("censusApi", true, "enable census API")
	globalCfg.API.Vote = *flag.Bool("voteApi", true, "enable vote API")
	globalCfg.API.Results = *flag.Bool("resultsApi", true, "enable results API")
	globalCfg.API.Route = *flag.String("apiRoute", "/dvote", "dvote API route")
	globalCfg.API.AllowPrivate = *flag.Bool("apiAllowPrivate", false, "allows private methods over the APIs")
	globalCfg.API.AllowedAddrs = *flag.String("apiAllowedAddrs", "", "comma delimited list of allowed client ETH addresses for private methods")
	globalCfg.API.ListenHost = *flag.String("listenHost", "0.0.0.0", "API endpoint listen address")
	globalCfg.API.ListenPort = *flag.Int("listenPort", 9090, "API endpoint http port")
	// ssl
	globalCfg.API.Ssl.Domain = *flag.String("sslDomain", "", "enable TLS secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")
	// ethereum node
	globalCfg.EthConfig.SigningKey = *flag.String("ethSigningKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	globalCfg.EthConfig.ChainType = *flag.String("ethChain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	globalCfg.EthConfig.LightMode = *flag.Bool("ethChainLightMode", false, "synchronize Ethereum blockchain in light mode")
	globalCfg.EthConfig.NodePort = *flag.Int("ethNodePort", 30303, "Ethereum p2p node port to use")
	globalCfg.EthConfig.BootNodes = *flag.StringArray("ethBootNodes", []string{}, "Ethereum p2p custom bootstrap nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.TrustedPeers = *flag.StringArray("ethTrustedPeers", []string{}, "Ethereum p2p trusted peer nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.ProcessDomain = *flag.String("ethProcessDomain", "voting-process.vocdoni.eth", "voting contract ENS domain")
	globalCfg.EthConfig.NoWaitSync = *flag.Bool("ethNoWaitSync", false, "do not wait for Ethereum to synchronize (for testing only)")
	// ethereum events
	globalCfg.EthEventConfig.CensusSync = *flag.Bool("ethCensusSync", true, "automatically import new census published on the smart contract")
	globalCfg.EthEventConfig.SubscribeOnly = *flag.Bool("ethSubscribeOnly", false, "only subscribe to new ethereum events (do not read past log)")
	// ethereum web3
	globalCfg.W3Config.W3External = *flag.String("w3External", "", "use an external web3 endpoint (if set, local Ethereum node will not be started)")
	globalCfg.W3Config.Enabled = *flag.Bool("w3Enabled", true, "if true web3 will be enabled")
	globalCfg.W3Config.Route = *flag.String("w3Route", "/web3", "web3 endpoint API route")
	globalCfg.W3Config.RPCPort = *flag.Int("w3RPCPort", 9091, "web3 RPC port")
	globalCfg.W3Config.RPCHost = *flag.String("w3RPCHost", "127.0.0.1", "web3 RPC host")

	// ipfs
	globalCfg.Ipfs.NoInit = *flag.Bool("ipfsNoInit", false, "disables inter planetary file system support")
	globalCfg.Ipfs.SyncKey = *flag.String("ipfsSyncKey", "", "enable IPFS cluster synchronization using the given secret key")
	globalCfg.Ipfs.SyncPeers = *flag.StringArray("ipfsSyncPeers", []string{}, "use custom ipfsSync peers/bootnodes for accessing the DHT")
	// vochain
	globalCfg.VochainConfig.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656", "p2p host and port to listent for the voting chain")
	globalCfg.VochainConfig.PublicAddr = *flag.String("vochainPublicAddr", "", "external addrress:port to announce to other peers (automatically guessed if empty)")
	globalCfg.VochainConfig.RPCListen = *flag.String("vochainRPCListen", "127.0.0.1:26657", "rpc host and port to listen for the voting chain")
	globalCfg.VochainConfig.CreateGenesis = *flag.Bool("vochainCreateGenesis", false, "create own/testing genesis file on vochain")
	globalCfg.VochainConfig.Genesis = *flag.String("vochainGenesis", "", "use alternative genesis file for the voting chain")
	globalCfg.VochainConfig.LogLevel = *flag.String("vochainLogLevel", "error", "voting chain node log level")
	globalCfg.VochainConfig.Peers = *flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	globalCfg.VochainConfig.Seeds = *flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.VochainConfig.MinerKey = *flag.String("vochainMinerKey", "", "user alternative vochain miner private key (hexstring[64])")
	globalCfg.VochainConfig.NodeKey = *flag.String("vochainNodeKey", "", "user alternative vochain private key (hexstring[64])")
	globalCfg.VochainConfig.NoWaitSync = *flag.Bool("vochainNoWaitSync", false, "do not wait for Vochain to synchronize (for testing only)")
	globalCfg.VochainConfig.SeedMode = *flag.Bool("vochainSeedMode", false, "act as a vochain seed node")
	globalCfg.VochainConfig.MempoolSize = *flag.Int("vochainMempoolSize", 200000, "vochain mempool size")
	globalCfg.VochainConfig.KeyKeeperIndex = *flag.Int8("keyKeeperIndex", 0, "if this node is a key keeper, use this index slot")
	globalCfg.VochainConfig.ImportPreviousCensus = *flag.Bool("importPreviousCensus", false, "if enabled the census downloader will import all existing census")
	// metrics
	globalCfg.Metrics.Enabled = *flag.Bool("metricsEnabled", false, "enable prometheus metrics")
	globalCfg.Metrics.RefreshInterval = *flag.Int("metricsRefreshInterval", 5, "metrics refresh interval in seconds")

	// parse flags
	flag.Parse()

	// setting up viper
	viper := viper.New()
	viper.SetConfigName("dvote")
	viper.SetConfigType("yml")
	viper.SetEnvPrefix("DVOTE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set FlagVars first
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	globalCfg.DataDir = viper.GetString("dataDir")
	viper.BindPFlag("dev", flag.Lookup("dev"))
	globalCfg.Dev = viper.GetBool("dev")

	// If dev enabled, modify dataDir
	if globalCfg.Dev {
		globalCfg.DataDir += "/dev"
	}
	// Add viper config path (now we know it)
	viper.AddConfigPath(globalCfg.DataDir)

	// binding flags to viper
	// global
	viper.BindPFlag("mode", flag.Lookup("mode"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logErrorFile", flag.Lookup("logErrorFile"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("saveConfig", flag.Lookup("saveConfig"))

	// api
	viper.BindPFlag("api.file", flag.Lookup("fileApi"))
	viper.BindPFlag("api.census", flag.Lookup("censusApi"))
	viper.BindPFlag("api.vote", flag.Lookup("voteApi"))
	viper.BindPFlag("api.results", flag.Lookup("resultsApi"))
	viper.BindPFlag("api.route", flag.Lookup("apiRoute"))
	viper.BindPFlag("api.allowPrivate", flag.Lookup("apiAllowPrivate"))
	viper.BindPFlag("api.allowedAddrs", flag.Lookup("apiAllowedAddrs"))
	viper.BindPFlag("api.listenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("api.listenPort", flag.Lookup("listenPort"))
	viper.Set("api.ssl.dirCert", globalCfg.DataDir+"/tls")
	viper.BindPFlag("api.ssl.domain", flag.Lookup("sslDomain"))

	// ethereum node
	viper.Set("ethConfig.datadir", globalCfg.DataDir+"/ethereum")
	viper.BindPFlag("ethConfig.signingKey", flag.Lookup("ethSigningKey"))
	viper.BindPFlag("ethConfig.chainType", flag.Lookup("ethChain"))
	viper.BindPFlag("ethConfig.lightMode", flag.Lookup("ethChainLightMode"))
	viper.BindPFlag("ethConfig.nodePort", flag.Lookup("ethNodePort"))
	viper.BindPFlag("ethConfig.bootNodes", flag.Lookup("ethBootNodes"))
	viper.BindPFlag("ethConfig.trustedPeers", flag.Lookup("ethTrustedPeers"))
	viper.BindPFlag("ethConfig.processDomain", flag.Lookup("ethProcessDomain"))
	viper.BindPFlag("ethConfig.noWaitSync", flag.Lookup("ethNoWaitSync"))
	viper.BindPFlag("ethEventConfig.censusSync", flag.Lookup("ethCensusSync"))
	viper.BindPFlag("ethEventConfig.subscribeOnly", flag.Lookup("ethSubscribeOnly"))

	// ethereum web3
	viper.BindPFlag("w3Config.w3External", flag.Lookup("w3External"))
	viper.BindPFlag("w3Config.route", flag.Lookup("w3Route"))
	viper.BindPFlag("w3Config.enabled", flag.Lookup("w3Enabled"))
	viper.BindPFlag("w3Config.RPCPort", flag.Lookup("w3RPCPort"))
	viper.BindPFlag("w3Config.RPCHost", flag.Lookup("w3RPCHost"))

	// ipfs
	viper.Set("ipfs.configPath", globalCfg.DataDir+"/ipfs")
	viper.BindPFlag("ipfs.noInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ipfs.syncKey", flag.Lookup("ipfsSyncKey"))
	viper.BindPFlag("ipfs.syncPeers", flag.Lookup("ipfsSyncPeers"))

	// vochain
	viper.Set("vochainConfig.dataDir", globalCfg.DataDir+"/vochain")
	viper.BindPFlag("vochainConfig.p2pListen", flag.Lookup("vochainP2PListen"))
	viper.BindPFlag("vochainConfig.publicAddr", flag.Lookup("vochainPublicAddr"))
	viper.BindPFlag("vochainConfig.rpcListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("vochainConfig.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochainConfig.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.createGenesis", flag.Lookup("vochainCreateGenesis"))
	viper.BindPFlag("vochainConfig.genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.MinerKey", flag.Lookup("vochainMinerKey"))
	viper.BindPFlag("vochainConfig.NodeKey", flag.Lookup("vochainNodeKey"))
	viper.BindPFlag("vochainConfig.NoWaitSync", flag.Lookup("vochainNoWaitSync"))
	viper.BindPFlag("vochainConfig.seedMode", flag.Lookup("vochainSeedMode"))
	viper.BindPFlag("vochainConfig.Dev", flag.Lookup("dev"))
	viper.BindPFlag("vochainConfig.MempoolSize", flag.Lookup("vochainMempoolSize"))
	viper.BindPFlag("vochainConfig.KeyKeeperIndex", flag.Lookup("keyKeeperIndex"))
	viper.BindPFlag("vochainConfig.ImportPreviousCensus", flag.Lookup("importPreviousCensus"))

	// metrics
	viper.BindPFlag("metrics.enabled", flag.Lookup("metricsEnabled"))
	viper.BindPFlag("metrics.refreshInterval", flag.Lookup("metricsRefreshInterval"))

	// check if config file exists
	_, err = os.Stat(globalCfg.DataDir + "/dvote.yml")
	if os.IsNotExist(err) {
		cfgError = config.Error{
			Message: fmt.Sprintf("creating new config file in %s", globalCfg.DataDir),
		}
		// creting config folder if not exists
		err = os.MkdirAll(globalCfg.DataDir, os.ModePerm)
		if err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot create data directory: %s", err),
			}
		}
		// create config file if not exists
		if err := viper.SafeWriteConfig(); err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot write config file into config dir: %s", err),
			}
		}
	} else {
		// read config file
		err = viper.ReadInConfig()
		if err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot read loaded config file in %s: %s", globalCfg.DataDir, err),
			}
		}
	}
	err = viper.Unmarshal(&globalCfg)
	if err != nil {
		cfgError = config.Error{
			Message: fmt.Sprintf("cannot unmarshal loaded config file: %s", err),
		}
	}

	if len(globalCfg.EthConfig.SigningKey) < 32 {
		fmt.Println("no signing key, generating one...")
		var signer ethereum.SignKeys
		err = signer.Generate()
		if err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot generate signing key: %s", err),
			}
			return globalCfg, cfgError
		}
		_, priv := signer.HexString()
		viper.Set("ethConfig.signingKey", priv)
		globalCfg.EthConfig.SigningKey = priv
		globalCfg.SaveConfig = true
	}

	if globalCfg.SaveConfig {
		viper.Set("saveConfig", false)
		if err := viper.WriteConfig(); err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot overwrite config file into config dir: %s", err),
			}
		}
	}

	return globalCfg, cfgError
}

/*
  Gateway needs:
	- signing key
	- ethereum
	- ipfs storage
	- proxy
	- api
	- ethevents
	- census
	- vochain
	- scrutinizer
	- metrics

  Oracle needs:
	- signing key
	- ethereum
	- ethevents
	- vochain

  Miner needs:
	- vochain
*/

func main() {
	// setup config
	// creating config and init logger
	globalCfg, cfgErr := newConfig()
	if globalCfg == nil {
		log.Fatal("cannot read configuration")
	}
	log.Init(globalCfg.LogLevel, globalCfg.LogOutput)
	if path := globalCfg.LogErrorFile; path != "" {
		if err := log.SetFileErrorLog(path); err != nil {
			log.Fatal(err)
		}
	}
	log.Debugf("initializing config %+v", *globalCfg)

	// using dev mode
	if globalCfg.Dev {
		log.Infof("using development mode, datadir %s", globalCfg.DataDir)
	}

	// check if errors during config creation and determine if Critical
	if cfgErr.Critical && cfgErr.Message != "" {
		log.Fatalf("critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message != "" {
		log.Warnf("non Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message == "" {
		log.Infof("config file loaded successfully, remember CLI flags have preference")
	}

	// Ensure we can have at least 8k open files. This is necessary, since
	// many components like IPFS and Tendermint require keeping many active
	// connections. Some systems have low defaults like 1024, which can make
	// the program crash after it's been running for a bit.
	if err := ensureNumberFiles(8000); err != nil {
		log.Errorf("could not ensure we can have enough open files: %v", err)
	}

	// Check the mode is valid
	if !globalCfg.ValidMode() {
		log.Fatalf("mode %s is invalid", globalCfg.Mode)
	}

	log.Infof("starting vocdoni dvote node in %s mode", globalCfg.Mode)
	var err error
	var signer *ethereum.SignKeys
	var node *chain.EthChainContext
	var pxy *vnet.Proxy
	var storage data.Storage
	var cm *census.Manager
	var vnode *vochain.BaseApplication
	var vinfo *vochaininfo.VochainInfo
	var sc *scrutinizer.Scrutinizer
	var kk *keykeeper.KeyKeeper
	var ma *metrics.Agent

	if globalCfg.Mode == "gateway" || globalCfg.Mode == "oracle" || globalCfg.Mode == "web3" {
		// Signing key
		signer = new(ethereum.SignKeys)
		// Add Authorized keys for private methods
		if globalCfg.API.AllowPrivate && globalCfg.API.AllowedAddrs != "" {
			keys := strings.Split(globalCfg.API.AllowedAddrs, ",")
			for _, key := range keys {
				err := signer.AddAuthKey(key)
				if err != nil {
					log.Error(err)
				}
			}
		}

		// Add signing private key if exist in configuration or flags
		if len(globalCfg.EthConfig.SigningKey) != 32 {
			log.Infof("adding custom signing key")
			err := signer.AddHexKey(globalCfg.EthConfig.SigningKey)
			if err != nil {
				log.Fatalf("error adding hex key: (%s)", err)
			}
			pub, _ := signer.HexString()
			log.Infof("using custom pubKey %s", pub)
		} else {
			log.Fatal("no private key or wrong key (size != 16 bytes)")
		}
	}

	// Websockets and HTTPs proxy
	if globalCfg.Mode == "gateway" || globalCfg.Mode == "web3" || globalCfg.Metrics.Enabled ||
		(globalCfg.Mode == "oracle" && len(globalCfg.W3Config.W3External) > 0) {
		// Proxy service
		pxy, err = service.Proxy(globalCfg.API.ListenHost, globalCfg.API.ListenPort,
			globalCfg.API.Ssl.Domain, globalCfg.API.Ssl.DirCert)
		if err != nil {
			log.Fatal(err)
		}

		// Enable metrics via proxy
		if globalCfg.Metrics.Enabled && pxy != nil {
			ma = metrics.NewAgent("/metrics", time.Duration(globalCfg.Metrics.RefreshInterval)*time.Second, pxy)
		}
	}

	if globalCfg.Mode == "gateway" {
		// Storage service
		storage, err = service.IPFS(globalCfg.Ipfs, signer, ma)
		if err != nil {
			log.Fatal(err)
		}

		// Census service
		if globalCfg.API.Census {
			cm, err = service.Census(globalCfg.DataDir, ma)
			if err != nil {
				log.Fatal(err)
			}
			cm.RemoteStorage = storage
		}
	}

	// Ethereum service
	if (globalCfg.Mode == "gateway" && globalCfg.W3Config.Enabled) || globalCfg.Mode == "oracle" || globalCfg.Mode == "web3" {
		node, err = service.Ethereum(globalCfg.EthConfig, globalCfg.W3Config, pxy, signer, ma)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Vochain and Scrutinizer service
	if !globalCfg.API.Vote && globalCfg.API.Census {
		log.Fatal("census API needs the vote API enabled")
	}
	if (globalCfg.Mode == "gateway" && globalCfg.API.Vote) || globalCfg.Mode == "miner" || globalCfg.Mode == "oracle" {
		scrutinizer := (globalCfg.Mode == "gateway" && globalCfg.API.Results)
		vnode, sc, vinfo, err = service.Vochain(globalCfg.VochainConfig, globalCfg.Dev, scrutinizer, !globalCfg.VochainConfig.NoWaitSync, ma, cm)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			vnode.Node.Stop()
			vnode.Node.Wait()
		}()

		// Wait for Vochain to be ready
		var h, hPrev int64
		for vnode.Node == nil {
			hPrev = h
			time.Sleep(time.Second * 5)
			if header := vnode.State.Header(true); header != nil {
				h = header.Height
			}
			log.Infof("[vochain info] replaying block %d at %d b/s",
				h, (h-hPrev)/5)
		}
	}

	// Start keykeeper service
	if globalCfg.Mode == "oracle" && globalCfg.VochainConfig.KeyKeeperIndex > 0 {
		kk, err = keykeeper.NewKeyKeeper(globalCfg.VochainConfig.DataDir+"/keykeeper", vnode, signer, globalCfg.VochainConfig.KeyKeeperIndex)
		if err != nil {
			log.Fatal(err)
		}
		go kk.RevealUnpublished()
		go kk.PrintInfo(time.Second * 20)
	}

	if (globalCfg.Mode == "gateway" && globalCfg.W3Config.Enabled) || globalCfg.Mode == "oracle" {
		// Wait for Ethereum to be ready
		if !globalCfg.EthConfig.NoWaitSync {
			requiredPeers := 2
			if len(globalCfg.W3Config.W3External) > 0 {
				requiredPeers = 1
			}
			for {
				if info, err := node.SyncInfo(); err == nil && info.Synced && info.Peers >= requiredPeers && info.Height > 0 {
					log.Infof("ethereum blockchain synchronized (%+v)", info)
					break
				}
				time.Sleep(time.Second * 5)
			}
		}

		// Ethereum events service (needs Ethereum synchronized)
		if !globalCfg.EthConfig.NoWaitSync && globalCfg.Mode == "oracle" {
			var evh []ethevents.EventHandler
			var w3uri string
			if globalCfg.W3Config.W3External != "" {
				// If w3 external is enabled, use the local websockets proxy
				if !strings.HasPrefix(globalCfg.W3Config.W3External, "ws") {
					log.Fatal("web3 external muts be websocket for event subscription")
				}
				prefix := "ws"
				if globalCfg.API.Ssl.Domain != "" {
					prefix = "wss"
				}
				w3uri = fmt.Sprintf("%s://127.0.0.1:%d%s", prefix, globalCfg.API.ListenPort, globalCfg.W3Config.Route+"ws")
			} else {
				// If local ethereum node enabled, use the Go-Ethereum websockets endpoint
				w3uri = "ws://" + net.JoinHostPort(globalCfg.W3Config.RPCHost, fmt.Sprintf("%d", globalCfg.W3Config.RPCPort))
			}
			if globalCfg.Mode == "oracle" {
				evh = append(evh, ethevents.HandleVochainOracle)
			}
			initBlock := int64(0)
			chainSpecs, err := chain.SpecsFor(globalCfg.EthConfig.ChainType)
			if err != nil {
				log.Warn("cannot get chain block to start looking for events, using 0")
			} else {
				initBlock = chainSpecs.StartingBlock
			}
			syncInfo, err := node.SyncInfo()
			if err != nil {
				log.Fatal(err)
			}

			// Register the event handlers
			if globalCfg.EthEventConfig.SubscribeOnly {
				syncInfo.Height = 0
			}
			if err := service.EthEvents(globalCfg.EthConfig.ProcessDomain, w3uri, initBlock, int64(syncInfo.Height), cm, signer, vnode, evh); err != nil {
				log.Fatal(err)
			}
		}
	}

	if globalCfg.Mode == "gateway" {
		// dvote API service
		if globalCfg.API.File || globalCfg.API.Census || globalCfg.API.Vote {
			if err := service.API(globalCfg.API, pxy, storage, cm, vnode, sc, vinfo, globalCfg.VochainConfig.RPCListen, signer, ma); err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Info("startup complete")

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warnf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
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
