package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // for the pprof endpoints
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	urlapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/metrics"
	"go.vocdoni.io/dvote/oracle"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/keykeeper"
)

var createVochainGenesisFile = ""

// deprecatedFlagsFunc makes deprecated flags work the same as the new flags, but prints a warning
func deprecatedFlagsFunc(f *flag.FlagSet, name string) flag.NormalizedName {
	oldName := name
	switch name {
	case "ipfsSyncKey":
		name = "ipfsConnectKey"
	case "ipfsSyncPeers":
		name = "ipfsConnectPeers"
	}
	if oldName != name {
		fmt.Printf("Flag --%s has been deprecated, please use --%s instead\n", oldName, name)
	}
	return flag.NormalizedName(name)
}

// newConfig creates a new config object and loads the stored configuration file
func newConfig() (*config.Config, config.Error) {
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
	flag.StringVarP(&globalCfg.DataDir, "dataDir", "d", home+"/.vocdoni",
		"directory where data is stored")
	flag.StringVarP(&globalCfg.Vochain.DBType, "dbType", "t", db.TypePebble,
		fmt.Sprintf("key-value db type (%s, %s)", db.TypePebble, db.TypeBadger))
	flag.StringVarP(&globalCfg.Vochain.Chain, "chain", "c", "dev",
		fmt.Sprintf("vocdoni blockchain to connect with: %q", genesis.GenesisAvailableChains()))
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
	globalCfg.Mode = *flag.StringP("mode", "m", types.ModeGateway,
		"global operation mode. Available options: [gateway,oracle,ethApiOracle,miner,seed]")
	globalCfg.SigningKey = *flag.StringP("signingKey", "k", "",
		"signing private Key as hex string (auto-generated if empty)")

	// api & rpc
	globalCfg.ListenHost = *flag.String("listenHost", "0.0.0.0",
		"API endpoint listen address")
	globalCfg.ListenPort = *flag.IntP("listenPort", "p", 9090,
		"API endpoint http port")
	globalCfg.EnableAPI = *flag.Bool("enableAPI", true,
		"enable HTTP API endpoints")
	globalCfg.EnableRPC = *flag.Bool("enableRPC", false,
		"enable legacy JSON-RPC endpoint (deprecated)")
	globalCfg.TLS.Domain = *flag.String("tlsDomain", "",
		"enable TLS-secure domain with LetsEncrypt (listenPort=443 is required)")

	// ipfs
	globalCfg.Ipfs.ConnectKey = *flag.StringP("ipfsConnectKey", "i", "",
		"enable IPFS group synchronization using the given secret key")
	globalCfg.Ipfs.ConnectPeers = *flag.StringSlice("ipfsConnectPeers", []string{},
		"use custom ipfsconnect peers/bootnodes for accessing the DHT (comma-separated)")

	// vochain
	globalCfg.Vochain.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656",
		"p2p host and port to listent for the voting chain")
	globalCfg.Vochain.PublicAddr = *flag.String("vochainPublicAddr", "",
		"external address:port to announce to other peers (automatically guessed if empty)")
	globalCfg.Vochain.RPCListen = *flag.String("vochainRPCListen", "127.0.0.1:26657",
		"rpc host and port to listen to for the voting chain")
	globalCfg.Vochain.Genesis = *flag.String("vochainGenesis", "",
		"use alternative genesis file for the vochain")
	globalCfg.Vochain.LogLevel = *flag.String("vochainLogLevel", "disabled",
		"tendermint node log level (debug, info, error, disabled)")
	globalCfg.Vochain.Peers = *flag.StringSlice("vochainPeers", []string{},
		"comma-separated list of p2p peers")
	globalCfg.Vochain.Seeds = *flag.StringSlice("vochainSeeds", []string{},
		"comma-separated list of p2p seed nodes")
	globalCfg.Vochain.MinerKey = *flag.String("vochainMinerKey", "",
		"user alternative vochain miner private key (hexstring[64])")
	globalCfg.Vochain.NodeKey = *flag.String("vochainNodeKey", "",
		"user alternative vochain private key (hexstring[64])")
	globalCfg.Vochain.NoWaitSync = *flag.Bool("vochainNoWaitSync", false,
		"do not wait for Vochain to synchronize (for testing only)")
	globalCfg.Vochain.MempoolSize = *flag.Int("vochainMempoolSize", 20000,
		"vochain mempool size")
	globalCfg.Vochain.MinerTargetBlockTimeSeconds = *flag.Int("vochainBlockTime", 10,
		"vochain consensus block time target (in seconds)")
	globalCfg.Vochain.KeyKeeperIndex = *flag.Int8("keyKeeperIndex", 0,
		"index slot used by this node if it is a key keeper")
	globalCfg.Vochain.ImportPreviousCensus = *flag.Bool("importPreviousCensus", false,
		"if enabled the census downloader will import all existing census")
	globalCfg.Vochain.ProcessArchive = *flag.Bool("processArchive", false,
		"enables the process archiver component")
	globalCfg.Vochain.ProcessArchiveKey = *flag.String("processArchiveKey", "",
		"IPFS base64 encoded private key for process archive IPNS")
	globalCfg.Vochain.OffChainDataDownloader = *flag.Bool("offChainDataDownload", true,
		"enables the off-chain data downloader component")
	flag.StringVar(&createVochainGenesisFile, "vochainCreateGenesis", "",
		"create a genesis file for the vochain with validators and exit"+
			" (syntax <dir>:<numValidators>)")

	// metrics
	globalCfg.Metrics.Enabled = *flag.Bool("metricsEnabled", false, "enable prometheus metrics")
	globalCfg.Metrics.RefreshInterval = *flag.Int("metricsRefreshInterval", 5,
		"metrics refresh interval in seconds")

	// parse flags
	flag.CommandLine.SortFlags = false
	flag.CommandLine.SetNormalizeFunc(deprecatedFlagsFunc)
	flag.Parse()

	// setting up viper
	viper := viper.New()
	viper.SetConfigName("vocdoni")
	viper.SetConfigType("yml")
	viper.SetEnvPrefix("VOCDONI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// set FlagVars first
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	globalCfg.DataDir = viper.GetString("dataDir")
	viper.BindPFlag("chain", flag.Lookup("chain"))
	globalCfg.Vochain.Chain = viper.GetString("chain")
	viper.BindPFlag("dev", flag.Lookup("dev"))
	globalCfg.Dev = viper.GetBool("dev")
	viper.BindPFlag("pprofPort", flag.Lookup("pprof"))

	// use different datadirs for different chains
	globalCfg.DataDir = filepath.Join(globalCfg.DataDir, globalCfg.Vochain.Chain)

	// add viper config path (now we know it)
	viper.AddConfigPath(globalCfg.DataDir)

	// binding flags to viper
	viper.BindPFlag("mode", flag.Lookup("mode"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logErrorFile", flag.Lookup("logErrorFile"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("saveConfig", flag.Lookup("saveConfig"))
	viper.BindPFlag("signingKey", flag.Lookup("signingKey"))

	viper.BindPFlag("enableAPI", flag.Lookup("enableAPI"))
	viper.BindPFlag("enableRPC", flag.Lookup("enableRPC"))

	viper.Set("TLS.DirCert", globalCfg.DataDir+"/tls")
	viper.BindPFlag("TLS.Domain", flag.Lookup("tlsDomain"))

	// ipfs
	viper.Set("ipfs.ConfigPath", globalCfg.DataDir+"/ipfs")
	viper.BindPFlag("ipfs.ConnectKey", flag.Lookup("ipfsConnectKey"))
	viper.BindPFlag("ipfs.ConnectPeers", flag.Lookup("ipfsConnectPeers"))

	// vochain
	viper.Set("vochain.DataDir", globalCfg.DataDir+"/vochain")
	viper.Set("vochain.Dev", globalCfg.Dev)
	viper.BindPFlag("vochain.P2PListen", flag.Lookup("vochainP2PListen"))
	viper.BindPFlag("vochain.PublicAddr", flag.Lookup("vochainPublicAddr"))
	viper.BindPFlag("vochain.RPCListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("vochain.LogLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochain.Peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochain.Seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochain.CreateGenesis", flag.Lookup("vochainCreateGenesis"))
	viper.BindPFlag("vochain.Genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochain.MinerKey", flag.Lookup("vochainMinerKey"))
	viper.BindPFlag("vochain.NodeKey", flag.Lookup("vochainNodeKey"))
	viper.BindPFlag("vochain.NoWaitSync", flag.Lookup("vochainNoWaitSync"))
	viper.BindPFlag("vochain.MempoolSize", flag.Lookup("vochainMempoolSize"))
	viper.BindPFlag("vochain.MinerTargetBlockTimeSeconds", flag.Lookup("vochainBlockTime"))
	viper.BindPFlag("vochain.KeyKeeperIndex", flag.Lookup("keyKeeperIndex"))
	viper.BindPFlag("vochain.ImportPreviousCensus", flag.Lookup("importPreviousCensus"))
	viper.Set("vochain.ProcessArchiveDataDir", globalCfg.DataDir+"/archive")
	viper.BindPFlag("vochain.ProcessArchive", flag.Lookup("processArchive"))
	viper.BindPFlag("vochain.ProcessArchiveKey", flag.Lookup("processArchiveKey"))
	viper.BindPFlag("vochain.OffChainDataDownload", flag.Lookup("offChainDataDownload"))

	// metrics
	viper.BindPFlag("metrics.Enabled", flag.Lookup("metricsEnabled"))
	viper.BindPFlag("metrics.RefreshInterval", flag.Lookup("metricsRefreshInterval"))

	// check if config file exists
	_, err = os.Stat(globalCfg.DataDir + "/vocdoni.yml")
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

	if len(globalCfg.SigningKey) < 32 {
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
		viper.Set("signingKey", priv)
		globalCfg.SigningKey = priv
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

func main() {
	// Don't use the log package here, because we want to report the version
	// before loading the config. This is because something could go wrong
	// while loading the config, and because the logger isn't set up yet.
	// For the sake of including the version in the log, it's also included
	// in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

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

	// Check if we need to create a vochain genesis file with validators and exit.
	if createVochainGenesisFile != "" {
		dirWithNodes := strings.Split(createVochainGenesisFile, ":")
		if len(dirWithNodes) != 2 {
			log.Fatal("invalid format for --vochainCreateGenesis expected dir:numValidators (e.g. /tmp/vochain:4)")
		}
		num, err := strconv.Atoi(dirWithNodes[1])
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("creating vochain genesis file with %d validators in %s", num, dirWithNodes[0])
		if err := vochain.NewTemplateGenesisFile(dirWithNodes[0], num); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Check if errors during config creation and determine if Critical.
	log.Debugf("initializing config %+v", *globalCfg)
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
	if !globalCfg.Vochain.ValidDBType() {
		log.Fatalf("dbType %s is invalid. Valid ones: %s, %s", globalCfg.Vochain.DBType, db.TypePebble, db.TypeBadger)
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
	if globalCfg.Dev {
		log.Warn("developer mode is enabled!")
	}

	var err error
	var vochainKeykeeper *keykeeper.KeyKeeper
	var vochainOracle *oracle.Oracle
	srv := service.VocdoniService{Config: globalCfg.Vochain}

	if globalCfg.Mode == types.ModeGateway ||
		globalCfg.Mode == types.ModeOracle {
		// Signing key
		srv.Signer = ethereum.NewSignKeys()

		// Add signing private key if exist in configuration or flags
		if len(globalCfg.SigningKey) != 32 {
			err := srv.Signer.AddHexKey(globalCfg.SigningKey)
			if err != nil {
				log.Fatalf("error adding hex key: (%s)", err)
			}
		} else {
			log.Fatal("wrong signing key length (32 hexadecomal chars expected)")
		}
		log.Infof("signing address %s, pubKey %x", srv.Signer.AddressString(), srv.Signer.PublicKey())
	}

	// HTTP(s) router for Gateway or Prometheus metrics
	if globalCfg.Mode == types.ModeGateway || globalCfg.Metrics.Enabled {
		// Initialize the HTTP router
		srv.Router = new(httprouter.HTTProuter)
		srv.Router.TLSdomain = globalCfg.TLS.Domain
		srv.Router.TLSdirCert = globalCfg.TLS.DirCert
		if err = srv.Router.Init(globalCfg.ListenHost, globalCfg.ListenPort); err != nil {
			log.Fatal(err)
		}
		// Enable metrics via proxy
		if globalCfg.Metrics.Enabled {
			srv.MetricsAgent = metrics.NewAgent("/metrics",
				time.Duration(globalCfg.Metrics.RefreshInterval)*time.Second, srv.Router)
		}
	}

	// Storage service for Gateway
	if globalCfg.Mode == types.ModeGateway {
		srv.Storage, err = srv.IPFS(globalCfg.Ipfs)
		if err != nil {
			log.Fatal(err)
		}
	}

	//
	// Vochain and Indexer
	//
	if globalCfg.Mode == types.ModeGateway ||
		globalCfg.Mode == types.ModeMiner ||
		globalCfg.Mode == types.ModeOracle ||
		globalCfg.Mode == types.ModeSeed {
		// set IsSeedNode to true if seed mode configured
		globalCfg.Vochain.IsSeedNode = types.ModeSeed == globalCfg.Mode
		// do we need indexer?
		globalCfg.Vochain.Indexer.Enabled = (globalCfg.Mode == types.ModeGateway) ||
			(globalCfg.Mode == types.ModeOracle)
		// if oracle mode, we don't need live results
		globalCfg.Vochain.Indexer.IgnoreLiveResults = (globalCfg.Mode == types.ModeOracle)
		// offchainDataDownloader is only needed for gateways
		globalCfg.Vochain.OffChainDataDownloader = globalCfg.Vochain.OffChainDataDownloader &&
			globalCfg.Mode == types.ModeGateway
		// create the vochain service
		if err = srv.Vochain(); err != nil {
			log.Fatal(err)
		}
		defer func() {
			srv.App.Service.Stop()
			srv.App.Service.Wait()
		}()

		if globalCfg.Vochain.OffChainDataDownloader {
			if err := srv.OffChainDataHandler(); err != nil {
				log.Fatal(err)
			}
		}

		if globalCfg.Vochain.Indexer.Enabled {
			if err := srv.VochainIndexer(); err != nil {
				log.Fatal(err)
			}
		}

		if globalCfg.Vochain.ProcessArchive {
			if err := srv.ProcessArchiver(); err != nil {
				log.Fatal(err)
			}
		}

		// Wait for Vochain to be ready
		var h, hPrev uint32
		for srv.App.Node == nil {
			hPrev = h
			time.Sleep(time.Second * 10)
			h = srv.App.Height()
			log.Infof("[vochain info] replaying height %d at %d blocks/s",
				h, (h-hPrev)/5)
		}
		log.Infof("vochain chainID %s", srv.App.ChainID())
	}

	//
	// Oracle
	//
	if globalCfg.Mode == types.ModeOracle {
		if vochainOracle, err = oracle.NewOracle(srv.App, srv.Signer); err != nil {
			log.Fatal(err)
		}
		// Start oracle results indexer
		vochainOracle.EnableResults(srv.Indexer)
		// Start keykeeper service (if key index specified)
		if globalCfg.Vochain.KeyKeeperIndex > 0 {
			vochainKeykeeper, err = keykeeper.NewKeyKeeper(
				path.Join(globalCfg.Vochain.DataDir, "keykeeper"),
				srv.App,
				srv.Signer,
				globalCfg.Vochain.KeyKeeperIndex)
			if err != nil {
				log.Fatal(err)
			}
			go vochainKeykeeper.RevealUnpublished()
		}
	}

	//
	// Gateway API and RPC
	//
	if globalCfg.Mode == types.ModeGateway {
		// JSON-RPC service
		if globalCfg.EnableRPC {
			log.Info("enabling JSON-RPC")
			if _, err = srv.LegacyRPC(); err != nil {
				log.Fatal(err)
			}
		}

		// HTTP API REST service
		if globalCfg.EnableAPI {
			log.Info("enabling API")
			uAPI, err := urlapi.NewAPI(srv.Router, "/v2", globalCfg.DataDir)
			if err != nil {
				log.Fatal(err)
			}
			uAPI.Attach(
				srv.App,
				srv.Stats,
				srv.Indexer,
				srv.Storage,
				srv.CensusDB,
			)
			if err := uAPI.EnableHandlers(
				urlapi.ElectionHandler,
				urlapi.VoteHandler,
				urlapi.ChainHandler,
				urlapi.WalletHandler,
				urlapi.AccountHandler,
				urlapi.CensusHandler,
			); err != nil {
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
