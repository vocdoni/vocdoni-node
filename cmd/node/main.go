package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	_ "net/http/pprof" // for the pprof endpoints
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/google/uuid"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	urlapi "go.vocdoni.io/dvote/api"
	"go.vocdoni.io/dvote/api/censusdb"
	"go.vocdoni.io/dvote/api/faucet"
	"go.vocdoni.io/dvote/config"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/crypto/zk/circuit"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/metadb"
	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/internal"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/service"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/genesis"
	"go.vocdoni.io/dvote/vochain/keykeeper"
)

var (
	flagSaveConfig           bool
	flagVochainCreateGenesis string
)

// deprecatedFlagsFunc makes deprecated flags work the same as the new flags, but prints a warning
func deprecatedFlagsFunc(_ *flag.FlagSet, name string) flag.NormalizedName {
	oldName := name
	switch name {
	case "ipfsSyncKey":
		name = "ipfsConnectKey"
	case "ipfsSyncPeers":
		name = "ipfsConnectPeers"
	case "vochainBlockTime":
		return "vochainMinerTargetBlockTimeSeconds"
	case "skipPreviousOffchainData":
		return "vochainSkipPreviousOffchainData"
	case "offChainDataDownload":
		return "vochainOffChainDataDownload"
	case "pprof":
		return "pprofPort"
	}
	if oldName != name {
		log.Warnf("Flag --%s has been deprecated, please use --%s instead", oldName, name)
	}
	return flag.NormalizedName(name)
}

// pflagValueSet implements viper.FlagValueSet with our tweaks for categories.
type pflagValueSet struct {
	flags *flag.FlagSet
}

func (p pflagValueSet) VisitAll(fn func(flag viper.FlagValue)) {
	p.flags.VisitAll(func(flag *flag.Flag) {
		fn(pflagValue{flag})
	})
}

// pflagValue wraps pflag.Flag so it implements viper.FlagValue.
type pflagValue struct {
	flag *flag.Flag
}

var viperGroups = []string{"vochain", "ipfs", "metrics", "tls"}

func (p pflagValue) Name() string {
	name := p.flag.Name
	// In some cases, vochainFoo becomes vochain.Foo to get YAML nesting
	for _, group := range viperGroups {
		if after, ok := strings.CutPrefix(name, group); ok {
			return group + "." + after
		}
	}
	return name
}
func (p pflagValue) HasChanged() bool    { return p.flag.Changed }
func (p pflagValue) ValueString() string { return p.flag.Value.String() }
func (p pflagValue) ValueType() string   { return p.flag.Value.Type() }

