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
	"context"
	"encoding/json"
	"io/ioutil"
	"math/big"
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
	dbm "github.com/tendermint/tm-db"
	"gitlab.com/vocdoni/go-dvote/config"
	sig "gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/util"
	vochain "gitlab.com/vocdoni/go-dvote/vochain"
	voclient "gitlab.com/vocdoni/go-dvote/vochain/client"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"gitlab.com/vocdoni/go-dvote/chain"
	contract "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/log"
	app "gitlab.com/vocdoni/go-dvote/vochain/app"
)

// Oracle represents an oracle with a connection to Ethereum and Vochain
type Oracle struct {
	// ethereum connection
	ethereumConnection *chain.EthChainContext
	// vochain connection
	vochainConnection interface{}
	// voting process contract handle
	processHandle *chain.ProcessHandle
	// ethereum subscribed events
	ethereumEventList []string
	// vochain subscribed events
	vochainEventList []string
}

// NewOracle creates an Oracle given an existing Ethereum and Vochain connection
func NewOracle(ethCon *chain.EthChainContext, vochainApp *app.BaseApplication, contractAddressHex string) (*Oracle, error) {
	PH, err := chain.NewVotingProcessHandle(contractAddressHex)
	if err != nil {
		return new(Oracle), err
	}
	return &Oracle{
		ethereumConnection: ethCon,
		vochainConnection:  voclient.NewLocalClient(nil, vochainApp),
		processHandle:      PH,
		ethereumEventList: []string{
			"GenesisChanged(string)",
			"ChainIdChanged(uint)",
			"ProcessCreated(address,bytes32,string)",
			"ProcessCanceled(address,bytes32)",
			"ValidatorAdded(string)",
			"ValidatorRemoved(string)",
			"OracleAdded(string)",
			"OracleRemoved(string)",
			"PrivateKeyPublished(bytes32,string)",
			"ResultsPublished(bytes32,string)",
		},
	}, nil
}

type EventGenesisChanged string
type EventChainIdChanged *big.Int
type EventProcessCreated struct {
	EntityAddress [20]byte
	ProcessId     [32]byte
	MerkleTree    string
}
type EventProcessCanceled struct {
	EntityAddress [20]byte
	ProcessId     [32]byte
}
type ValidatorAdded string
type ValidatorRemoved string
type OracleAdded struct {
	OraclePublicKey string
}
type OracleRemoved string
type PrivateKeyPublished struct {
	ProcessId  [32]byte
	PrivateKey string
}
type ResultsPublished struct {
	ProcessId [32]byte
	Results   string
}

// BlockInfo represents the basic Ethereum block information
type BlockInfo struct {
	Hash     string
	Number   uint64
	Time     uint64
	Nonce    uint64
	TxNumber int
}

// NewBlockInfo creates a pointer to a new BlockInfo
func NewBlockInfo() *BlockInfo {
	return &BlockInfo{}
}

// Info retuns information about the Oracle
func (o *Oracle) Info() {}

// Start starts the oracle
func (o *Oracle) Start() {}

// GetEthereumContract gets a contract from ethereum if exists
func (o *Oracle) GetEthereumContract() {}

// GetEthereumEventList gets the ethereum events to which we are subscribed
func (o *Oracle) GetEthereumEventList() {}

// GetVochainEventList gets the vochain events to which we are subscribed
func (o *Oracle) GetVochainEventList() {}

// SendEthereumTx sends a transaction to ethereum
func (o *Oracle) SendEthereumTx() {}

// SendVochainTx sends a transaction to vochain
func (o *Oracle) SendVochainTx() {}

// EthereumBlockListener returns a block info when a new block is created on Ethereum
func (o *Oracle) EthereumBlockListener() BlockInfo {
	client, err := ethclient.Dial(o.ethereumConnection.Node.WSEndpoint())
	if err != nil {
		log.Fatal(err)
	}
	headers := make(chan *ethtypes.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case header := <-headers:
			fmt.Println(header.Hash().Hex()) // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f

			block, err := client.BlockByHash(context.Background(), header.Hash())
			if err != nil {
				log.Fatal(err)
			}

			blockInfo := NewBlockInfo()

			blockInfo.Hash = block.Hash().Hex()            // 0xbc10defa8dda384c96a17640d84de5578804945d347072e091b4e5f390ddea7f
			blockInfo.Number = block.Number().Uint64()     // 3477413
			blockInfo.Time = block.Time()                  // 1529525947
			blockInfo.Nonce = block.Nonce()                // 130524141876765836
			blockInfo.TxNumber = len(block.Transactions()) // 7
		}
	}
}

