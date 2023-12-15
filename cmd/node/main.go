package main

import (
	"encoding/hex"
	"fmt"
	"io"
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

var createVochainGenesisFile = ""

// deprecatedFlagsFunc makes deprecated flags work the same as the new flags, but prints a warning
func deprecatedFlagsFunc(_ *flag.FlagSet, name string) flag.NormalizedName {
	oldName := name
	switch name {
	case "ipfsSyncKey":
		name = "ipfsConnectKey"
	case "ipfsSyncPeers":
		name = "ipfsConnectPeers"
	}
	if oldName != name {
		log.Warnf("Flag --%s has been deprecated, please use --%s instead", oldName, name)
	}
	// These were just renamed; don't warn just yet.
	switch name {
	case "vochainBlockTime":
		return "vochainMinerTargetBlockTimeSeconds"
	case "skipPreviousOffchainData":
		return "vochainSkipPreviousOffchainData"
	case "processArchive":
		return "vochainProcessArchive"
	case "processArchiveKey":
		return "vochainProcessArchiveKey"
	case "offChainDataDownload":
		return "vochainOffChainDataDownload"
	case "pprof":
		return "pprofPort"
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

func (p pflagValue) Name() string {
	name := p.flag.Name
	// In some cases, vochainFoo becomes vochain.Foo to get YAML nesting
	if after, ok := strings.CutPrefix(name, "vochain"); ok {
		return "vochain." + after
	}
	if after, ok := strings.CutPrefix(name, "ipfs"); ok {
		return "ipfs." + after
	}
	if after, ok := strings.CutPrefix(name, "metrics"); ok {
		return "metrics." + after
	}
	if after, ok := strings.CutPrefix(name, "tls"); ok {
		return "TLS." + after // note that it's all uppercase
	}
	return name
}
func (p pflagValue) HasChanged() bool    { return p.flag.Changed }
func (p pflagValue) ValueString() string { return p.flag.Value.String() }
func (p pflagValue) ValueType() string   { return p.flag.Value.Type() }

// newConfig creates a new config object and loads the stored configuration file
func newConfig() (*config.Config, config.Error) {
	var cfgError config.Error
	conf := &config.Config{}

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
	// TODO: remove from vocdoni.yml file; it's literally where the file sits
	flag.StringP("dataDir", "d", home+"/.vocdoni",
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
	// TODO: remove from vocdoni.yml file
	flag.Bool("saveConfig", false,
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
	flag.Uint64("enableFaucetWithAmount", 0,
		"enable faucet for the current network and the specified amount (testing purposes only)")

	// ipfs
	flag.StringP("ipfsConnectKey", "i", "",
		"enable IPFS group synchronization using the given secret key")
	flag.StringSlice("ipfsConnectPeers", []string{},
		"use custom ipfsconnect peers/bootnodes for accessing the DHT (comma-separated)")
	flag.String("archiveURL", types.ArchiveURL, "enable archive retrival from the given IPNS url")

	// vochain
	flag.String("vochainP2PListen", "0.0.0.0:26656",
		"p2p host and port to listent for the voting chain")
	flag.String("vochainPublicAddr", "",
		"external address:port to announce to other peers (automatically guessed if empty)")
	flag.String("vochainGenesis", "",
		"use alternative genesis file for the vochain")
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

	flag.Int("vochainMinerTargetBlockTimeSeconds", 10,
		"vochain consensus block time target (in seconds)")
	flag.Bool("vochainSkipPreviousOffchainData", false,
		"if enabled the census downloader will import all existing census")
	flag.Bool("vochainProcessArchive", false,
		"enables the process archiver component")
	flag.String("vochainProcessArchiveKey", "",
		"IPFS base64 encoded private key for process archive IPNS")
	flag.Bool("vochainOffChainDataDownload", true,
		"enables the off-chain data downloader component")
	// TODO: remove from vocdoni.yml file
	flag.StringVar(&createVochainGenesisFile, "vochainCreateGenesis", "",
		"create a genesis file for the vochain with validators and exit"+
			" (syntax <dir>:<numValidators>)")

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

	viper.BindFlagValues(pflagValueSet{flag.CommandLine})

	conf.Vochain.DBType = viper.GetString("dbType")                 // TODO: unused?
	conf.Vochain.Indexer.ArchiveURL = viper.GetString("archiveURL") // TODO: unused?

	// use different datadirs for different networks
	conf.DataDir = viper.GetString("dataDir")
	conf.Vochain.Network = viper.GetString("chain")
	conf.DataDir = filepath.Join(conf.DataDir, conf.Vochain.Network)
	viper.Set("dataDir", conf.DataDir)

	// set up the data subdirectories
	viper.Set("TLS.DirCert", conf.DataDir+"/tls")
	viper.Set("ipfs.ConfigPath", conf.DataDir+"/ipfs")
	viper.Set("vochain.DataDir", conf.DataDir+"/vochain")
	viper.Set("vochain.ProcessArchiveDataDir", conf.DataDir+"/archive")

	// propagate dev to vochain.Dev
	viper.Set("vochain.Dev", viper.GetBool("dev"))

	// add viper config path (now we know it)
	viper.AddConfigPath(conf.DataDir)
	// check if config file exists
	_, err = os.Stat(conf.DataDir + "/vocdoni.yml")
	if os.IsNotExist(err) {
		cfgError = config.Error{
			Message: fmt.Sprintf("creating new config file in %s", conf.DataDir),
		}
		// creating config folder if not exists
		err = os.MkdirAll(conf.DataDir, os.ModePerm)
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
				Message: fmt.Sprintf("cannot read loaded config file in %s: %s", conf.DataDir, err),
			}
		}
	}

	err = viper.Unmarshal(&conf)
	if err != nil {
		cfgError = config.Error{
			Message: fmt.Sprintf("cannot unmarshal loaded config file: %s", err),
		}
	}

	if conf.SigningKey == "" {
		log.Info("no signing key, generating one...")
		signer := ethereum.NewSignKeys()
		err = signer.Generate()
		if err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot generate signing key: %s", err),
			}
			return conf, cfgError
		}
		_, priv := signer.HexString()
		viper.Set("signingKey", priv)
		conf.SigningKey = priv
		conf.SaveConfig = true
	}

	if conf.Vochain.MinerKey == "" {
		conf.Vochain.MinerKey = conf.SigningKey
	}

	if conf.AdminToken == "" {
		conf.AdminToken = uuid.New().String()
		log.Info("created new admin API token", conf.AdminToken)
	}

	if conf.SaveConfig {
		viper.Set("saveConfig", false)
		if err := viper.WriteConfig(); err != nil {
			cfgError = config.Error{
				Message: fmt.Sprintf("cannot overwrite config file into config dir: %s", err),
			}
		}
	}

	return conf, cfgError
}

func main() {
	// Report the version before loading the config or logger init, just in case something goes wrong.
	// For the sake of including the version in the log, it's also included in a log line later on.
	fmt.Fprintf(os.Stderr, "vocdoni version %q\n", internal.Version)

	// creating config and init logger
	conf, cfgErr := newConfig()
	if conf == nil {
		log.Fatal("cannot read configuration")
	}
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
		if _, err := vochain.NewTemplateGenesisFile(dirWithNodes[0], num); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Check if errors during config creation and determine if Critical.
	log.Debugf("initializing config %+v", *conf)
	if cfgErr.Critical && cfgErr.Message != "" {
		log.Fatalf("critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message != "" {
		log.Warnf("non-critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message == "" {
		log.Infof("config file loaded successfully. Reminder: CLI flags have preference")
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
		if err = srv.Router.Init(conf.ListenHost, conf.ListenPort); err != nil {
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
		// do we need indexer?
		conf.Vochain.Indexer.Enabled = conf.Mode == types.ModeGateway
		// offchainDataDownloader is only needed for gateways
		conf.Vochain.OffChainDataDownloader = conf.Vochain.OffChainDataDownloader &&
			conf.Mode == types.ModeGateway

			// create the vochain service
		if err = srv.Vochain(); err != nil {
			log.Fatal(err)
		}
		// create the offchain data downloader service
		if conf.Vochain.OffChainDataDownloader {
			if err := srv.OffChainDataHandler(); err != nil {
				log.Fatal(err)
			}
		}
		// create the indexer service
		if conf.Vochain.Indexer.Enabled {
			// disable archive if process archive mode is enabled
			if conf.Vochain.ProcessArchive {
				conf.Vochain.Indexer.ArchiveURL = ""
			}
			if err := srv.VochainIndexer(); err != nil {
				log.Fatal(err)
			}
		}
		// create the process archiver service
		if conf.Vochain.ProcessArchive {
			if err := srv.ProcessArchiver(); err != nil {
				log.Fatal(err)
			}
		}
		// start the service and block until finish fast sync
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
			log.Errorf("add hex key failed %v", err)
			return
		}
		validator, err := srv.App.State.Validator(signer.Address(), true)
		if err != nil {
			log.Fatal(err)
		}
		if validator != nil {
			// start keykeeper service (if key index specified)
			if validator.KeyIndex > 0 {
				srv.KeyKeeper, err = keykeeper.NewKeyKeeper(
					path.Join(conf.Vochain.DataDir, "keykeeper"),
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
				uAPI.RouterHandler(),
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