// loadConfig creates a new config object and loads the stored configuration file
func loadConfig() *config.Config {
	conf := &config.Config{}

	// get current user home dir
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	// CLI flags will be used if something fails from this point
	// CLI flags have preference over the config file
	// Booleans should be passed to the CLI as: var=True/false

	// global
	flag.StringP("dataDir", "d", filepath.Join(home, ".vocdoni"),
		"directory where data is stored")
	flag.StringP("dbType", "t", db.TypePebble,
		fmt.Sprintf("key-value db type [%s,%s,%s]", db.TypePebble, db.TypeLevelDB, db.TypeMongo))
	flag.StringP("chain", "c", "dev",
		fmt.Sprintf("vocdoni network to connect with: %q", genesis.AvailableNetworks()))
	flag.Bool("dev", false,
		"use developer mode (less security)")

	flag.Int("pprofPort", 0,
		"pprof port for runtime profiling data (zero is disabled)")
	flag.StringP("logLevel", "l", "info",
		"log level (debug, info, warn, error, fatal)")
	flag.String("logOutput", "stdout",
		"log output (stdout, stderr or filepath)")
	flag.String("logErrorFile", "",
		"log errors and warnings to a file")
	flag.BoolVar(&flagSaveConfig, "saveConfig", false,
		"overwrite an existing config file with the provided CLI flags")
	flag.StringP("mode", "m", types.ModeGateway,
		"global operation mode. Available options: [gateway, miner, seed, census]")
	flag.StringP("signingKey", "k", "",
		"signing private Key as hex string (auto-generated if empty)")

	// api
	flag.String("listenHost", "0.0.0.0",
		"API endpoint listen address")
	flag.IntP("listenPort", "p", 9090,
		"API endpoint http port")
	flag.Bool("enableAPI", true,
		"enable HTTP API endpoints")
	flag.String("adminToken", "",
		"bearer token for admin API endpoints (leave empty to autogenerate)")
	flag.String("tlsDomain", "",
		"enable TLS-secure domain with LetsEncrypt (listenPort=443 is required)")
	flag.String("tlsDirCert", "",
		"directory where LetsEncrypt data is stored")
	flag.Uint64("enableFaucetWithAmount", 0,
		"enable faucet for the current network and the specified amount (testing purposes only)")

	// ipfs
	flag.StringP("ipfsConnectKey", "i", "",
		"enable IPFS group synchronization using the given secret key")
	flag.StringSlice("ipfsConnectPeers", []string{},
		"use custom ipfsconnect peers/bootnodes for accessing the DHT (comma-separated)")

	// vochain
	flag.String("vochainP2PListen", "0.0.0.0:26656",
		"p2p host and port to listent for the voting chain")
	flag.String("vochainPublicAddr", "",
		"external address:port to announce to other peers (automatically guessed if empty)")
	flag.String("vochainGenesis", "",
		"use alternative genesis file for the vochain")
	flag.String("vochainGenesisChainID", "",
		"override ChainID in genesis for the vochain")
	flag.Int64("vochainGenesisInitialHeight", 0,
		"override InitialHeight in genesis for the vochain")
	flag.String("vochainGenesisAppHash", "",
		"override AppHash in genesis for the vochain")
	flag.Int64("vochainGenesisEndOfChain", 0,
		"height at which this node will refuse adding new blocks to the chain")
	flag.String("vochainLogLevel", "disabled",
		"tendermint node log level (debug, info, error, disabled)")
	flag.StringSlice("vochainPeers", []string{},
		"comma-separated list of p2p peers")
	flag.StringSlice("vochainSeeds", []string{},
		"comma-separated list of p2p seed nodes")
	flag.String("vochainMinerKey", "",
		"user alternative vochain miner private key (hexstring[64])")
	flag.String("vochainNodeKey", "",
		"user alternative vochain private key (hexstring[64])")
	flag.Bool("vochainNoWaitSync", false,
		"do not wait for Vochain to synchronize (for testing only)")
	flag.Int("vochainMempoolSize", 20000,
		"vochain mempool size")
	flag.Int("vochainSnapshotInterval", 1000, // circa every 3hs (at 10s block interval)
		"create state snapshot every N blocks (0 to disable)")
	flag.Bool("vochainStateSyncEnabled", true,
		"during startup, let cometBFT ask peers for available snapshots and use them to bootstrap the state")
	flag.StringSlice("vochainStateSyncRPCServers", []string{},
		"list of RPC servers to bootstrap the StateSync (optional, defaults to using seeds)")
	flag.String("vochainStateSyncTrustHash", "",
		"hash of the trusted block (takes precedence over RPC and hardcoded defaults)")
	flag.Int64("vochainStateSyncTrustHeight", 0,
		"height of the trusted block (takes precedence over RPC and hardcoded defaults)")
	flag.Int64("vochainStateSyncChunkSize", 10*(1<<20), // 10 MB
		"cometBFT chunk size in bytes")
	flag.Bool("vochainStateSyncFetchParamsFromRPC", true,
		"allow statesync to fetch TrustHash and TrustHeight from the first RPCServer")

	flag.Int("vochainMinerTargetBlockTimeSeconds", int(config.DefaultMinerTargetBlockTime.Seconds()),
		"vochain consensus block time target (in seconds)")
	flag.Bool("vochainSkipPreviousOffchainData", false,
		"if enabled the census downloader will import all existing census")
	flag.Bool("vochainOffChainDataDownload", true,
		"enables the off-chain data downloader component")
	flag.StringVar(&flagVochainCreateGenesis, "vochainCreateGenesis", "",
		"create a genesis file for the vochain with validators and exit"+
			" (syntax <dir>:<numValidators>)")
	flag.Bool("vochainIndexerDisabled", false,
		"disables the vochain indexer component")

	// metrics
	flag.Bool("metricsEnabled", false, "enable prometheus metrics")
	flag.Int("metricsRefreshInterval", 5,
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

	// If VOCDONI_VOCHAIN is set, then other values like VOCDONI_VOCHAIN_PEERS
	// would be entirely ignored and left empty by viper as "shadowed".
	// Since that's almost always a human error and would lead to confusing failures,
	// refuse to continue any further.
	for _, group := range viperGroups {
		name := "VOCDONI_" + strings.ToUpper(group)
		if val := os.Getenv(name); val != "" {
			log.Fatalf("found %s=%s, which breaks our viper config", name, val)
		}
	}

	viper.BindFlagValues(pflagValueSet{flag.CommandLine})

	// use different datadirs for different networks
	conf.DataDir = filepath.Join(viper.GetString("dataDir"), viper.GetString("chain"))
	viper.Set("dataDir", conf.DataDir)

	// set up the data subdirectories (if no flag or env was passed)
	if viper.GetString("TLS.DirCert") == "" {
		viper.Set("TLS.DirCert", filepath.Join(conf.DataDir, "tls"))
	}
	if viper.GetString("ipfs.ConfigPath") == "" {
		viper.Set("ipfs.ConfigPath", filepath.Join(conf.DataDir, "ipfs"))
	}
	if viper.GetString("vochain.DataDir") == "" {
		viper.Set("vochain.DataDir", filepath.Join(conf.DataDir, "vochain"))
	}

	// propagate some keys to the vochain category
	viper.Set("vochain.dbType", viper.GetString("dbType"))
	viper.Set("vochain.Dev", viper.GetBool("dev"))

	// add viper config path (now we know it)
	viper.AddConfigPath(conf.DataDir)
	// check if config file exists
	if _, err := os.Stat(conf.DataDir + "/vocdoni.yml"); errors.Is(err, fs.ErrNotExist) {
		log.Infof("creating new config file in %s", conf.DataDir)
		// creating config folder if not exists
		if err := os.MkdirAll(conf.DataDir, os.ModePerm); err != nil {
			log.Fatalf("cannot create data directory: %s", err)
		}
		// create config file if not exists
		if err := viper.SafeWriteConfig(); err != nil {
			log.Fatalf("cannot write config file into config dir: %s", err)
		}
	} else {
		// read config file
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("cannot read loaded config file in %s: %s", conf.DataDir, err)
		}
	}

	if err := viper.Unmarshal(&conf); err != nil {
		log.Fatalf("cannot unmarshal loaded config file: %s", err)
	}
	// Note that these Config.Vochain fields aren't bound via viper.
	// We could do that if we rename the flags.
	conf.Vochain.Indexer.Enabled = !viper.GetBool("vochainIndexerDisabled")
	conf.Vochain.Network = viper.GetString("chain")

	if conf.SigningKey == "" {
		log.Info("no signing key, generating one...")
		signer := ethereum.NewSignKeys()
		if err := signer.Generate(); err != nil {
			log.Fatalf("cannot generate signing key: %s", err)
		}
		_, priv := signer.HexString()
		viper.Set("signingKey", priv)
		conf.SigningKey = priv
		flagSaveConfig = true
	}

	if conf.Vochain.MinerKey == "" {
		conf.Vochain.MinerKey = conf.SigningKey
	}

	if conf.AdminToken == "" {
		conf.AdminToken = uuid.New().String()
		log.Info("created new admin API token", conf.AdminToken)
	}

	if flagSaveConfig {
		viper.Set("saveConfig", false)
		if err := viper.WriteConfig(); err != nil {
			log.Fatalf("cannot overwrite config file into config dir: %s", err)
		}
	}

	return conf
}

