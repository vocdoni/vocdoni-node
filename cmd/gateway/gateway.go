package main

import (
	"fmt"
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
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/service"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"gitlab.com/vocdoni/go-dvote/vochain/scrutinizer"
)

var ethNoWaitSync bool

const ensRegistryAddr = "0x00000000000C2E074eC69A0dFb2997BA6C7d2e1e"

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
	// Booleans should be passed to the CLI as: var=True/false

	// gateway
	flag.StringVar(&globalCfg.DataDir, "dataDir", home+"/.dvote", "directory where data is stored")
	flag.BoolVar(&globalCfg.Dev, "dev", true, "run and connect to the development network")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.ListenHost = *flag.String("listenHost", "0.0.0.0", "API endpoint listen address")
	globalCfg.ListenPort = *flag.Int("listenPort", 9090, "API endpoint http port")
	globalCfg.CensusSync = *flag.Bool("censusSync", true, "automatically import new census published on smart contract")
	globalCfg.SaveConfig = *flag.Bool("saveConfig", false, "overwrites an existing config file with the CLI provided flags")
	globalCfg.EthProcessDomain = *flag.String("ethProcessDomain", "voting-process.vocdoni.eth", "voting contract ENS domain")
	// api
	globalCfg.API.File = *flag.Bool("fileApi", true, "enable file API")
	globalCfg.API.Census = *flag.Bool("censusApi", true, "enable census API")
	globalCfg.API.Vote = *flag.Bool("voteApi", true, "enable vote API")
	globalCfg.API.Results = *flag.Bool("resultsApi", true, "enable results API")
	globalCfg.API.Route = *flag.String("apiRoute", "/dvote", "dvote API route")
	globalCfg.API.AllowPrivate = *flag.Bool("apiAllowPrivate", false, "allows private methods over the APIs")
	globalCfg.API.AllowedAddrs = *flag.String("apiAllowedAddrs", "", "comma delimited list of allowed client ETH addresses for private methods")
	// ethereum node
	globalCfg.EthConfig.SigningKey = *flag.String("ethSigningKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	globalCfg.EthConfig.ChainType = *flag.String("ethChain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	globalCfg.EthConfig.LightMode = *flag.Bool("ethChainLightMode", false, "synchronize Ethereum blockchain in light mode")
	globalCfg.EthConfig.NodePort = *flag.Int("ethNodePort", 30303, "Ethereum p2p node port to use")
	globalCfg.EthConfig.BootNodes = *flag.StringArray("ethBootNodes", []string{}, "Ethereum p2p custom bootstrap nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.TrustedPeers = *flag.StringArray("ethTrustedPeers", []string{}, "Ethereum p2p trusted peer nodes (enode://<pubKey>@<ip>[:port])")
	flag.BoolVar(&ethNoWaitSync, "ethNoWaitSync", false, "do not wait for Ethereum to synchronize (for testing only)")
	// ethereum web3
	globalCfg.W3Config.Enabled = *flag.Bool("w3Enabled", true, "if true web3 will be enabled")
	globalCfg.W3Config.Route = *flag.String("w3Route", "/web3", "web3 endpoint API route")
	globalCfg.W3Config.WsPort = *flag.Int("w3WsPort", 9092, "web3 websocket port")
	globalCfg.W3Config.WsHost = *flag.String("w3WsHost", "0.0.0.0", "web3 websocket host")
	globalCfg.W3Config.HTTPPort = *flag.Int("w3HTTPPort", 9091, "ethereum http server port")
	globalCfg.W3Config.HTTPHost = *flag.String("w3HTTPHost", "0.0.0.0", "ethereum http server host")
	// ipfs
	globalCfg.Ipfs.NoInit = *flag.Bool("ipfsNoInit", false, "disables inter planetary file system support")
	globalCfg.Ipfs.SyncKey = *flag.String("ipfsSyncKey", "", "enable IPFS cluster synchronization using the given secret key")
	globalCfg.Ipfs.SyncPeers = *flag.StringArray("ipfsSyncPeers", []string{}, "use custom ipfsSync peers/bootnodes for accessing the DHT")
	// ssl
	globalCfg.Ssl.Domain = *flag.String("sslDomain", "", "enable SSL secure domain with LetsEncrypt auto-generated certificate (listenPort=443 is required)")
	// vochain
	globalCfg.VochainConfig.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656", "p2p host and port to listent for the voting chain")
	globalCfg.VochainConfig.PublicAddr = *flag.String("vochainPublicAddr", "", "external addrress:port to announce to other peers (automatically guessed if empty)")
	globalCfg.VochainConfig.RPCListen = *flag.String("vochainRPCListen", "0.0.0.0:26657", "rpc host and port to listent for the voting chain")
	globalCfg.VochainConfig.CreateGenesis = *flag.Bool("vochainCreateGenesis", false, "create own/testing genesis file on vochain")
	globalCfg.VochainConfig.Genesis = *flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	globalCfg.VochainConfig.LogLevel = *flag.String("vochainLogLevel", "error", "voting chain node log level")
	globalCfg.VochainConfig.Peers = *flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	globalCfg.VochainConfig.Seeds = *flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.VochainConfig.MinerKey = *flag.String("vochainKey", "", "user alternative vochain private key (hexstring[64])")

	// parse flags
	flag.Parse()

	if globalCfg.Dev {
		globalCfg.DataDir += "/dev"
	}
	// setting up viper
	viper := viper.New()
	viper.AddConfigPath(globalCfg.DataDir)
	viper.SetConfigName("gateway")
	viper.SetConfigType("yml")
	// binding flags to viper
	// gateway
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("listenHost", flag.Lookup("listenHost"))
	viper.BindPFlag("listenPort", flag.Lookup("listenPort"))
	viper.BindPFlag("censusSync", flag.Lookup("censusSync"))
	viper.BindPFlag("saveConfig", flag.Lookup("saveConfig"))
	viper.BindPFlag("ethProcessDomain", flag.Lookup("ethProcessDomain"))
	viper.BindPFlag("dev", flag.Lookup("dev"))

	// api
	viper.BindPFlag("api.file", flag.Lookup("fileApi"))
	viper.BindPFlag("api.census", flag.Lookup("censusApi"))
	viper.BindPFlag("api.vote", flag.Lookup("voteApi"))
	viper.BindPFlag("api.results", flag.Lookup("resultsApi"))
	viper.BindPFlag("api.route", flag.Lookup("apiRoute"))
	viper.BindPFlag("api.allowPrivate", flag.Lookup("apiAllowPrivate"))
	viper.BindPFlag("api.allowedAddrs", flag.Lookup("apiAllowedAddrs"))
	// ethereum node
	viper.Set("ethConfig.datadir", globalCfg.DataDir+"/ethereum")
	viper.BindPFlag("ethConfig.signingKey", flag.Lookup("ethSigningKey"))
	viper.BindPFlag("ethConfig.chainType", flag.Lookup("ethChain"))
	viper.BindPFlag("ethConfig.lightMode", flag.Lookup("ethChainLightMode"))
	viper.BindPFlag("ethConfig.nodePort", flag.Lookup("ethNodePort"))
	viper.BindPFlag("ethConfig.bootNodes", flag.Lookup("ethBootNodes"))
	viper.BindPFlag("ethConfig.trustedPeers", flag.Lookup("ethTrustedPeers"))
	// ethereum web3
	viper.BindPFlag("w3Config.route", flag.Lookup("w3Route"))
	viper.BindPFlag("w3Config.enabled", flag.Lookup("w3Enabled"))
	viper.BindPFlag("w3Config.wsPort", flag.Lookup("w3WsPort"))
	viper.BindPFlag("w3Config.wsHost", flag.Lookup("w3WsHost"))
	viper.BindPFlag("w3Config.httpPort", flag.Lookup("w3HTTPPort"))
	viper.BindPFlag("w3Config.httpHost", flag.Lookup("w3HTTPHost"))
	// ssl
	viper.Set("ssl.dirCert", globalCfg.DataDir+"/tls")
	viper.BindPFlag("ssl.domain", flag.Lookup("sslDomain"))
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
	viper.BindPFlag("vochainConfig.MinerKey", flag.Lookup("vochainKey"))
	viper.BindPFlag("vochainConfig.Dev", flag.Lookup("dev"))

	// check if config file exists
	_, err = os.Stat(globalCfg.DataDir + "/gateway.yml")
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
		var signer signature.SignKeys
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

func main() {
	// setup config
	// creating config and init logger
	globalCfg, cfgErr := newConfig()
	if globalCfg == nil {
		panic("cannot read configuration")
	}
	log.Init(globalCfg.LogLevel, globalCfg.LogOutput)
	log.Debugf("initializing gateway config %+v", *globalCfg)

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

	log.Info("starting gateway")

	// Ensure we can have at least 8k open files. This is necessary, since
	// many components like IPFS and Tendermint require keeping many active
	// connections. Some systems have low defaults like 1024, which can make
	// the program crash after it's been running for a bit.
	if err := ensureNumberFiles(8000); err != nil {
		log.Fatalf("could not ensure we can have enough open files: %v", err)
	}

	// Signing key
	signer := new(signature.SignKeys)
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
			log.Fatalf("error adding hex key: %s", err)
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
	} else {
		log.Fatal("no private key or wrong key (size != 256 bits)")
	}

	// Proxy service
	pxy, err := service.Proxy(globalCfg.ListenHost, globalCfg.ListenPort, globalCfg.Ssl.Domain, globalCfg.Ssl.DirCert)
	if err != nil {
		log.Fatal(err)
	}

	// Ethereum service
	node, err := service.Ethereum(globalCfg.EthConfig, globalCfg.W3Config, pxy, signer)
	if err != nil {
		log.Fatal(err)
	}

	// Storage service
	storage, err := service.IPFS(globalCfg.Ipfs, signer)
	if err != nil {
		log.Fatal(err)
	}

	// Census service
	var cm *census.Manager
	if globalCfg.API.Census {
		cm, err = service.Census(globalCfg.DataDir)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Vochain and Scrutinizer service
	var vnode *vochain.BaseApplication
	var sc *scrutinizer.Scrutinizer
	if globalCfg.API.Vote {
		vnode, sc, err = service.Vochain(globalCfg.VochainConfig, globalCfg.Dev, globalCfg.API.Results)
		if err != nil {
			log.Fatal(err)
		}
		// Wait for Vochain to be ready
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
				h, (h-hPrev)/5)
		}
	}

	// Wait for Ethereum to be ready
	if !ethNoWaitSync {
		for {
			if info, _ := node.SyncInfo(); info.Synced && info.Peers > 0 && info.Height > 0 {
				log.Infof("ethereum blockchain synchronized")
				break
			}
			time.Sleep(time.Second * 5)
		}
	}

	// dvote API service
	if globalCfg.API.File || globalCfg.API.Census || globalCfg.API.Vote {
		if err := service.API(globalCfg.API, pxy, storage, cm, sc, globalCfg.VochainConfig.RPCListen, signer); err != nil {
			log.Fatal(err)
		}
	}

	log.Info("gateway startup complete")

	// Ethereum events service (needs Ethereum synchronized)
	if !ethNoWaitSync && globalCfg.CensusSync { // Or other kind of events

		var evh []ethevents.EventHandler
		if globalCfg.CensusSync && !globalCfg.API.Census {
			log.Fatal("censusSync function requires the census API enabled")
		} else {
			evh = append(evh, ethevents.HandleCensus)
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
		if err := service.EthEvents(globalCfg.EthProcessDomain, ensRegistryAddr, globalCfg.W3Config.WsHost,
			globalCfg.W3Config.WsPort, initBlock, int64(syncInfo.Height), true, cm, evh); err != nil {
			log.Fatal(err)
		}
	}

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