// SubscribeToEthereumContract sets the ethereum contract to which oracle should listen
// and listens for events on this contract
func (o *Oracle) SubscribeToEthereumContract(address string) {
	// create ws client
	client, err := ethclient.Dial(o.ethereumConnection.Node.WSEndpoint())
	if err != nil {
		log.Fatal(err)
	}

	contractAddress := common.HexToAddress(address)
	query := eth.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	logs := make(chan ethtypes.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-sub.Err():
			log.Fatal(err)
		case vLog := <-logs:
			fmt.Println(vLog) // pointer to event log
		}
	}
}

func (o *Oracle) ReadEthereumEventLogs(from, to int64, contractAddr string) interface{} {
	log.Debug(o.ethereumConnection.Node.WSEndpoint())

	client, err := ethclient.Dial("http://localhost:9091")
	if err != nil {
		log.Fatal(err)
	}

	contractAddress := common.HexToAddress(contractAddr)
	query := eth.FilterQuery{
		FromBlock: big.NewInt(from),
		ToBlock:   big.NewInt(to),
		Addresses: []common.Address{
			contractAddress,
		},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}

	contractABI, err := abi.JSON(strings.NewReader(contract.VotingProcessABI))
	if err != nil {
		log.Fatal(err)
	}

	votingContract, err := contract.NewVotingProcess(common.HexToAddress(contractAddr), client)
	if err != nil {
		log.Errorf("cannot get the contract at %v", contractAddr)
	}

	logGenesisChanged := []byte(o.ethereumEventList[0])
	logChainIdChanged := []byte(o.ethereumEventList[1])
	logProcessCreated := []byte(o.ethereumEventList[2])
	logProcessCanceled := []byte(o.ethereumEventList[3])
	logValidatorAdded := []byte(o.ethereumEventList[4])
	logValidatorRemoved := []byte(o.ethereumEventList[5])
	logOracleAdded := []byte(o.ethereumEventList[6])
	logOracleRemoved := []byte(o.ethereumEventList[7])
	logPrivateKeyPublished := []byte(o.ethereumEventList[8])
	logResultsPublished := []byte(o.ethereumEventList[9])

	HashLogGenesisChanged := crypto.Keccak256Hash(logGenesisChanged)
	HashLogChainIdChanged := crypto.Keccak256Hash(logChainIdChanged)
	HashLogProcessCreated := crypto.Keccak256Hash(logProcessCreated)
	HashLogProcessCanceled := crypto.Keccak256Hash(logProcessCanceled)
	HashLogValidatorAdded := crypto.Keccak256Hash(logValidatorAdded)
	HashLogValidatorRemoved := crypto.Keccak256Hash(logValidatorRemoved)
	HashLogOracleAdded := crypto.Keccak256Hash(logOracleAdded)
	HashLogOracleRemoved := crypto.Keccak256Hash(logOracleRemoved)
	HashLogPrivateKeyPublished := crypto.Keccak256Hash(logPrivateKeyPublished)
	HashLogResultsPublished := crypto.Keccak256Hash(logResultsPublished)

	for _, vLog := range logs {
		//fmt.Println(vLog.BlockHash.Hex()) // 0x3404b8c050aa0aacd0223e91b5c32fee6400f357764771d0684fa7b3f448f1a8
		//fmt.Println(vLog.BlockNumber)     // 2394201
		//fmt.Println(vLog.TxHash.Hex())    // 0x280201eda63c9ff6f305fcee51d5eb86167fab40ca3108ec784e8652a0e2b1a6

		// need to crete struct to decode raw log data
		switch vLog.Topics[0].Hex() {
		case HashLogGenesisChanged.Hex():
			/*
				log.Info("New log: GenesisChanged")
				var eventGenesisChanged EventGenesisChanged
				err := contractABI.Unpack(&eventGenesisChanged, "GenesisChanged", vLog.Data)
				if err != nil {
					log.Fatal(err)
				}
				eventGenesisChanged = EventGenesisChanged(vLog.Topics[1].String())
				return eventGenesisChanged
			*/
			//return nil
		case HashLogChainIdChanged.Hex():
			//return nil
		case HashLogProcessCreated.Hex():
			log.Debug("new log: processCreated")
			var eventProcessCreated EventProcessCreated
			err := contractABI.Unpack(&eventProcessCreated, "ProcessCreated", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}
			//log.Debugf("PROCESS EVENT, PROCESSID STRING: %v", hex.EncodeToString(eventProcessCreated.ProcessId[:]))

			var topics [4]string
			for i := range vLog.Topics {
				topics[i] = vLog.Topics[i].Hex()
			}
			log.Debugf("topics: %v", topics)
			processIdx, err := votingContract.GetProcessIndex(nil, eventProcessCreated.ProcessId)
			if err != nil {
				log.Error("cannot get process index from smartcontract")
			}
			log.Debugf("Pprocess index loaded: %v", processIdx)

			processInfo, err := o.processHandle.GetProcessMetadata(eventProcessCreated.ProcessId)

			//pinfoMarshal := []byte(processInfo.String())

			//testTx := abci.RequestDeliverTx{
			//	Tx: pinfoMarshal,
			//}

			//response := o.vochainConnection.DeliverTx(testTx)
			//log.Debugf("response deliverTX success: %t", response.IsOK())
			//processes, err := votingContract.Get(nil, eventProcessCreated.ProcessId)

			//processInfo, err := o.storage.Retrieve(processes.Metadata)
			if err != nil {
				log.Errorf("error fetching process metadata from chain module: %s", err)
			}
			log.Debugf("process info: %v", processInfo)
			if err != nil {
				log.Warnf("cannot get process given the index: %v", err)
			}
			//log.Fatalf("PROCESS LOADED: %v", processInfo)
			_, err = o.processHandle.GetOracles()
			if err != nil {
				log.Errorf("error getting oracles: %s", err)
			}

			_, err = o.processHandle.GetValidators()
			if err != nil {
				log.Errorf("error getting validators: %s", err)
			}

		case HashLogProcessCanceled.Hex():
			//stub
			//return nil
		case HashLogValidatorAdded.Hex():
			//stub
			//return nil
		case HashLogValidatorRemoved.Hex():
			//stub
			return nil
		case HashLogOracleAdded.Hex():
			log.Debug("new log event: AddOracle")
			var eventAddOracle OracleAdded
			log.Debugf("added event data: %v", vLog.Data)
			err := contractABI.Unpack(&eventAddOracle, "OracleAdded", vLog.Data)
			if err != nil {
				return err
			}
			log.Debugf("AddOracleEvent: %v", eventAddOracle.OraclePublicKey)
			//stub
			return nil
		case HashLogOracleRemoved.Hex():
			//stub
			//return nil
		case HashLogPrivateKeyPublished.Hex():
			//stub
			//return nil
		case HashLogResultsPublished.Hex():
			//stub
			//return nil
		}

	}
	return nil
}

