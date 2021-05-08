package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // for the pprof endpoints
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/vocdoni/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/chain"
	"go.vocdoni.io/dvote/chain/ethevents"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/keykeeper"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/vochaininfo"
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
	flag.StringVar(&globalCfg.VochainConfig.Chain, "vochain", "dev", "vocdoni blokchain network to connect with")
	flag.BoolVar(&globalCfg.Dev, "dev", false, "use developer mode (less security)")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.LogErrorFile = *flag.String("logErrorFile", "", "Log errors and warnings to a file")
	globalCfg.SaveConfig = *flag.Bool("saveConfig", false, "overwrites an existing config file with the CLI provided flags")
	// TODO(mvdan): turn this into an enum to avoid human error
	globalCfg.Mode = *flag.String("mode", types.ModeGateway, "global operation mode. Available options: [gateway,web3,oracle,miner]")
	// api
	globalCfg.API.Websockets = *flag.Bool("apiws", true, "enable websockets transport for the API")
	globalCfg.API.HTTP = *flag.Bool("apihttp", true, "enable http transport for the API")
	globalCfg.API.File = *flag.Bool("fileApi", true, "enable the file API")
	globalCfg.API.Census = *flag.Bool("censusApi", false, "enable the census API")
	globalCfg.API.Vote = *flag.Bool("voteApi", true, "enable the vote API")
	globalCfg.API.Tendermint = *flag.Bool("tendermintApi", false, "make the Tendermint API public available")
	globalCfg.API.Results = *flag.Bool("resultsApi", true, "enable the results API")
	globalCfg.API.Indexer = *flag.Bool("indexerApi", false, "enable the indexer API (required for explorer)")
	globalCfg.API.Route = *flag.String("apiRoute", "/", "dvote API base route for HTTP and Websockets")
	globalCfg.API.AllowPrivate = *flag.Bool("apiAllowPrivate", false, "allows private methods over the APIs")
	globalCfg.API.AllowedAddrs = *flag.String("apiAllowedAddrs", "", "comma delimited list of allowed client ETH addresses for private methods")
	globalCfg.API.ListenHost = *flag.String("listenHost", "0.0.0.0", "API endpoint listen address")
	globalCfg.API.ListenPort = *flag.Int("listenPort", 9090, "API endpoint http port")
	globalCfg.API.WebsocketsReadLimit = *flag.Int64("apiWsReadLimit", types.Web3WsReadLimit, "dvote websocket API read size limit in bytes")
	// ssl
	globalCfg.API.Ssl.Domain = *flag.String("sslDomain", "", "enable TLS secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")
	// ethereum node
	globalCfg.EthConfig.SigningKey = *flag.String("ethSigningKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	globalCfg.EthConfig.ChainType = *flag.String("ethChain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	globalCfg.EthConfig.LightMode = *flag.Bool("ethChainLightMode", false, "synchronize Ethereum blockchain in light mode")
	globalCfg.EthConfig.NodePort = *flag.Int("ethNodePort", 30303, "Ethereum p2p node port to use")
	globalCfg.EthConfig.BootNodes = *flag.StringArray("ethBootNodes", []string{}, "Ethereum p2p custom bootstrap nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.TrustedPeers = *flag.StringArray("ethTrustedPeers", []string{}, "Ethereum p2p trusted peer nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.NoWaitSync = *flag.Bool("ethNoWaitSync", false, "do not wait for Ethereum to synchronize (for testing only)")
	// ethereum events
	globalCfg.EthEventConfig.CensusSync = *flag.Bool("ethCensusSync", true, "automatically import new census published on the smart contract")
	globalCfg.EthEventConfig.SubscribeOnly = *flag.Bool("ethSubscribeOnly", true, "only subscribe to new ethereum events (do not read past log)")
	// ethereum web3
	globalCfg.W3Config.W3External = *flag.String("w3External", "", "use an external web3 endpoint instead of the local one. Supported protocols: http(s)://, ws(s):// and IPC filepath")
	globalCfg.W3Config.Enabled = *flag.Bool("w3Enabled", false, "if true, Ethereum will be synced and a web3 public endpoint will be available")
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
	globalCfg.VochainConfig.Genesis = *flag.String("vochainGenesis", "", "use alternative genesis file for the vochain")
	globalCfg.VochainConfig.LogLevel = *flag.String("vochainLogLevel", "none", "tendermint node log level (error, info, debug, nonde)")
	globalCfg.VochainConfig.LogLevelMemPool = *flag.String("vochainLogLevelMemPool", "error", "tendermint mempool log level")
	globalCfg.VochainConfig.Peers = *flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	globalCfg.VochainConfig.Seeds = *flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.VochainConfig.MinerKey = *flag.String("vochainMinerKey", "", "user alternative vochain miner private key (hexstring[64])")
	globalCfg.VochainConfig.NodeKey = *flag.String("vochainNodeKey", "", "user alternative vochain private key (hexstring[64])")
	globalCfg.VochainConfig.NoWaitSync = *flag.Bool("vochainNoWaitSync", false, "do not wait for Vochain to synchronize (for testing only)")
	globalCfg.VochainConfig.SeedMode = *flag.Bool("vochainSeedMode", false, "act as a vochain seed node")
	globalCfg.VochainConfig.MempoolSize = *flag.Int("vochainMempoolSize", 20000, "vochain mempool size")
	globalCfg.VochainConfig.MinerTargetBlockTimeSeconds = *flag.Int("vochainBlockTime", 10, "vohain consensus block time target (in seconds)")
	globalCfg.VochainConfig.KeyKeeperIndex = *flag.Int8("keyKeeperIndex", 0, "if this node is a key keeper, use this index slot")
	globalCfg.VochainConfig.ImportPreviousCensus = *flag.Bool("importPreviousCensus", false, "if enabled the census downloader will import all existing census")
	globalCfg.VochainConfig.EthereumWhiteListAddrs = *flag.StringArray("ethereumWhiteListAddrs", []string{}, "list of allowed ethereum address for creating processes on the vochain (oracle mode only)")
	// metrics
	globalCfg.Metrics.Enabled = *flag.Bool("metricsEnabled", false, "enable prometheus metrics")
	globalCfg.Metrics.RefreshInterval = *flag.Int("metricsRefreshInterval", 5, "metrics refresh interval in seconds")

	flag.CommandLine.SortFlags = false
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
	viper.BindPFlag("vochain", flag.Lookup("vochain"))
	globalCfg.VochainConfig.Chain = viper.GetString("vochain")
	viper.BindPFlag("dev", flag.Lookup("dev"))
	globalCfg.Dev = viper.GetBool("dev")

	// Use different datadirs for non main chains
	if globalCfg.VochainConfig.Chain != "main" {
		globalCfg.DataDir += "/" + globalCfg.VochainConfig.Chain
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
	viper.BindPFlag("api.Websockets", flag.Lookup("apiws"))
	viper.BindPFlag("api.WebsocketsReadLimit", flag.Lookup("apiWsReadLimit"))
	viper.BindPFlag("api.Http", flag.Lookup("apihttp"))
	viper.BindPFlag("api.File", flag.Lookup("fileApi"))
	viper.BindPFlag("api.Census", flag.Lookup("censusApi"))
	viper.BindPFlag("api.Vote", flag.Lookup("voteApi"))
	viper.BindPFlag("api.Results", flag.Lookup("resultsApi"))
	viper.BindPFlag("api.Indexer", flag.Lookup("indexerApi"))
	viper.BindPFlag("api.Tendermint", flag.Lookup("tendermintApi"))
	viper.BindPFlag("api.Route", flag.Lookup("apiRoute"))
	viper.BindPFlag("api.AllowPrivate", flag.Lookup("apiAllowPrivate"))
	viper.BindPFlag("api.AllowedAddrs", flag.Lookup("apiAllowedAddrs"))
	viper.BindPFlag("api.ListenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("api.ListenPort", flag.Lookup("listenPort"))
	viper.Set("api.Ssl.DirCert", globalCfg.DataDir+"/tls")
	viper.BindPFlag("api.Ssl.Domain", flag.Lookup("sslDomain"))

	// ethereum node
	viper.Set("ethConfig.Datadir", globalCfg.DataDir+"/ethereum")
	viper.BindPFlag("ethConfig.SigningKey", flag.Lookup("ethSigningKey"))
	viper.BindPFlag("ethConfig.ChainType", flag.Lookup("ethChain"))
	viper.BindPFlag("ethConfig.LightMode", flag.Lookup("ethChainLightMode"))
	viper.BindPFlag("ethConfig.NodePort", flag.Lookup("ethNodePort"))
	viper.BindPFlag("ethConfig.BootNodes", flag.Lookup("ethBootNodes"))
	viper.BindPFlag("ethConfig.TrustedPeers", flag.Lookup("ethTrustedPeers"))
	viper.BindPFlag("ethConfig.NoWaitSync", flag.Lookup("ethNoWaitSync"))
	viper.BindPFlag("ethEventConfig.CensusSync", flag.Lookup("ethCensusSync"))
	viper.BindPFlag("ethEventConfig.SubscribeOnly", flag.Lookup("ethSubscribeOnly"))

	// ethereum web3
	viper.BindPFlag("w3Config.W3External", flag.Lookup("w3External"))
	viper.BindPFlag("w3Config.Route", flag.Lookup("w3Route"))
	viper.BindPFlag("w3Config.enabled", flag.Lookup("w3Enabled"))
	viper.BindPFlag("w3Config.RPCPort", flag.Lookup("w3RPCPort"))
	viper.BindPFlag("w3Config.RPCHost", flag.Lookup("w3RPCHost"))

	// ipfs
	viper.Set("ipfs.ConfigPath", globalCfg.DataDir+"/ipfs")
	viper.BindPFlag("ipfs.NoInit", flag.Lookup("ipfsNoInit"))
	viper.BindPFlag("ipfs.SyncKey", flag.Lookup("ipfsSyncKey"))
	viper.BindPFlag("ipfs.SyncPeers", flag.Lookup("ipfsSyncPeers"))

	// vochain
	viper.Set("vochainConfig.DataDir", globalCfg.DataDir+"/vochain")
	viper.Set("vochainConfig.Dev", globalCfg.Dev)
	viper.BindPFlag("vochainConfig.P2PListen", flag.Lookup("vochainP2PListen"))
	viper.BindPFlag("vochainConfig.PublicAddr", flag.Lookup("vochainPublicAddr"))
	viper.BindPFlag("vochainConfig.RPCListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("vochainConfig.LogLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochainConfig.LogLevelMemPool", flag.Lookup("vochainLogLevelMemPool"))
	viper.BindPFlag("vochainConfig.Peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.Seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.CreateGenesis", flag.Lookup("vochainCreateGenesis"))
	viper.BindPFlag("vochainConfig.Genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.MinerKey", flag.Lookup("vochainMinerKey"))
	viper.BindPFlag("vochainConfig.NodeKey", flag.Lookup("vochainNodeKey"))
	viper.BindPFlag("vochainConfig.NoWaitSync", flag.Lookup("vochainNoWaitSync"))
	viper.BindPFlag("vochainConfig.SeedMode", flag.Lookup("vochainSeedMode"))
	viper.BindPFlag("vochainConfig.MempoolSize", flag.Lookup("vochainMempoolSize"))
	viper.BindPFlag("vochainConfig.MinerTargetBlockTimeSeconds", flag.Lookup("vochainBlockTime"))
	viper.BindPFlag("vochainConfig.KeyKeeperIndex", flag.Lookup("keyKeeperIndex"))
	viper.BindPFlag("vochainConfig.ImportPreviousCensus", flag.Lookup("importPreviousCensus"))
	viper.BindPFlag("vochainConfig.EthereumWhiteListAddrs", flag.Lookup("ethereumWhiteListAddrs"))

	// metrics
	viper.BindPFlag("metrics.Enabled", flag.Lookup("metricsEnabled"))
	viper.BindPFlag("metrics.RefreshInterval", flag.Lookup("metricsRefreshInterval"))

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
		signer := ethereum.NewSignKeys()
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
	// Don't use the log package here, because we want to report the version
	// before loading the config. This is because something could go wrong
	// while loading the config, and because the logger isn't set up yet.
	// For the sake of including the version in the log, it's also included
	// in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni dvote version %q\n", internal.Version)

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

	// If dev enabled, expose debugging profiles under a port between 61000 and 61100.
	// We log what port is being used near the start of the logs, so it can
	// be easily grabbed. Start this before the rest of the node, since it
	// is helpful to debug if some other component hangs.
	if globalCfg.Dev {
		go func() {
			port := (time.Now().Unix() % 100) + 61000
			ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
			if err != nil {
				log.Fatal(err)
			}
			log.Warnf("started pprof http endpoints at http://%s/debug/pprof", ln.Addr())
			log.Error(http.Serve(ln, nil))
		}()
	}

	log.Infof("starting vocdoni dvote node version %q in %s mode", internal.Version, globalCfg.Mode)
	var err error
	var signer *ethereum.SignKeys
	var node *chain.EthChainContext
	var pxy *mhttp.Proxy
	var storage data.Storage
	var cm *census.Manager
	var vnode *vochain.BaseApplication
	var vinfo *vochaininfo.VochainInfo
	var sc *scrutinizer.Scrutinizer
	var kk *keykeeper.KeyKeeper
	var ma *metrics.Agent

	if globalCfg.Dev {
		log.Warn("developer mode is enabled, I hope you know what you are doing ;)")
	}
	if globalCfg.Mode == types.ModeGateway || globalCfg.Mode == types.ModeOracle || globalCfg.Mode == types.ModeWeb3 {
		// Signing key
		signer = ethereum.NewSignKeys()
		// Add Authorized keys for private methods
		if globalCfg.API.AllowPrivate && globalCfg.API.AllowedAddrs != "" {
			keys := strings.Split(globalCfg.API.AllowedAddrs, ",")
			for _, key := range keys {
				signer.AddAuthKey(ethcommon.HexToAddress(key))
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
	if globalCfg.Mode == types.ModeGateway || globalCfg.Mode == types.ModeWeb3 || globalCfg.Metrics.Enabled ||
		(globalCfg.Mode == types.ModeOracle && len(globalCfg.W3Config.W3External) > 0) {
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

	if globalCfg.Mode == types.ModeGateway {
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
	if (globalCfg.Mode == types.ModeGateway && globalCfg.W3Config.Enabled) ||
		globalCfg.Mode == types.ModeOracle || globalCfg.Mode == types.ModeWeb3 {
		node, err = service.Ethereum(globalCfg.EthConfig, globalCfg.W3Config, pxy, signer, ma)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Vochain and Scrutinizer service
	if !globalCfg.API.Vote && globalCfg.API.Census {
		log.Fatal("census API needs the vote API enabled")
	}
	if (globalCfg.Mode == types.ModeGateway && globalCfg.API.Vote) ||
		globalCfg.Mode == types.ModeMiner || globalCfg.Mode == types.ModeOracle {
		scrutinizer := (globalCfg.Mode == types.ModeGateway && globalCfg.API.Results) || (globalCfg.Mode == types.ModeOracle)
		vnode, sc, vinfo, err = service.Vochain(globalCfg.VochainConfig, scrutinizer, !globalCfg.VochainConfig.NoWaitSync, ma, cm)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			vnode.Node.Stop()
			vnode.Node.Wait()
		}()

		if globalCfg.Mode == types.ModeGateway && globalCfg.API.Tendermint {
			// Enable Tendermint RPC proxy endpoint on /tendermint
			tp := strings.Split(globalCfg.VochainConfig.RPCListen, ":")
			if len(tp) != 2 {
				log.Warnf("cannot get port from vochain RPC listen: %s", globalCfg.VochainConfig.RPCListen)
			} else {
				pxy.AddWsHandler("/tendermint", pxy.AddWsWsBridge("ws://127.0.0.1:"+tp[1]+"/websocket", types.VochainWsReadLimit), types.VochainWsReadLimit) // tendermint needs up to 20 MB
				log.Infof("tendermint API endpoint available at %s", "/tendermint")
			}
		}

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
	if globalCfg.Mode == types.ModeOracle && globalCfg.VochainConfig.KeyKeeperIndex > 0 {
		kk, err = keykeeper.NewKeyKeeper(globalCfg.VochainConfig.DataDir+"/keykeeper",
			vnode,
			signer,
			globalCfg.VochainConfig.KeyKeeperIndex)
		if err != nil {
			log.Fatal(err)
		}
		go kk.RevealUnpublished()
		go kk.PrintInfo(time.Second * 20)
	}

	if (globalCfg.Mode == types.ModeGateway && globalCfg.W3Config.Enabled) ||
		globalCfg.Mode == types.ModeOracle {
		// Wait for Ethereum to be ready
		if !globalCfg.EthConfig.NoWaitSync {
			requiredPeers := 2
			if len(globalCfg.W3Config.W3External) > 0 {
				requiredPeers = 1
			}
			for {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				if info, err := node.SyncInfo(ctx); err == nil &&
					info.Synced && info.Peers >= requiredPeers && info.Height > 0 {
					log.Infof("ethereum blockchain synchronized (%+v)", info)
					cancel()
					break
				}
				cancel()
				time.Sleep(time.Second * 5)
			}
		}

		// Ethereum events service (needs Ethereum synchronized)
		if !globalCfg.EthConfig.NoWaitSync && globalCfg.Mode == types.ModeOracle {
			var evh []ethevents.EventHandler
			var w3uri string
			switch {
			case globalCfg.W3Config.W3External == "":
				// If local ethereum node enabled, use the Go-Ethereum websockets endpoint
				w3uri = "ws://" + net.JoinHostPort(globalCfg.W3Config.RPCHost,
					fmt.Sprintf("%d", globalCfg.W3Config.RPCPort))
			case strings.HasPrefix(globalCfg.W3Config.W3External, "ws"):
				w3uri = globalCfg.W3Config.W3External
			case strings.HasSuffix(globalCfg.W3Config.W3External, "ipc"):
				w3uri = globalCfg.W3Config.W3External

			default:
				log.Fatal("web3 external must be websocket or IPC for event subscription")
			}

			if globalCfg.Mode == types.ModeOracle {
				evh = append(evh, ethevents.HandleVochainOracle)
			}

			var initBlock *int64
			if !globalCfg.EthEventConfig.SubscribeOnly {
				initBlock = new(int64)
				chainSpecs, err := chain.SpecsFor(globalCfg.EthConfig.ChainType)
				if err != nil {
					log.Warn("cannot get chain block to start looking for events, using 0")
					*initBlock = 0
				} else {
					*initBlock = chainSpecs.StartingBlock
				}
			}

			whiteListedAddr := []string{}
			for _, addr := range globalCfg.VochainConfig.EthereumWhiteListAddrs {
				if ethcommon.IsHexAddress(addr) {
					whiteListedAddr = append(whiteListedAddr, addr)
				}
			}
			if len(whiteListedAddr) > 0 {
				log.Infof("ethereum whitelisted addresses %+v", whiteListedAddr)
			}
			if err := service.EthEvents(
				context.Background(),
				w3uri,
				globalCfg.EthConfig.ChainType,
				initBlock,
				cm,
				signer,
				vnode,
				evh,
				sc,
				whiteListedAddr); err != nil {
				log.Fatal(err)
			}
		}
	}

	if globalCfg.Mode == types.ModeGateway {
		// dvote API service
		if globalCfg.API.File || globalCfg.API.Census || globalCfg.API.Vote {
			if err := service.API(globalCfg.API,
				pxy,
				storage,
				cm, vnode,
				sc,
				vinfo,
				globalCfg.VochainConfig.RPCListen,
				signer,
				ma); err != nil {
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
