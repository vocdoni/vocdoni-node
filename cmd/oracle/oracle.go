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
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	nm "github.com/tendermint/tendermint/node"
	voclient "github.com/tendermint/tendermint/rpc/client"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/ethevents"
	"gitlab.com/vocdoni/go-dvote/config"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/util"
	"gitlab.com/vocdoni/go-dvote/vochain"
)

func newConfig() (config.OracleCfg, error) {
	var cfg config.OracleCfg

	// setup flags
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	userDir := home + "/.dvote"
	dataDir := flag.String("dataDir", userDir, "directory where data is stored")
	flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	flag.String("signingKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	flag.String("chain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	flag.Bool("chainLightMode", false, "synchronize Ethereum blockchain in light mode")
	flag.Int("w3nodePort", 30303, "Ethereum p2p node port to use")
	flag.String("w3external", "", "use external WEB3 endpoint. Local Ethereum node won't be initialized.")
	flag.Int("w3WSPort", 9092, "web3 websocket port")
	flag.String("vochainListen", "0.0.0.0:26656", "p2p host and port to listent for the voting chain")
	flag.String("vochainAddress", "", "external addrress:port to announce to other peers (automatically guessed if empty)")
	flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	flag.String("vochainLogLevel", "error", "voting chain node log level")
	flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	flag.String("vochainContract", "0x6f55bAE05cd2C88e792d4179C051359d02C6b34f", "voting smart contract where the oracle will listen")
	flag.String("vochainRPCListen", "127.0.0.1:26657", "rpc host and port to listent for the voting chain")
	flag.Bool("subscribeOnly", true, "oracle can read all ethereum logs or just subscribe to the new ones, by default only subscribe")

	flag.Parse()

	viper := viper.New()

	viper.SetDefault("ethereumClient.signingKey", "")
	viper.SetDefault("ethereumConfig.chainType", "goerli")
	viper.SetDefault("ethereumConfig.lightMode", false)
	viper.SetDefault("ethereumConfig.nodePort", 32000)
	viper.SetDefault("ethereumConfig.w3external", "")
	viper.SetDefault("ethereumClient.allowPrivate", false)
	viper.SetDefault("ethereumClient.allowedAddrs", "")
	viper.SetDefault("ethereumConfig.httpPort", "")
	viper.SetDefault("ethereumConfig.httpHost", "")
	viper.SetDefault("ethereumConfig.wsPort", "9092")
	viper.SetDefault("ethereumConfig.wsHost", "127.0.0.1")
	viper.SetDefault("dataDir", dataDir)
	viper.SetDefault("logLevel", "warn")
	viper.SetDefault("logOutput", "stdout")
	viper.SetDefault("vochainConfig.p2pListen", "0.0.0.0:26656")
	viper.SetDefault("vochainConfig.address", "")
	viper.SetDefault("vochainConfig.genesis", "")
	viper.SetDefault("vochainConfig.logLevel", "error")
	viper.SetDefault("vochainConfig.peers", []string{})
	viper.SetDefault("vochainConfig.seeds", []string{})
	viper.SetDefault("vochainConfig.contract", "0x6f55bAE05cd2C88e792d4179C051359d02C6b34f")
	viper.SetDefault("vochainConfig.rpcListen", "0.0.0.0:26657")
	viper.SetDefault("vochainConfig.dataDir", *dataDir+"/vochain")
	viper.SetDefault("subscribeOnly", true)

	viper.SetConfigType("yaml")

	if err = viper.SafeWriteConfigAs(*dataDir + "/oracle.yaml"); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(*dataDir, os.ModePerm)
			if err != nil {
				return cfg, err
			}
			err = viper.WriteConfigAs(*dataDir + "/oracle.yaml")
			if err != nil {
				return cfg, err
			}
		}
	}

	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("ethereumClient.signingKey", flag.Lookup("signingKey"))
	viper.BindPFlag("ethereumConfig.chainType", flag.Lookup("chain"))
	viper.BindPFlag("ethereumConfig.lightMode", flag.Lookup("chainLightMode"))
	viper.BindPFlag("ethereumConfig.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("ethereumConfig.w3external", flag.Lookup("w3external"))
	viper.BindPFlag("ethereumConfig.wsPort", flag.Lookup("w3WSPort"))
	viper.BindPFlag("vochainConfig.p2pListen", flag.Lookup("vochainListen"))
	viper.BindPFlag("vochainConfig.address", flag.Lookup("vochainAddress"))
	viper.BindPFlag("vochainConfig.genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochainConfig.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.contract", flag.Lookup("vochainContract"))
	viper.BindPFlag("vochainConfig.rpcListen", flag.Lookup("vochainRPCListen"))
	viper.BindPFlag("subscribeOnly", flag.Lookup("subscribeOnly"))

	viper.SetConfigFile(*dataDir + "/oracle.yaml")
	err = viper.ReadInConfig()
	if err != nil {
		return cfg, err
	}
	err = viper.Unmarshal(&cfg)
	return cfg, err
}