func newConfig() (config.OracleCfg, error) {
	var cfg config.OracleCfg

	//setup flags
	usr, err := user.Current()
	if err != nil {
		return cfg, err
	}
	userDir := usr.HomeDir + "/.dvote"
	path := flag.String("cfgpath", userDir+"/oracle.yaml", "filepath for custom gateway config")
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
	flag.String("vochainContract", "0x3eF4dE917a6315c1De87b02FD8b19EACef324c3b", "voting smart contract where the oracle will listen")
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
	viper.SetDefault("dataDir", userDir)
	viper.SetDefault("logLevel", "warn")
	viper.SetDefault("logOutput", "stdout")
	viper.SetDefault("vochainConfig.p2pListen", "0.0.0.0:26656")
	viper.SetDefault("vochainConfig.address", "")
	viper.SetDefault("vochainConfig.genesis", "")
	viper.SetDefault("vochainConfig.logLevel", "error")
	viper.SetDefault("vochainConfig.peers", []string{})
	viper.SetDefault("vochainConfig.seeds", []string{})
	viper.SetDefault("vochainConfig.contract", "0x3eF4dE917a6315c1De87b02FD8b19EACef324c3b")
	viper.SetDefault("vochainConfig.rpcListen", "0.0.0.0:26657")
	viper.SetDefault("vochainConfig.dataDir", *dataDir+"/vochain")

	viper.SetConfigType("yaml")

	if err = viper.SafeWriteConfigAs(*path); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(userDir, os.ModePerm)
			if err != nil {
				return cfg, err
			}
			err = viper.WriteConfigAs(*path)
			if err != nil {
				return cfg, err
			}
		}
	}

	viper.BindPFlag("logLevel", flag.Lookup("logLevel"))
	viper.BindPFlag("dataDir", flag.Lookup("dataDir"))
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
	viper.BindPFlag("ethereumClient.signingKey", flag.Lookup("signingKey"))
	viper.BindPFlag("logOutput", flag.Lookup("logOutput"))
	viper.BindPFlag("vochainConfig.rpcListen", flag.Lookup("vochainRPCListen"))

	viper.SetConfigFile(*path)
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
	var app *app.BaseApplication
	db, err := dbm.NewGoLevelDBWithOpts("vochain", globalCfg.DataDir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	if len(globalCfg.VochainConfig.PublicAddr) == 0 {
		ip, err := util.GetPublicIP()

		if err != nil || len(ip.String()) < 8 {
			log.Warnf("public IP discovery failed: %s", err.Error())
		} else {
			addrport := strings.Split(globalCfg.VochainConfig.P2pListen, ":")
			if len(addrport) > 0 {
				globalCfg.VochainConfig.PublicAddr = fmt.Sprintf("%s:%s", ip.String(), addrport[len(addrport)-1])
				log.Infof("public IP address: %s", globalCfg.VochainConfig.PublicAddr)
			}
		}
	}

	log.Debugf("initializing vochain with tendermint config %s", globalCfg.VochainConfig)
	_, vnode := vochain.Start(globalCfg.VochainConfig, db)
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
				log.Error(err.Error())
			}
		}
	}

	// Set Ethereum node context
	globalCfg.EthereumConfig.DataDir = globalCfg.DataDir
	w3cfg, err := chain.NewConfig(globalCfg.EthereumConfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	node, err := chain.Init(w3cfg)
	if err != nil {
		log.Panic(err.Error())
	}

	// Add signing private key if exist in configuration or flags
	if globalCfg.EthereumClient.SigningKey != "" {
		log.Infof("adding custom signing key")
		err := signer.AddHexKey(globalCfg.EthereumClient.SigningKey)
		if err != nil {
			log.Fatalf("Fatal error adding hex key: %v", err.Error())
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
				log.Fatal(err.Error())
			}
			err = addKeyFromEncryptedJSON(keyJSON, "", signer)
			pub, _ := signer.HexString()
			log.Infof("using pubKey %s from keystore", pub)
			if err != nil {
				log.Fatalf(err.Error())
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
			log.Fatal(err.Error())
		}
		resp, err := http.Post(globalCfg.W3external,
			"application/json", strings.NewReader(string(data)))
		if err != nil {
			log.Fatal("cannot connect to web3 endpoint")
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err.Error())
		}
		log.Infof("successfuly connected to web3 endpoint at external url: %s", globalCfg.W3external)
	}

	// wait chains sync
	// eth
	orc, err := NewOracle(node, app, globalCfg.VochainConfig.Contract)
	if err != nil {
		log.Fatalf("couldn't create oracle: %s", err.Error())
	}

	go func() {
		if node.Eth != nil {
			for {
				if node.Eth.Synced() {
					log.Info("ethereum node fully synced, starting Oracle")
					orc.ReadEthereumEventLogs(1000000, 1314200, globalCfg.VochainConfig.Contract)
					return
				}
				time.Sleep(10 * time.Second)
				log.Debug("waiting for ethereum to sync before starting Oracle")
			}
		} else {
			time.Sleep(time.Second * 1)
		}
	}()

	// tendermint

	// end wait chains sync

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)

	for {
		time.Sleep(1 * time.Second)
	}
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
