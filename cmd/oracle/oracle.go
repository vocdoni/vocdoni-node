package main

// CONNECT TO WEB3
// CONNECT TO TENDERMINT

// INSTANTIATE THE CONTRACT

// GET METHODS FOR THE CONTRACT
//		PROCESS
//		VALIDATORS
// 		ORACLES

// SUBSCRIBE TO EVENTS

// CREATE TM TX BASED ON EVENTS

// WRITE TO ETH SM IF PROCESS FINISHED

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	voclient "github.com/tendermint/tendermint/rpc/client"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/config"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

const ensRegistryAddr = "0x00000000000C2E074eC69A0dFb2997BA6C7d2e1e"

func newConfig() (*config.OracleCfg, config.Error) {
	var err error
	var cfgError config.Error
	// create base config
	globalCfg := config.NewOracleCfg()
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

	// oracle
	flag.StringVar(&globalCfg.DataDir, "dataDir", home+"/.dvote", "directory where data is stored")
	flag.BoolVar(&globalCfg.Dev, "dev", false, "run and connect to the development network")
	globalCfg.SubscribeOnly = *flag.Bool("subscribeOnly", true, "oracle can read all ethereum logs or just subscribe to the new ones, by default only subscribe")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.SaveConfig = *flag.Bool("saveConfig", false, "overwrites an existing config file with the CLI provided flags")
	globalCfg.EthProcessDomain = *flag.String("ethProcessDomain", "voting-process.vocdoni.eth", "voting contract ENS domain")

	// vochain
	globalCfg.VochainConfig.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656", "vochain p2p host and port to listen on")
	globalCfg.VochainConfig.Genesis = *flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	globalCfg.VochainConfig.LogLevel = *flag.String("vochainLogLevel", "error", "voting chain node log level")
	globalCfg.VochainConfig.Peers = *flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	globalCfg.VochainConfig.Seeds = *flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.VochainConfig.RPCListen = *flag.String("vochainRPCListen", "0.0.0.0:26657", "vochain rpc host and port to listen on")
	globalCfg.VochainConfig.KeyFile = *flag.String("vochainKeyFile", "", "user alternative vochain p2p node key file")
	globalCfg.VochainConfig.PublicAddr = *flag.String("vochainPublicAddr", "", "IP address where the vochain node will be exposed, guessed automatically if empty")

	// ethereum
	globalCfg.EthConfig.SigningKey = *flag.String("ethSigningKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	globalCfg.EthConfig.ChainType = *flag.String("ethChain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	globalCfg.EthConfig.LightMode = *flag.Bool("ethChainLightMode", false, "synchronize Ethereum blockchain in light mode")
	globalCfg.EthConfig.NodePort = *flag.Int("ethNodePort", 30303, "Ethereum p2p node port to listen on")
	globalCfg.EthConfig.BootNodes = *flag.StringArray("ethBootnodes", []string{}, "Ethereum p2p custom bootstrap nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.TrustedPeers = *flag.StringArray("ethTrustedPeers", []string{}, "Ethereum p2p trusted peer nodes (enode://<pubKey>@<ip>[:port])")
	globalCfg.EthConfig.DataDir = globalCfg.DataDir + "/ethereum"

	// web3
	globalCfg.W3Config.WsPort = *flag.Int("w3WsPort", 9092, "web3 websocket server port")
	globalCfg.W3Config.WsHost = *flag.String("w3WsHost", "0.0.0.0", "web3 websocket server host")
	globalCfg.W3Config.HTTPPort = *flag.Int("w3HTTPPort", 9091, "web3 http server port")
	globalCfg.W3Config.HTTPHost = *flag.String("w3HTTPHost", "0.0.0.0", "web3 http server host")
	// parse flags
	flag.Parse()

	if globalCfg.Dev {
		globalCfg.DataDir += "/dev"
	}

	// setting up viper
	viper := viper.New()
	viper.AddConfigPath(globalCfg.DataDir)
	viper.SetConfigName("oracle")
	viper.SetConfigType("yml")

	// oracle
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("subscribeOnly", flag.Lookup("subscribeOnly"))
	viper.BindPFlag("saveConfig", flag.Lookup("saveConfig"))
	viper.BindPFlag("ethProcessDomain", flag.Lookup("ethProcessDomain"))
	viper.BindPFlag("dev", flag.Lookup("dev"))

	// vochain
	viper.Set("vochainConfig.dataDir", globalCfg.DataDir+"/vochain")
	viper.BindPFlag("vochainConfig.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochainConfig.p2pListen", flag.Lookup("vochainP2PListen"))
	viper.BindPFlag("vochainConfig.rpcListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("vochainConfig.publicAddr", flag.Lookup("vochainPublicAddr"))
	viper.BindPFlag("vochainConfig.genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.keyFile", flag.Lookup("vochainKeyFile"))
	viper.BindPFlag("vochainConfig.Dev", flag.Lookup("dev"))

	// ethereum
	viper.Set("ethConfig.datadir", globalCfg.DataDir+"/ethereum")
	viper.BindPFlag("ethConfig.signingKey", flag.Lookup("ethSigningKey"))
	viper.BindPFlag("ethConfig.chainType", flag.Lookup("ethChain"))
	viper.BindPFlag("ethConfig.lightMode", flag.Lookup("ethChainLightMode"))
	viper.BindPFlag("ethConfig.nodePort", flag.Lookup("ethNodePort"))
	viper.BindPFlag("ethConfig.trustedPeers", flag.Lookup("ethTrustedPeers"))
	viper.BindPFlag("ethConfig.bootnodes", flag.Lookup("ethBootnodes"))

	viper.Set("w3Config.enabled", true)
	viper.BindPFlag("w3Config.wsPort", flag.Lookup("w3WsPort"))
	viper.BindPFlag("w3Config.wsHost", flag.Lookup("w3WsHost"))
	viper.BindPFlag("w3Config.httpPort", flag.Lookup("w3HTTPPort"))
	viper.BindPFlag("w3Config.httpHost", flag.Lookup("w3HTTPHost"))

	// check if config file exists
	_, err = os.Stat(globalCfg.DataDir + "/oracle.yml")
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
	// creating config and init logger
	globalCfg, cfgErr := newConfig()
	if globalCfg == nil {
		panic("cannot read configuration")
	}
	fmt.Println(globalCfg.LogLevel)
	log.Init(globalCfg.LogLevel, globalCfg.LogOutput)

	// check if errors during config creation and determine if Critical
	if cfgErr.Critical && cfgErr.Message != "" {
		log.Fatalf("Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message != "" {
		log.Warnf("non Critical error loading config: %s", cfgErr.Message)
	} else if !cfgErr.Critical && cfgErr.Message == "" {
		log.Infof("config file loaded successfully, remember CLI flags have preference")
	}

	log.Info("starting oracle")

	if globalCfg.Dev {
		log.Info("using development mode")
	}

	// start vochain node
	log.Info("initializing Vochain")
	// getting node exposed IP if not set
	if len(globalCfg.VochainConfig.PublicAddr) == 0 {
		ip, err := util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			_, port, err := net.SplitHostPort(globalCfg.VochainConfig.P2PListen)
			if err == nil {
				globalCfg.VochainConfig.PublicAddr = net.JoinHostPort(ip.String(), port)
			}
		}
	} else {
		host, port, err := net.SplitHostPort(globalCfg.VochainConfig.P2PListen)
		if err == nil {
			globalCfg.VochainConfig.PublicAddr = net.JoinHostPort(host, port)
		}
	}
	if globalCfg.VochainConfig.PublicAddr != "" {
		log.Infof("vochain exposed IP address: %s", globalCfg.VochainConfig.PublicAddr)
	}

	log.Infof("starting Vochain synchronization")
	var vnode *vochain.BaseApplication
	if globalCfg.Dev {
		vnode = vochain.NewVochain(globalCfg.VochainConfig, []byte(vochain.DevelopmentGenesis1), nil)
	} else {
		vnode = vochain.NewVochain(globalCfg.VochainConfig, []byte(vochain.TestnetGenesis1), nil)
	}
	go func() {
		log.Infof("vochain current height: %d", vnode.State.Height())
		for {
			if vnode.Node != nil {
				log.Infof("[vochain info] Height:%d Mempool:%d AppTree:%d ProcessTree:%d VoteTree:%d",
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

	// Ethereum
	log.Debugf("initializing ethereum")
	// Signing key
	signer := new(sig.SignKeys)
	// Set Ethereum node context
	w3cfg, err := chain.NewConfig(globalCfg.EthConfig, globalCfg.W3Config)
	if err != nil {
		log.Fatal(err)
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Add signing private key if exist in configuration or flags
	if globalCfg.EthConfig.SigningKey != "" {
		log.Infof("adding ethereum custom signing key")
		err := signer.AddHexKey(globalCfg.EthConfig.SigningKey)
		if err != nil {
			log.Fatalf("fatal error adding ethereum hex key: %v", err)
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
		os.RemoveAll(globalCfg.EthConfig.DataDir + "/keystore/tmp")
		node.Keys = keystore.NewPlaintextKeyStore(globalCfg.EthConfig.DataDir + "/keystore/tmp")
		node.Keys.ImportECDSA(signer.Private, "")
		defer os.RemoveAll(globalCfg.EthConfig.DataDir + "/keystore/tmp")
	} else {
		// Get stored keys from Ethereum node context
		acc := node.Keys.Accounts()
		if len(acc) > 0 {
			keyJSON, err := node.Keys.Export(acc[0], "", "")
			if err != nil {
				log.Fatalf("cannot open JSON keystore from %s (%s)", acc[0], err)
			}
			err = addKeyFromEncryptedJSON(keyJSON, "", signer)
			pub, _ := signer.HexString()
			log.Infof("using ethereum pubkey %s from keystore", pub)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Info("starting Ethereum node")
	node.Start()
	for i := 0; i < len(node.Keys.Accounts()); i++ {
		log.Debugf("got ethereum address: %x", node.Keys.Accounts()[i].Address)
	}
	log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
	log.Infof("web3 available at localhost:%d", globalCfg.EthConfig.NodePort)
	log.Infof("web3 WS-RPC endpoint at %s:%d", globalCfg.W3Config.WsHost, globalCfg.W3Config.WsPort)
	go node.PrintInfo(time.Second * 20)

	// initializing Vochain connection
	vochainConn, err := voclient.NewHTTP("tcp://"+globalCfg.VochainConfig.RPCListen, "/websocket")

	if err != nil {
		log.Fatal(err)
	}

	// Wait for Vochain to be ready
	for {
		if vnode.Node != nil {
			log.Infof("vochain blockchain synchronized")
			break
		}
		time.Sleep(time.Second * 10)
		log.Infof("[synchronizing vochain] block:%d iavl-size:%d process-tree-size:%d vote-tree-size:%d",
			vnode.State.Height(), vnode.State.AppTree.Size(),
			vnode.State.ProcessTree.Size(), vnode.State.VoteTree.Size())
	}

	// wait Ethereum to be synced
	go func() {
		for {
			info, _ := node.SyncInfo()
			if info.Synced && info.Peers > 1 && info.Height > 0 && vnode != nil {
				log.Info("ethereum node fully synced, oracle startup complete")
				if err != nil {
					log.Fatalf("cannot read logs, ethereum last block parsing failed: %d at block %d", err, info.Height)
				}
				// get voting contract
				votingProcessAddr, err := chain.VotingProcessAddress(
					ensRegistryAddr, globalCfg.EthProcessDomain, fmt.Sprintf("http://%s:%d", w3cfg.HTTPHost, w3cfg.HTTPPort))
				if err != nil || votingProcessAddr == "" {
					log.Warnf("cannot get voting process contract: %s", err)
				} else {
					log.Infof("loaded voting contract at address: %s", votingProcessAddr)
				}

				// Create Ethereum Event Log listener and register oracle handlers
				ev, err := ethevents.NewEthEvents(
					votingProcessAddr, signer, fmt.Sprintf("ws://%s:%d", globalCfg.W3Config.WsHost, globalCfg.W3Config.WsPort), nil)
				if err != nil {
					log.Fatalf("couldn't create ethereum  events listener: %s", err)
				}
				ev.VochainCLI = vochainConn
				ev.AddEventHandler(ethevents.HandleVochainOracle)
				if globalCfg.SubscribeOnly {
					log.Infof("reading ethereum events from current block %d", info.Height)
					go ev.SubscribeEthereumEventLogs()
				} else {
					log.Infof("reading ethereum events from block 0 to %d", info.Height)
					go ev.ReadEthereumEventLogs(0, int64(info.Height))
					go ev.SubscribeEthereumEventLogs()
				}
				break
			} else {
				time.Sleep(10 * time.Second)
				log.Info("waiting for Ethereum and Vochain to sync before starting oracle")
			}
		}
	}()

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)
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