func main() {
	globalCfg, err := newConfig()
	log.InitLogger(globalCfg.LogLevel, "stdout")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}
	log.Info("starting oracle")

	// start vochain node
	log.Info("initializing vochain")
	// node + app layer
	if len(globalCfg.VochainConfig.PublicAddr) == 0 {
		ip, err := util.PublicIP()
		if err != nil {
			log.Warn(err)
		} else {
			addrport := strings.Split(globalCfg.VochainConfig.P2pListen, ":")
			if len(addrport) > 0 {
				globalCfg.VochainConfig.PublicAddr = fmt.Sprintf("%s:%s", ip, addrport[len(addrport)-1])
			}
		}
	}
	if globalCfg.VochainConfig.PublicAddr != "" {
		log.Infof("public IP address: %s", globalCfg.VochainConfig.PublicAddr)
	}
	var vnode *nm.Node
	var app *vochain.BaseApplication
	go func() {
		log.Infof("starting Vochain synchronization")
		app, vnode = vochain.NewVochain(globalCfg.VochainConfig)
		vnode.Start()
		log.Infof("vochain current height: %d", app.State.Height())
		for {
			log.Infof("[vochain info] Height:%d Mempool:%d Clients:%d",
				vnode.BlockStore().Height(),
				vnode.Mempool().Size(),
				vnode.EventBus().NumClients(),
			)
			time.Sleep(20 * time.Second)
		}
	}()
	defer func() {
		vnode.Stop()
		vnode.Wait()
	}()

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
	globalCfg.EthereumConfig.DataDir = globalCfg.DataDir
	w3cfg, err := chain.NewConfig(globalCfg.EthereumConfig)
	if err != nil {
		log.Fatal(err)
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		log.Panic(err)
	}

	// Add signing private key if exist in configuration or flags
	if globalCfg.EthereumClient.SigningKey != "" {
		log.Infof("adding custom signing key")
		err := signer.AddHexKey(globalCfg.EthereumClient.SigningKey)
		if err != nil {
			log.Fatalf("Fatal error adding hex key: %v", err)
		}
		pub, _ := signer.HexString()
		log.Infof("using custom pubKey %s", pub)
		os.RemoveAll(globalCfg.DataDir + "/.keyStore.tmp")
		node.Keys = keystore.NewPlaintextKeyStore(globalCfg.DataDir + "/.keyStore.tmp")
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
			log.Infof("using pubKey %s from keystore", pub)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	// Start Ethereum Web3 native node
	log.Info("starting Ethereum node")
	node.Start()
	for i := 0; i < len(node.Keys.Accounts()); i++ {
		log.Debugf("got ethereum address: %x", node.Keys.Accounts()[i].Address)
	}
	time.Sleep(1 * time.Second)
	log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
	log.Infof("web3 available at localhost:%d", globalCfg.EthereumConfig.NodePort)
	log.Infof("web3 WS-RPC endpoint at %s:%d", globalCfg.EthereumConfig.WsHost, globalCfg.EthereumConfig.WsPort)
	go node.PrintInfo(20 * time.Second)

	// Create Ethereum Event Log listener and register oracle handlers
	ev, err := ethevents.NewEthEvents(globalCfg.VochainConfig.Contract, signer, "")
	if err != nil {
		log.Fatalf("couldn't create ethereum  events listener: %s", err)
	}
	if len(globalCfg.W3external) > 0 {
		ev.DialAddr = globalCfg.W3external
	}
	vochainConn := voclient.NewHTTP("localhost:26657", "/websocket")
	if vochainConn == nil {
		log.Fatal("cannot connect to vochain http endpoint")
	}
	ev.VochainCLI = vochainConn

	go func() {
		for {
			height, synced, peers, _ := node.SyncInfo()
			if synced && peers > 0 && vnode != nil {
				log.Info("ethereum node fully synced")
				log.Info("oracle startup complete")
				ev.AddEventHandler(ethevents.HandleVochainOracle)
				if globalCfg.SubscribeOnly {
					go ev.SubscribeEthereumEventLogs()
				} else {
					go ev.ReadEthereumEventLogs(0, hex2int64(height))
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

func hex2int64(hexStr string) int64 {
	// remove 0x suffix if found in the input string
	cleaned := strings.Replace(hexStr, "0x", "", -1)

	// base 16 for hexadecimal
	result, _ := strconv.ParseUint(cleaned, 16, 64)
	return int64(result)
}
