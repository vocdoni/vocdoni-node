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

	voclient "github.com/tendermint/tendermint/rpc/client"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/config"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

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

	userDir := home + "/.dvote"
	// oracle
	globalCfg.SubscribeOnly = *flag.Bool("subscribeOnly", true, "oracle can read all ethereum logs or just subscribe to the new ones, by default only subscribe")
	globalCfg.ConfigFilePath = *flag.String("configFilePath", userDir+"/oracle/", "oracle config file path")
	globalCfg.DataDir = *flag.String("dataDir", userDir+"/oracle/", "sets the path indicating where to store the oracle related data")
	globalCfg.LogLevel = *flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	globalCfg.LogOutput = *flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	globalCfg.Contract = *flag.String("contract", "0x6f55bAE05cd2C88e792d4179C051359d02C6b34f", "voting smart contract where the oracle will listen")
	// vochain
	globalCfg.VochainConfig.DataDir = *flag.String("vochainDataDir", userDir+"/vochain/", "sets the path indicating where to store the Vochain related data")
	globalCfg.VochainConfig.P2PListen = *flag.String("vochainP2PListen", "0.0.0.0:26656", "vochain p2p host and port to listen on")
	globalCfg.VochainConfig.Genesis = *flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	globalCfg.VochainConfig.LogLevel = *flag.String("vochainLogLevel", "error", "voting chain node log level")
	globalCfg.VochainConfig.Peers = *flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	globalCfg.VochainConfig.Seeds = *flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	globalCfg.VochainConfig.RPCListen = *flag.String("vochainRPCListen", "0.0.0.0:26657", "vochain rpc host and port to listen on")
	globalCfg.VochainConfig.KeyFile = *flag.String("vochainKeyFile", "", "user alternative vochain p2p node key file")
	globalCfg.VochainConfig.PublicAddr = *flag.String("vochainPublicAddr", "", "IP address where the vochain node will be exposed, guessed automatically if empty")
	// ethereum
	globalCfg.EthereumConfig.DataDir = *flag.String("w3DataDir", userDir, "sets the path indicating where to store the ethereum related data")
	globalCfg.EthereumClient.SigningKey = *flag.String("w3SigningKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	globalCfg.EthereumConfig.ChainType = *flag.String("w3Chain", "goerli", fmt.Sprintf("ethereum blockchain to use: %s", chain.AvailableChains))
	globalCfg.EthereumConfig.LightMode = *flag.Bool("w3ChainLightMode", false, "synchronize Ethereum blockchain in light mode")
	globalCfg.EthereumConfig.Enabled = *flag.Bool("w3Enabled", true, "synchronize Ethereum blockchain in light mode")
	globalCfg.EthereumConfig.NodePort = *flag.Int("w3NodePort", 30303, "ethereum p2p node port to listen on")
	globalCfg.EthereumConfig.W3External = *flag.String("w3External", "", "use external web3 ethereum endpoint. Local Ethereum node won't be initialized.")
	globalCfg.EthereumConfig.WsPort = *flag.Int("w3WSPort", 9092, "ethereum websocket server port")
	globalCfg.EthereumConfig.WsHost = *flag.String("w3WSHost", "0.0.0.0", "ethereum websocket server host")
	globalCfg.EthereumConfig.HTTPPort = *flag.Int("w3HTTPPort", 9091, "ethereum http server port")
	globalCfg.EthereumConfig.HTTPHost = *flag.String("w3HTTPHost", "0.0.0.0", "ethereum http server host")
	// parse flags
	flag.Parse()
	// setting up viper
	viper := viper.New()
	viper.AddConfigPath(globalCfg.ConfigFilePath)
	viper.SetConfigName("oracle")
	viper.SetConfigType("yml")
	// binding flags to viper
	// oracle
	viper.BindPFlag("subscribeOnly", flag.Lookup("subscribeOnly"))
	viper.BindPFlag("configFilePath", flag.Lookup("configFilePath"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("contract", flag.Lookup("contract"))
	// vochain
	viper.BindPFlag("vochainConfig.dataDir", flag.Lookup("vochainDataDir"))
	viper.BindPFlag("vochainConfig.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochainConfig.p2pListen", flag.Lookup("vochainP2PListen"))
	viper.BindPFlag("vochainConfig.rpcListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("vochainConfig.publicAddr", flag.Lookup("vochainPublicAddr"))
	viper.BindPFlag("vochainConfig.genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.keyFile", flag.Lookup("vochainKeyFile"))
	// ethereum
	viper.BindPFlag("ethereumClient.signingKey", flag.Lookup("w3SigningKey"))
	viper.BindPFlag("ethereumConfig.datadir", flag.Lookup("w3DataDir"))
	viper.BindPFlag("ethereumConfig.chainType", flag.Lookup("w3Chain"))
	viper.BindPFlag("ethereumConfig.lightMode", flag.Lookup("w3ChainLightMode"))
	viper.BindPFlag("ethereumConfig.enabled", flag.Lookup("w3Enabled"))
	viper.BindPFlag("ethereumConfig.nodePort", flag.Lookup("w3NodePort"))
	viper.BindPFlag("ethereumConfig.w3External", flag.Lookup("w3External"))
	viper.BindPFlag("ethereumConfig.wsPort", flag.Lookup("w3WSPort"))
	viper.BindPFlag("ethereumConfig.wsHost", flag.Lookup("w3WSHost"))
	viper.BindPFlag("ethereumConfig.httpPort", flag.Lookup("w3HTTPPort"))
	viper.BindPFlag("ethereumConfig.httpHost", flag.Lookup("w3HTTPHost"))

	// check if config file exists
	_, err = os.Stat(globalCfg.ConfigFilePath + "oracle.yml")
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

func main() {
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

	log.Info("starting oracle")

	// start vochain node
	log.Info("initializing Vochain")
	// getting node exposed IP if not set
	if len(globalCfg.VochainConfig.PublicAddr) == 0 {
		ip, err := util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			addrport := strings.Split(globalCfg.VochainConfig.P2PListen, ":")
			if len(addrport) > 0 {
				globalCfg.VochainConfig.PublicAddr = fmt.Sprintf("%s:%s", ip, addrport[len(addrport)-1])
			}
		}
	} else {
		addrport := strings.Split(globalCfg.VochainConfig.P2PListen, ":")
		if len(addrport) > 0 {
			globalCfg.VochainConfig.PublicAddr = fmt.Sprintf("%s:%s", addrport[0], addrport[1])
		}
	}
	if globalCfg.VochainConfig.PublicAddr != "" {
		log.Infof("vochain exposed IP address: %s", globalCfg.VochainConfig.PublicAddr)
	}

	log.Infof("starting Vochain synchronization")
	vnode := vochain.NewVochain(globalCfg.VochainConfig)
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

	// ethereum
	log.Debugf("initializing ethereum")
	// Signing key
	signer := new(sig.SignKeys)
	// Add Authorized keys for private methods
	if globalCfg.EthereumClient.AllowPrivate && globalCfg.EthereumClient.AllowedAddrs != "" {
		keys := strings.Split(globalCfg.EthereumClient.AllowedAddrs, ",")
		for _, key := range keys {
			err := signer.AddAuthKey(key)
			if err != nil {
				log.Error(err)
			}
		}
	}

	// Set Ethereum node context
	w3cfg, err := chain.NewConfig(*globalCfg.EthereumConfig)
	if err != nil {
		log.Fatal(err)
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		log.Panic(err)
	}

	// Add signing private key if exist in configuration or flags
	if globalCfg.EthereumClient.SigningKey != "" {
		log.Infof("adding ethereum custom signing key")
		err := signer.AddHexKey(globalCfg.EthereumClient.SigningKey)
		if err != nil {
			log.Fatalf("fatal error adding ethereum hex key: %v", err)
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
		os.RemoveAll(globalCfg.DataDir + "/ethereum/keystore/.keyStore.tmp")
		node.Keys = keystore.NewPlaintextKeyStore(globalCfg.DataDir + "/ethereum/keystore/.keyStore.tmp")
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
			log.Infof("using ethereum pubkey %s from keystore", pub)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if globalCfg.EthereumConfig.Enabled {
		log.Info("starting Ethereum node")
		node.Start()
		for i := 0; i < len(node.Keys.Accounts()); i++ {
			log.Debugf("got ethereum address: %x", node.Keys.Accounts()[i].Address)
		}
		if len(globalCfg.EthereumConfig.W3External) == 0 {
			time.Sleep(1 * time.Second)
			log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
			log.Infof("web3 available at localhost:%d", globalCfg.EthereumConfig.NodePort)
			log.Infof("web3 WS-RPC endpoint at %s:%d", globalCfg.EthereumConfig.WsHost, globalCfg.EthereumConfig.WsPort)
		}
		go node.PrintInfo(time.Second * 15)
	}

	if globalCfg.EthereumConfig.Enabled && len(globalCfg.EthereumConfig.W3External) > 0 {
		// TO-DO create signing key since node.Start() is not executed and the ethereum account is not created on first run
		url, err := neturl.Parse(globalCfg.EthereumConfig.W3External)
		if err != nil {
			log.Fatal("cannot parse w3external URL")
		}

		log.Debugf("testing web3 endpoint %s", url)
		data, err := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "net_peerCount",
			"id":      74,
			"params":  []interface{}{},
		})
		if err != nil {
			log.Fatal(err)
		}
		resp, err := http.Post(globalCfg.EthereumConfig.W3External,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatal("cannot connect to web3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("successfuly connected to web3 endpoint at external url: %s", globalCfg.EthereumConfig.W3External)
	}

	// Create Ethereum Event Log listener and register oracle handlers
	ev, err := ethevents.NewEthEvents(globalCfg.Contract, signer, fmt.Sprintf("ws://%s:%d", globalCfg.EthereumConfig.WsHost, globalCfg.EthereumConfig.WsPort), nil)
	if err != nil {
		log.Fatalf("couldn't create ethereum  events listener: %s", err)
	}

	if len(globalCfg.EthereumConfig.W3External) > 0 {
		ev.DialAddr = globalCfg.EthereumConfig.W3External
	}

	// initializing Vochain connection
	vochainConn := voclient.NewHTTP(globalCfg.VochainConfig.RPCListen, "/websocket")
	if vochainConn == nil {
		log.Fatal("cannot connect to vochain http endpoint")
	}
	ev.VochainCLI = vochainConn

	// Wait for Vochain to be ready
	for {
		if vnode.Node != nil {
			log.Infof("vochain blockchain synchronized")
			break
		}
		time.Sleep(time.Second * 5)
		log.Infof("[synchronizing vochain] block:%d iavl-size:%d process-tree-size:%d vote-tree-size:%d",
			vnode.State.Height(), vnode.State.AppTree.Size(),
			vnode.State.ProcessTree.Size(), vnode.State.VoteTree.Size())
	}

	// wait Ethereum to be synced
	go func() {
		for {
			height, synced, peers, _ := node.SyncInfo()
			if synced && peers > 1 && vnode != nil {
				log.Info("ethereum node fully synced")
				log.Info("oracle startup complete")
				ev.AddEventHandler(ethevents.HandleVochainOracle)
				if globalCfg.SubscribeOnly {
					go ev.SubscribeEthereumEventLogs()
				} else {
					go ev.ReadEthereumEventLogs(0, util.Hex2int64(height))
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
