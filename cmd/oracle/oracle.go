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
	"io/ioutil"
	"net/http"
	"strings"

	"fmt"
	goneturl "net/url"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	tmnode "github.com/tendermint/tendermint/node"
	dbm "github.com/tendermint/tm-db"
	"gitlab.com/vocdoni/go-dvote/config"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/util"
	vochain "gitlab.com/vocdoni/go-dvote/vochain"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"gitlab.com/vocdoni/go-dvote/chain"
	oracle "gitlab.com/vocdoni/go-dvote/chain/oracle"
	"gitlab.com/vocdoni/go-dvote/log"
)

func newConfig() (config.OracleCfg, error) {
	var cfg config.OracleCfg

	//setup flags
	usr, err := user.Current()
	if err != nil {
		return cfg, err
	}
	userDir := usr.HomeDir + "/.dvote"
	dataDir := flag.String("dataDir", userDir, "directory where data is stored")
	flag.String("logLevel", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	flag.String("logOutput", "stdout", "Log output (stdout, stderr or filepath)")
	flag.String("signingKey", "", "signing private Key (if not specified the Ethereum keystore will be used)")
	flag.String("chain", "goerli", fmt.Sprintf("Ethereum blockchain to use: %s", chain.AvailableChains))
	flag.Bool("chainLightMode", false, "synchronize Ethereum blockchain in light mode")
	flag.Int("w3nodePort", 30303, "Ethereum p2p node port to use")
	flag.String("w3external", "", "use external WEB3 endpoint. Local Ethereum node won't be initialized.")
	flag.Bool("allowPrivate", false, "allows private methods over the APIs")
	flag.String("allowedAddrs", "", "comma delimited list of allowed client ETH addresses for private methods")
	flag.String("vochainListen", "0.0.0.0:26656", "p2p host and port to listent for the voting chain")
	flag.String("vochainAddress", "", "external addrress:port to announce to other peers (automatically guessed if empty)")
	flag.String("vochainGenesis", "", "use alternative geneiss file for the voting chain")
	flag.String("vochainLogLevel", "error", "voting chain node log level")
	flag.StringArray("vochainPeers", []string{}, "coma separated list of p2p peers")
	flag.StringArray("vochainSeeds", []string{}, "coma separated list of p2p seed nodes")
	flag.String("vochainContract", "0x6f55bAE05cd2C88e792d4179C051359d02C6b34f", "voting smart contract where the oracle will listen")
	flag.String("vochainRPCListen", "127.0.0.1:26657", "rpc host and port to listent for the voting chain")

	flag.Parse()

	viper := viper.New()

	viper.SetDefault("ethereumClient.signingKey", "")
	viper.SetDefault("ethereumConfig.chainType", "goerli")
	viper.SetDefault("ethereumConfig.lightMode", false)
	viper.SetDefault("ethereumConfig.nodePort", 32000)
	viper.SetDefault("w3external", "")
	viper.SetDefault("ethereumClient.allowPrivate", false)
	viper.SetDefault("ethereumClient.allowedAddrs", "")
	viper.SetDefault("ethereumConfig.httpPort", "9091")
	viper.SetDefault("ethereumConfig.httpHost", "127.0.0.1")
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
	viper.BindPFlag("ethereumConfig.lightNode", flag.Lookup("chainLightMode"))
	viper.BindPFlag("ethereumConfig.nodePort", flag.Lookup("w3nodePort"))
	viper.BindPFlag("w3external", flag.Lookup("w3external"))
	viper.BindPFlag("ethereumClient.allowPrivate", flag.Lookup("allowPrivate"))
	viper.BindPFlag("ethereumClient.allowedAddrs", flag.Lookup("allowedAddrs"))
	viper.BindPFlag("vochainConfig.p2pListen", flag.Lookup("vochainListen"))
	viper.BindPFlag("vochainConfig.address", flag.Lookup("vochainAddress"))
	viper.BindPFlag("vochainConfig.genesis", flag.Lookup("vochainGenesis"))
	viper.BindPFlag("vochainConfig.logLevel", flag.Lookup("vochainLogLevel"))
	viper.BindPFlag("vochainConfig.peers", flag.Lookup("vochainPeers"))
	viper.BindPFlag("vochainConfig.seeds", flag.Lookup("vochainSeeds"))
	viper.BindPFlag("vochainConfig.contract", flag.Lookup("vochainContract"))
	viper.BindPFlag("vochainConfig.rpcListen", flag.Lookup("vochainRPCListen"))

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
	var app *vochain.BaseApplication

	log.Info("initializing vochain")
	// app layer db
	db, err := dbm.NewGoLevelDBWithOpts("vochain", globalCfg.VochainConfig.DataDir+"/data", nil)
	if err != nil {
		log.Fatalf("failed to open db: %s", err)
	}
	defer db.Close()

	// node + app layer
	if len(globalCfg.VochainConfig.PublicAddr) == 0 {
		ip, err := util.GetPublicIP()
		if err != nil || len(ip.String()) < 8 {
			log.Warnf("public IP discovery failed: %s", err)
		} else {
			addrport := strings.Split(globalCfg.VochainConfig.P2pListen, ":")
			if len(addrport) > 0 {
				globalCfg.VochainConfig.PublicAddr = fmt.Sprintf("%s:%s", ip.String(), addrport[len(addrport)-1])
			}
		}
	}
	if globalCfg.VochainConfig.PublicAddr != "" {
		log.Infof("public IP address: %s", globalCfg.VochainConfig.PublicAddr)
	}
	var vnode *tmnode.Node
	app, vnode = vochain.Start(globalCfg.VochainConfig, db)
	defer func() {
		vnode.Stop()
		vnode.Wait()
	}()

	// start ethereum node

	// Signing key
	var signer *sig.SignKeys
	signer = new(sig.SignKeys)
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
	if len(globalCfg.W3external) == 0 {
		log.Info("starting Ethereum node")
		node.Start()
		for i := 0; i < len(node.Keys.Accounts()); i++ {
			log.Debugf("got ethereum address: %x", node.Keys.Accounts()[i].Address)
		}
		time.Sleep(1 * time.Second)
		log.Infof("ethereum node listening on %s", node.Node.Server().NodeInfo().ListenAddr)
		log.Infof("web3 available at localhost:%d", globalCfg.EthereumConfig.NodePort)
		go func() {
			for {
				time.Sleep(15 * time.Second)
				if node.Eth != nil {
					log.Infof("[ethereum info] peers:%d synced:%t block:%s",
						node.Node.Server().PeerCount(),
						node.Eth.Synced(),
						node.Eth.BlockChain().CurrentBlock().Number())
				}
			}
		}()
	}

	if len(globalCfg.W3external) > 0 {
		url, err := goneturl.Parse(globalCfg.W3external)
		if err != nil {
			log.Fatal("cannot parse w3external URL")
		}

		log.Debugf("testing web3 endpoint %s", url.String())
		data, err := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "net_peerCount",
			"id":      74,
			"params":  []interface{}{},
		})
		if err != nil {
			log.Fatal(err)
		}
		resp, err := http.Post(globalCfg.W3external,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatal("cannot connect to web3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("successfuly connected to web3 endpoint at external url: %s", globalCfg.W3external)
	}

	// eth
	orc, err := oracle.NewOracle(node, app, nil, globalCfg.VochainConfig.Contract, nil, signer)
	if err != nil {
		log.Fatalf("couldn't create oracle: %s", err)
	}

	go func() {
		if node.Eth != nil {
			for {
				if node.Eth.Synced() {
					log.Info("ethereum node fully synced")
					log.Info("oracle startup complete")
					go orc.ReadEthereumEventLogs(0, int64(node.Eth.BlockChain().CurrentBlock().NumberU64()))
					go orc.SubscribeEthereumEventLogs()
					break
				} else {
					time.Sleep(10 * time.Second)
					log.Info("Waiting for Eth to sync before starting oracle.")
				}
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