func main() {
	// Report the version before loading the config or logger init, just in case something goes wrong.
	// For the sake of including the version in the log, it's also included in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

	// creating config and init logger
	conf := loadConfig()

	var errorOutput io.Writer
	if path := conf.LogErrorFile; path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			panic(fmt.Sprintf("cannot create error log output: %v", err))
		}
		errorOutput = f
	}
	log.Init(conf.LogLevel, conf.LogOutput, errorOutput)
	if path := conf.LogErrorFile; path != "" {
		// Once the logger has been initialized.
		log.Infof("using file %s for logging warning and errors", path)
	}
	log.Debugf("loaded config %+v", *conf)

	// Check if we need to create a vochain genesis file with validators and exit.
	if flagVochainCreateGenesis != "" {
		dirWithNodes := strings.Split(flagVochainCreateGenesis, ":")
		if len(dirWithNodes) != 2 {
			log.Fatal("invalid format for --vochainCreateGenesis expected dir:numValidators (e.g. /tmp/vochain:4)")
		}
		num, err := strconv.Atoi(dirWithNodes[1])
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("creating vochain genesis file with %d validators in %s", num, dirWithNodes[0])
		if _, err := vochain.NewTemplateGenesisFile(dirWithNodes[0], num); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Overwrite the default path to download the zksnarks circuits artifacts
	// using the global datadir as parent folder.
	circuit.BaseDir = filepath.Join(conf.DataDir, "zkCircuits")

	// Check the mode is valid
	if !conf.ValidMode() {
		log.Fatalf("mode %s is invalid", conf.Mode)
	}

	// If dev enabled, expose debugging profiles under an http server
	// If PprofPort is not set, a random port between 61000 and 61100 is choosed.
	// We log what port is being used near the start of the logs, so it can
	// be easily grabbed. Start this before the rest of the node, since it
	// is helpful to debug if some other component hangs.
	if conf.Dev || conf.PprofPort > 0 {
		go func() {
			if conf.PprofPort == 0 {
				conf.PprofPort = int(time.Now().Unix()%100) + 61000
			}
			ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", conf.PprofPort))
			if err != nil {
				log.Fatal(err)
			}
			log.Warnf("started pprof http endpoints at http://%s/debug/pprof", ln.Addr())
			log.Error(http.Serve(ln, nil))
		}()
	}
	log.Infow("starting vocdoni node", "version", internal.Version, "mode", conf.Mode,
		"network", conf.Vochain.Network, "dbType", conf.Vochain.DBType)
	if conf.Dev {
		log.Warn("developer mode is enabled!")
	}

	var err error
	srv := service.VocdoniService{Config: &conf.Vochain}

	if conf.Mode == types.ModeGateway {
		// Signing key
		srv.Signer = ethereum.NewSignKeys()

		// Add signing private key if exist in configuration or flags
		if len(conf.SigningKey) != 32 {
			err := srv.Signer.AddHexKey(conf.SigningKey)
			if err != nil {
				log.Fatalf("error adding hex key: (%s)", err)
			}
		} else {
			log.Fatal("wrong signing key length (32 hexadecimal chars expected)")
		}
		log.Infof("signing address %s, pubKey %x", srv.Signer.AddressString(), srv.Signer.PublicKey())
	}

	// HTTP(s) router for Gateway or Prometheus metrics
	if conf.Mode == types.ModeGateway || conf.Metrics.Enabled || conf.Mode == types.ModeCensus {
		// Initialize the HTTP router
		srv.Router = new(httprouter.HTTProuter)
		srv.Router.TLSdomain = conf.TLS.Domain
		srv.Router.TLSdirCert = conf.TLS.DirCert
		if err := srv.Router.Init(conf.ListenHost, conf.ListenPort); err != nil {
			log.Fatal(err)
		}
		// Enable metrics via proxy
		if conf.Metrics.Enabled {
			// This flag will make CometBFT register their metrics in prometheus
			srv.Config.TendermintMetrics = true
			srv.Router.ExposePrometheusEndpoint("/metrics")

			metrics.NewCounter(fmt.Sprintf("vocdoni_info{version=%q,mode=%q,network=%q}",
				internal.Version, conf.Mode, conf.Vochain.Network)).Set(1)
		}
	}

	// Storage service for Gateway
	if conf.Mode == types.ModeGateway || conf.Mode == types.ModeCensus {
		srv.Storage, err = srv.IPFS(&conf.Ipfs)
		if err != nil {
			log.Fatal(err)
		}
	}

	//
	// Vochain and Indexer
	//
	if conf.Mode == types.ModeGateway ||
		conf.Mode == types.ModeMiner ||
		conf.Mode == types.ModeSeed {
		// set IsSeedNode to true if seed mode configured
		conf.Vochain.IsSeedNode = types.ModeSeed == conf.Mode
		// offchainDataDownload is only needed for gateways
		conf.Vochain.OffChainDataDownload = conf.Vochain.OffChainDataDownload &&
			conf.Mode == types.ModeGateway

		// if there's no indexer, then never do snapshots because they would be incomplete
		if !conf.Vochain.Indexer.Enabled {
			conf.Vochain.SnapshotInterval = 0
		}

		// create the vochain service
		if err := srv.Vochain(); err != nil {
			log.Fatal(err)
		}
		// create the offchain data downloader service
		if conf.Vochain.OffChainDataDownload {
			if err := srv.OffChainDataHandler(); err != nil {
				log.Fatal(err)
			}
		}
		// create the indexer service
		if conf.Vochain.Indexer.Enabled {
			if err := srv.VochainIndexer(); err != nil {
				log.Fatal(err)
			}
		}
		// start the service and block until finish sync:
		// StateSync (if enabled) happens first, and then fastsync in all cases
		if err := srv.Start(); err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := srv.App.Node.Stop(); err != nil {
				log.Warn(err)
			}
			ch := srv.App.Node.Quit()
			select {
			case <-ch:
				log.Info("vochain service stopped")
			case <-time.After(30 * time.Second):
				log.Warn("vochain service stop timeout")
			}
		}()
	}

	//
	// Validator
	//
	if conf.Mode == types.ModeMiner {
		// create the key for the validator used to sign transactions
		signer := ethereum.SignKeys{}
		if err := signer.AddHexKey(conf.Vochain.MinerKey); err != nil {
			log.Fatalf("add hex key failed %v", err)
		}
		validator, err := srv.App.State.Validator(signer.Address(), true)
		if err != nil {
			log.Fatal(err)
		}
		if validator != nil {
			// start keykeeper service (if key index specified)
			if validator.KeyIndex > 0 {
				srv.KeyKeeper, err = keykeeper.NewKeyKeeper(
					srv.App,
					&signer,
					int8(validator.KeyIndex))
				if err != nil {
					log.Fatal(err)
				}
				go srv.KeyKeeper.RevealUnpublished()
				log.Infow("configured keykeeper validator",
					"address", signer.Address().Hex(),
					"keyIndex", validator.KeyIndex)
			}
		}
	}

	//
	// Gateway API and RPC
	//
	if conf.Mode == types.ModeGateway && conf.EnableAPI {
		// HTTP API REST service
		log.Info("enabling API")
		uAPI, err := urlapi.NewAPI(srv.Router, "/v2", conf.DataDir, conf.Vochain.DBType)
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
		uAPI.Endpoint.SetAdminToken(conf.AdminToken)
		if err := uAPI.EnableHandlers(
			urlapi.ElectionHandler,
			urlapi.VoteHandler,
			urlapi.ChainHandler,
			urlapi.WalletHandler,
			urlapi.AccountHandler,
			urlapi.CensusHandler,
			urlapi.SIKHandler,
		); err != nil {
			log.Fatal(err)
		}
		// attach faucet to the API if enabled
		if conf.EnableFaucetWithAmount > 0 {
			if err := faucet.AttachFaucetAPI(srv.Signer,
				conf.EnableFaucetWithAmount,
				uAPI.Endpoint,
				"/open/claim",
			); err != nil {
				log.Fatal(err)
			}
		}
	}

	if conf.Mode == types.ModeCensus {
		log.Info("enabling API")
		uAPI, err := urlapi.NewAPI(srv.Router, "/v2", conf.DataDir, conf.Vochain.DBType)
		if err != nil {
			log.Fatal(err)
		}
		db, err := metadb.New(conf.Vochain.DBType, filepath.Join(conf.DataDir, "censusdb"))
		if err != nil {
			log.Fatal(err)
		}
		censusDB := censusdb.NewCensusDB(db)
		uAPI.Attach(
			nil,
			nil,
			nil,
			srv.Storage,
			censusDB,
		)
		uAPI.Endpoint.SetAdminToken(conf.AdminToken)
		if err := uAPI.EnableHandlers(
			urlapi.CensusHandler,
		); err != nil {
			log.Fatal(err)
		}
	}

	log.Infof("startup complete at %s", time.Now().Format(time.RFC850))

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warnf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	height, err := srv.App.State.LastHeight()
	if err != nil {
		log.Warn(err)
	}
	hash, err := srv.App.State.MainTreeView().Root()
	if err != nil {
		log.Warn(err)
	}
	tmBlock := srv.App.GetBlockByHeight(int64(height))
	log.Infow("last block", "height", height, "appHash", hex.EncodeToString(hash),
		"time", tmBlock.Time, "tmAppHash", tmBlock.AppHash.String(), "tmHeight", tmBlock.Height)
}
