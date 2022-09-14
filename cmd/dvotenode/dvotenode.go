package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // for the pprof endpoints
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"go.vocdoni.io/dvote/census"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/data"
	"go.vocdoni.io/dvote/db"
	ethchain "go.vocdoni.io/dvote/ethereum"
	"go.vocdoni.io/dvote/ethereum/ethevents"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/oracle"
	"go.vocdoni.io/dvote/oracle/apioracle"
	"go.vocdoni.io/dvote/rpcapi"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/urlapi"
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
	flag.StringVarP(&globalCfg.DataDir, "dataDir", "d", home+"/.dvote",
		"directory where data is stored")
	flag.StringVarP(&globalCfg.VochainConfig.DBType, "dbType", "t", db.TypePebble,
		fmt.Sprintf("key-value db type (%s, %s)", db.TypePebble, db.TypeBadger))
	flag.StringVarP(&globalCfg.VochainConfig.Chain, "vochain", "v", "dev",
		fmt.Sprintf("vocdoni blockchain to connect with: %q", vochain.GenesisAvailableChains()))
	flag.BoolVar(&globalCfg.Dev, "dev", false,
		"use developer mode (less security)")
	globalCfg.PprofPort = *flag.Int("pprof", 0,
		"pprof port for runtime profiling data (zero is disabled)")
	globalCfg.LogLevel = *flag.StringP("logLevel", "l", "info",
		"log level (debug, info, warn, error, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout",
		"log output (stdout, stderr or filepath)")
	globalCfg.LogErrorFile = *flag.String("logErrorFile", "",
		"log errors and warnings to a file")
	globalCfg.SaveConfig = *flag.Bool("saveConfig", false,
		"overwrite an existing config file with the provided CLI flags")
	// TODO(mvdan): turn this into an enum to avoid human error
	globalCfg.Mode = *flag.StringP("mode", "m", types.ModeGateway,
		"global operation mode. Available options: [gateway,oracle,ethApiOracle,miner,seed]")
	// api
	globalCfg.API.HTTP = *flag.Bool("apihttp", true, "enable http transport for the API")
	globalCfg.API.File = *flag.Bool("fileApi", true, "enable the file API")
	globalCfg.API.Census = *flag.Bool("censusApi", false, "enable the census API")
	globalCfg.API.Vote = *flag.Bool("voteApi", true, "enable the vote API")
	globalCfg.API.Results = *flag.Bool("resultsApi", true,
		"enable the results API")
	globalCfg.API.Indexer = *flag.Bool("indexerApi", false,
		"enable the indexer API (required for explorer)")
	globalCfg.API.URL = *flag.Bool("urlApi", false, "enable the url API")
	globalCfg.API.Route = *flag.String("apiRoute", "/",
		"dvote HTTP API base route")
	globalCfg.API.AllowPrivate = *flag.Bool("apiAllowPrivate", false,
		"allows private methods over the APIs")
	globalCfg.API.AllowedAddrs = *flag.String("apiAllowedAddrs", "",
		"comma-delimited list of allowed client ETH addresses for private methods")
	globalCfg.API.ListenHost = *flag.String("listenHost", "0.0.0.0",
		"API endpoint listen address")
	globalCfg.API.ListenPort = *flag.IntP("listenPort", "p", 9090,
		"API endpoint http port")
	// ssl
	globalCfg.API.Ssl.Domain = *flag.String("sslDomain", "",
		"enable TLS-secure domain with LetsEncrypt (listenPort=443 is required)")
	// ethereum node
	globalCfg.EthConfig.SigningKey = *flag.StringP("ethSigningKey", "k", "",
		"signing private Key (if not specified the Ethereum keystore will be used)")
	// ethereum web3
	globalCfg.W3Config.ChainType = *flag.StringP("ethChain", "c", "rinkeby",
		fmt.Sprintf("Ethereum blockchain to use: %s", ethchain.AvailableChains))
	globalCfg.W3Config.W3External = *flag.StringSliceP("w3External", "w", []string{},
		"comma-separated list of ethereum web3 endpoints. Supported protocols: http(s)://, ws(s):// and IPC filepath")
	// ipfs
	globalCfg.Ipfs.NoInit = *flag.Bool("ipfsNoInit", false,
		"disable inter planetary file system support")
	globalCfg.Ipfs.SyncKey = *flag.StringP("ipfsSyncKey", "i", "",
		"enable IPFS cluster synchronization using the given secret key")
	globalCfg.Ipfs.SyncPeers = *flag.StringSlice("ipfsSyncPeers", []string{},
		"use custom ipfsSync peers/bootnodes for accessing the DHT (comma-separated)")
	// vochain
	globalCfg.VochainConfig.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656",
		"p2p host and port to listent for the voting chain")
	globalCfg.VochainConfig.PublicAddr = *flag.String("vochainPublicAddr", "",
		"external address:port to announce to other peers (automatically guessed if empty)")
	globalCfg.VochainConfig.RPCListen = *flag.String("vochainRPCListen", "127.0.0.1:26657",
		"rpc host and port to listen to for the voting chain")
	globalCfg.VochainConfig.CreateGenesis = *flag.Bool("vochainCreateGenesis", false,
		"create local/testing genesis file for the vochain")
	globalCfg.VochainConfig.Genesis = *flag.String("vochainGenesis", "",
		"use alternative genesis file for the vochain")
	globalCfg.VochainConfig.LogLevel = *flag.String("vochainLogLevel", "none",
		"tendermint node log level (error, info, debug, none)")
	globalCfg.VochainConfig.Peers = *flag.StringSlice("vochainPeers", []string{},
		"comma-separated list of p2p peers")
	globalCfg.VochainConfig.Seeds = *flag.StringSlice("vochainSeeds", []string{},
		"comma-separated list of p2p seed nodes")
	globalCfg.VochainConfig.MinerKey = *flag.String("vochainMinerKey", "",
		"user alternative vochain miner private key (hexstring[64])")
	globalCfg.VochainConfig.NodeKey = *flag.String("vochainNodeKey", "",
		"user alternative vochain private key (hexstring[64])")
	globalCfg.VochainConfig.PrivValidatorListenAddr = *flag.String("vochainPrivValidator", "",
		"if defined, Tendermint node will open a port and wait for a remote private validator connection (example: tcp://0.0.0.0:26658)")
	globalCfg.VochainConfig.NoWaitSync = *flag.Bool("vochainNoWaitSync", false,
		"do not wait for Vochain to synchronize (for testing only)")
	globalCfg.VochainConfig.MempoolSize = *flag.Int("vochainMempoolSize", 20000,
		"vochain mempool size")
	globalCfg.VochainConfig.MinerTargetBlockTimeSeconds = *flag.Int("vochainBlockTime", 10,
		"vohain consensus block time target (in seconds)")
	globalCfg.VochainConfig.KeyKeeperIndex = *flag.Int8("keyKeeperIndex", 0,
		"index slot used by this node if it is a key keeper")
	globalCfg.VochainConfig.ImportPreviousCensus = *flag.Bool("importPreviousCensus", false,
		"if enabled the census downloader will import all existing census")
	globalCfg.VochainConfig.EthereumWhiteListAddrs = *flag.StringSlice("ethereumWhiteListAddrs",
		[]string{},
		"comma-separated list of ethereum addresses allowed to create processes on the vochain (oracle mode only)")
	globalCfg.VochainConfig.ProcessArchive = *flag.Bool("processArchive", false,
		"enables the process archiver component")
	globalCfg.VochainConfig.ProcessArchiveKey = *flag.String("processArchiveKey", "",
		"IPFS base64 encoded private key for process archive IPNS")

	// metrics
	globalCfg.Metrics.Enabled = *flag.Bool("metricsEnabled", false, "enable prometheus metrics")
	globalCfg.Metrics.RefreshInterval = *flag.Int("metricsRefreshInterval", 5,
		"metrics refresh interval in seconds")

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
	viper.BindPFlag("pprofPort", flag.Lookup("pprof"))

	// Use different datadirs for non main chains
	if globalCfg.VochainConfig.Chain != "main" {
		globalCfg.DataDir += "/" + globalCfg.VochainConfig.Chain
	}

	// Add viper config path (now we know it)
	globalCfg.DataDir = filepath.Clean(globalCfg.DataDir)
	viper.AddConfigPath(globalCfg.DataDir)

	// binding flags to viper
	// global
	viper.BindPFlag("mode", flag.Lookup("mode"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logErrorFile", flag.Lookup("logErrorFile"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("saveConfig", flag.Lookup("saveConfig"))

	// api
	viper.BindPFlag("api.Http", flag.Lookup("apihttp"))
	viper.BindPFlag("api.File", flag.Lookup("fileApi"))
	viper.BindPFlag("api.Census", flag.Lookup("censusApi"))
	viper.BindPFlag("api.Vote", flag.Lookup("voteApi"))
	viper.BindPFlag("api.Results", flag.Lookup("resultsApi"))
	viper.BindPFlag("api.Indexer", flag.Lookup("indexerApi"))
	viper.BindPFlag("api.Url", flag.Lookup("urlApi"))
	viper.BindPFlag("api.Route", flag.Lookup("apiRoute"))
	viper.BindPFlag("api.AllowPrivate", flag.Lookup("apiAllowPrivate"))
	viper.BindPFlag("api.AllowedAddrs", flag.Lookup("apiAllowedAddrs"))
	viper.BindPFlag("api.ListenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("api.ListenPort", flag.Lookup("listenPort"))
	viper.Set("api.Ssl.DirCert", globalCfg.DataDir+"/tls")
	viper.BindPFlag("api.Ssl.Domain", flag.Lookup("sslDomain"))

	// ethereum node
	viper.BindPFlag("ethConfig.SigningKey", flag.Lookup("ethSigningKey"))

	// ethereum web3
	viper.BindPFlag("w3Config.ChainType", flag.Lookup("ethChain"))
	viper.BindPFlag("w3Config.W3External", flag.Lookup("w3External"))

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
	viper.BindPFlag("vochainConfig.Peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.Seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.CreateGenesis", flag.Lookup("vochainCreateGenesis"))
	viper.BindPFlag("vochainConfig.Genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.MinerKey", flag.Lookup("vochainMinerKey"))
	viper.BindPFlag("vochainConfig.NodeKey", flag.Lookup("vochainNodeKey"))
	viper.BindPFlag("vochainConfig.PrivValidatorListenAddr", flag.Lookup("vochainPrivValidator"))
	viper.BindPFlag("vochainConfig.NoWaitSync", flag.Lookup("vochainNoWaitSync"))
	viper.BindPFlag("vochainConfig.MempoolSize", flag.Lookup("vochainMempoolSize"))
	viper.BindPFlag("vochainConfig.MinerTargetBlockTimeSeconds", flag.Lookup("vochainBlockTime"))
	viper.BindPFlag("vochainConfig.KeyKeeperIndex", flag.Lookup("keyKeeperIndex"))
	viper.BindPFlag("vochainConfig.ImportPreviousCensus", flag.Lookup("importPreviousCensus"))
	viper.BindPFlag("vochainConfig.EthereumWhiteListAddrs", flag.Lookup("ethereumWhiteListAddrs"))
	viper.Set("vochainConfig.ProcessArchiveDataDir", globalCfg.DataDir+"/archive")
	viper.BindPFlag("vochainConfig.ProcessArchive", flag.Lookup("processArchive"))
	viper.BindPFlag("vochainConfig.ProcessArchiveKey", flag.Lookup("processArchiveKey"))

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
		log.Warnf("non-critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message == "" {
		log.Infof("config file loaded successfully. Reminder: CLI flags have preference")
	}

	// Ensure we can have at least 8k open files. This is necessary, since
	// many components like IPFS and Tendermint require keeping many active
	// connections. Some systems have low defaults like 1024, which can make
	// the program crash after it's been running for a bit.
	if err := ensureNumberFiles(8000); err != nil {
		log.Errorf("could not ensure support for enough open files: %v", err)
	}

	// Check the mode is valid
	if !globalCfg.ValidMode() {
		log.Fatalf("mode %s is invalid", globalCfg.Mode)
	}
	// Check the dbType is valid
	if !globalCfg.VochainConfig.ValidDBType() {
		log.Fatalf("dbType %s is invalid. Valid ones: %s, %s", globalCfg.VochainConfig.DBType, db.TypePebble, db.TypeBadger)
	}

	// If dev enabled, expose debugging profiles under an http server
	// If PprofPort is not set, a random port between 61000 and 61100 is choosed.
	// We log what port is being used near the start of the logs, so it can
	// be easily grabbed. Start this before the rest of the node, since it
	// is helpful to debug if some other component hangs.
	if globalCfg.Dev || globalCfg.PprofPort > 0 {
		go func() {
			if globalCfg.PprofPort == 0 {
				globalCfg.PprofPort = int((time.Now().Unix() % 100)) + 61000
			}
			ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", globalCfg.PprofPort))
			if err != nil {
				log.Fatal(err)
			}
			log.Warnf("started pprof http endpoints at http://%s/debug/pprof", ln.Addr())
			log.Error(http.Serve(ln, nil))
		}()
	}

	log.Infof("starting vocdoni node version %q in %s mode",
		internal.Version, globalCfg.Mode)

	var err error
	var signer *ethereum.SignKeys
	var httpRouter httprouter.HTTProuter
	var rpc *rpcapi.RPCAPI
	var storage data.Storage
	var censusManager *census.Manager
	var vochainApp *vochain.BaseApplication
	var vochainInfo *vochaininfo.VochainInfo
	var scrutinizer *scrutinizer.Scrutinizer
	var vochainKeykeeper *keykeeper.KeyKeeper
	var vochainOracle *oracle.Oracle
	var metricsAgent *metrics.Agent

	if globalCfg.Dev {
		log.Warn("developer mode is enabled!")
	}
	if globalCfg.Mode == types.ModeGateway ||
		globalCfg.Mode == types.ModeOracle ||
		globalCfg.Mode == types.ModeEthAPIoracle {
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

	// HTTP(s) router and RPC for Gateway and EthApiOracle
	if globalCfg.Mode == types.ModeGateway || globalCfg.Metrics.Enabled ||
		globalCfg.Mode == types.ModeEthAPIoracle {
		// Initialize the HTTP router
		httpRouter.TLSdomain = globalCfg.API.Ssl.Domain
		httpRouter.TLSdirCert = globalCfg.API.Ssl.DirCert
		if err = httpRouter.Init(globalCfg.API.ListenHost, globalCfg.API.ListenPort); err != nil {
			log.Fatal(err)
		}
		// Initialize the RPC API
		if rpc, err = rpcapi.NewAPI(signer, &httpRouter, globalCfg.API.Route+"dvote", metricsAgent, globalCfg.API.AllowPrivate); err != nil {
			log.Fatal(err)
		}
		log.Infof("rpc API available at %s", globalCfg.API.Route+"dvote")

		// Enable metrics via proxy
		if globalCfg.Metrics.Enabled {
			metricsAgent = metrics.NewAgent("/metrics",
				time.Duration(globalCfg.Metrics.RefreshInterval)*time.Second, &httpRouter)
		}
	}

	if globalCfg.Mode == types.ModeGateway {
		// Storage service
		storage, err = service.IPFS(globalCfg.Ipfs, signer, metricsAgent)
		if err != nil {
			log.Fatal(err)
		}

		// Census service
		if globalCfg.API.Census {
			censusManager, err = service.Census(globalCfg.VochainConfig.DBType, globalCfg.DataDir, metricsAgent)
			if err != nil {
				log.Fatal(err)
			}
			censusManager.RemoteStorage = storage
		}
	}

	//
	// Vochain and Scrutinizer
	//
	if (globalCfg.Mode == types.ModeGateway && globalCfg.API.Vote) ||
		globalCfg.Mode == types.ModeMiner || globalCfg.Mode == types.ModeOracle ||
		globalCfg.Mode == types.ModeEthAPIoracle || globalCfg.Mode == types.ModeSeed {
		// set IsSeedNode to true if seed mode configured
		globalCfg.VochainConfig.IsSeedNode = types.ModeSeed == globalCfg.Mode
		// do we need scrutinizer?
		globalCfg.VochainConfig.Scrutinizer.Enabled = (globalCfg.Mode == types.ModeGateway && globalCfg.API.Results) ||
			(globalCfg.Mode == types.ModeOracle)
		// if oracle mode, we don't need live results
		globalCfg.VochainConfig.Scrutinizer.IgnoreLiveResults = (globalCfg.Mode == types.ModeOracle)

		// create the vochain node
		if vochainApp, scrutinizer, vochainInfo, err = service.Vochain(
			&service.VochainService{
				Config:        globalCfg.VochainConfig,
				MetricsAgent:  metricsAgent,
				CensusManager: censusManager,
				Storage:       storage,
			}); err != nil {
			log.Fatal(err)
		}
		defer func() {
			vochainApp.Service.Stop()
			vochainApp.Service.Wait()
		}()

		// Wait for Vochain to be ready
		var h, hPrev uint32
		for vochainApp.Node == nil {
			hPrev = h
			time.Sleep(time.Second * 10)
			h = vochainApp.Height()
			log.Infof("[vochain info] replaying height %d at %d blocks/s",
				h, (h-hPrev)/5)
		}
		log.Infof("vochain chainID %s", vochainApp.ChainID())
	}

	//
	// Oracle and ethApiOracle modes
	//
	if globalCfg.Mode == types.ModeOracle || globalCfg.Mode == types.ModeEthAPIoracle {
		if vochainOracle, err = oracle.NewOracle(vochainApp, signer); err != nil {
			log.Fatal(err)
		}

		if globalCfg.Mode == types.ModeOracle {
			// Start oracle results scrutinizer
			vochainOracle.EnableResults(scrutinizer)

			// Start keykeeper service (if key index specified)
			if globalCfg.VochainConfig.KeyKeeperIndex > 0 {
				vochainKeykeeper, err = keykeeper.NewKeyKeeper(
					path.Join(globalCfg.VochainConfig.DataDir, "keykeeper"),
					vochainApp,
					signer,
					globalCfg.VochainConfig.KeyKeeperIndex)
				if err != nil {
					log.Fatal(err)
				}
				go vochainKeykeeper.RevealUnpublished()
			}

			var w3uris []string
			if globalCfg.W3Config.W3External != nil {
				for idx, web3Endpoint := range globalCfg.W3Config.W3External {
					web3EndpointTrimmed := strings.Trim(web3Endpoint, `"[]`)
					log.Debugf("web3endpoint %d: %s", idx, web3EndpointTrimmed)
					switch {
					case strings.HasPrefix(web3EndpointTrimmed, "ws"):
						w3uris = append(w3uris, web3EndpointTrimmed)
					case strings.HasSuffix(web3EndpointTrimmed, "ipc"):
						w3uris = append(w3uris, web3EndpointTrimmed)
					default:
						log.Warnf(`invalid web3 endpoint %s must be websocket or IPC
						for event subscription`, web3EndpointTrimmed)
					}
				}
			}
			// Start ethereum events (if at least one web3 endpoint configured)
			if len(w3uris) > 0 {
				var evh []ethevents.EventHandler
				evh = append(evh, ethevents.HandleVochainOracle)

				whiteListedAddr := []string{}
				for _, addr := range globalCfg.VochainConfig.EthereumWhiteListAddrs {
					if addr == "[]" {
						// avoid empty address and pflag autofill in case of empty array
						continue
					}
					if ethcommon.IsHexAddress(addr) {
						whiteListedAddr = append(whiteListedAddr, addr)
					} else {
						log.Warnf("whitelist address %s is not a valid hex address", addr)
					}
				}
				if len(whiteListedAddr) > 0 {
					log.Infof("ethereum whitelisted addresses %+v", whiteListedAddr)
				}
				if err := service.EthEvents(
					context.Background(),
					w3uris,
					globalCfg.W3Config.ChainType,
					signer,
					vochainApp,
					evh,
					whiteListedAddr); err != nil {
					log.Fatal(err)
				}
			}
		}

		// Ethereum API oracle
		if globalCfg.Mode == types.ModeEthAPIoracle {
			log.Info("starting oracle API")
			apior, err := apioracle.NewAPIoracle(vochainOracle, rpc)
			if err != nil {
				log.Fatal(err)
			}
			if err := apior.EnableERC20(globalCfg.W3Config.ChainType, globalCfg.W3Config.W3External); err != nil {
				log.Fatal(err)
			}
		}
	}

	//
	// Gateway API
	//
	if globalCfg.Mode == types.ModeGateway {
		// dvote API service
		if globalCfg.API.File || globalCfg.API.Census || globalCfg.API.Vote || globalCfg.API.Indexer || globalCfg.API.URL {
			if _, err = service.API(globalCfg.API,
				rpc,           // rpcAPI
				storage,       // data storage
				censusManager, // census manager
				vochainApp,    // vochain core
				scrutinizer,   // scrutinizere
				vochainInfo,   // vochain info
				signer,
				metricsAgent); err != nil {
				log.Fatal(err)
			}
		}
		if globalCfg.API.URL {
			log.Info("enabling URL API")
			uAPI, err := urlapi.NewURLAPI(&httpRouter, "/v1/pub")
			if err != nil {
				log.Fatal(err)
			}
			if err := uAPI.EnableVotingHandlers(vochainApp, vochainInfo, scrutinizer); err != nil {
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
