package oracle

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	amino "github.com/tendermint/go-amino"
	voclient "github.com/tendermint/tendermint/rpc/client"

	"gitlab.com/vocdoni/go-dvote/census"
	"gitlab.com/vocdoni/go-dvote/chain"
	contract "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/crypto/signature"
	"gitlab.com/vocdoni/go-dvote/data"
	"gitlab.com/vocdoni/go-dvote/log"
	app "gitlab.com/vocdoni/go-dvote/vochain"
)

// Oracle represents an oracle with a connection to Ethereum and Vochain
type Oracle struct {
	// voting process contract address
	contractAddress common.Address
	// voting process contract abi
	contractABI abi.ABI
	// ethereum connection
	ethereumConnection *chain.EthChainContext
	// vochain connection
	vochainConnection *voclient.HTTP
	// census connection
	censusConnection *census.CensusManager
	// voting process contract handle
	processHandle *chain.ProcessHandle
	// ethereum subscribed events
	ethereumEventList []string
	codec             *amino.Codec
	signingKeys       *signature.SignKeys
	// vochain subscribed events
	// vochainEventList []string
	// census subscribed events
	// censusEventList []string
}

// NewOracle creates an Oracle given an existing Ethereum and Vochain and/or Census connection
func NewOracle(ethCon *chain.EthChainContext, vochainApp *app.BaseApplication, cm *census.CensusManager, contractAddressHex string, storage *data.Storage, signer *signature.SignKeys) (*Oracle, error) {
	contractAddr := common.HexToAddress(contractAddressHex)
	ph, err := chain.NewVotingProcessHandle(contractAddressHex, storage)
	if err != nil {
		return new(Oracle), err
	}
	var vochainConn *voclient.HTTP
	if vochainApp != nil {
		vochainConn = voclient.NewHTTP("localhost:26657", "/websocket")
		if vochainConn == nil {
			return nil, errors.New("cannot connect to tendermint http endpoint")
		}
	}
	contractABI, err := abi.JSON(strings.NewReader(contract.VotingProcessABI))
	if err != nil {
		log.Fatal(err)
	}
	return &Oracle{
		contractAddress:    contractAddr,
		contractABI:        contractABI,
		ethereumConnection: ethCon,
		vochainConnection:  vochainConn,
		censusConnection:   cm,
		processHandle:      ph,
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
		codec:       amino.NewCodec(),
		signingKeys: signer,
	}, nil
}

type (
	eventGenesisChanged string
	eventChainIDChanged *big.Int
	eventProcessCreated struct {
		EntityAddress [20]byte
		ProcessId     [32]byte
		MerkleTree    string
	}
)

type eventProcessCanceled struct {
	EntityAddress [20]byte
	ProcessId     [32]byte
}

type (
	validatorAdded   string
	validatorRemoved string
	oracleAdded      struct {
		OraclePublicKey string
	}
)

type (
	oracleRemoved       string
	privateKeyPublished struct {
		ProcessId  [32]byte
		PrivateKey string
	}
)

type resultsPublished struct {
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
	client, err := ethclient.Dial("ws://127.0.0.1:9092")
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

// SubscribeEthereumEventLogs subscribe to the oracle
// defined smart contract via websocket. Blocking function (use go routine)
func (o *Oracle) SubscribeEthereumEventLogs() {
	// create ws client
	// client, err := ethclient.Dial(o.ethereumConnection.Node.WSEndpoint())
	client, err := ethclient.Dial("ws://127.0.0.1:9092")
	if err != nil {
		log.Fatal(err)
	}

	query := eth.FilterQuery{
		Addresses: []common.Address{o.contractAddress},
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
		case event := <-logs:
			log.Warnf("Eth event recieved: %v", event) // pointer to event log
			if o.vochainConnection != nil {
				o.handleLogEntryVochain(event)
			}
			if o.censusConnection != nil {
				o.handleLogEntryCensus(event)
			}
		}
	}
}

// ReadEthereumEventLogs reads the oracle
// defined smart contract and looks for events.
func (o *Oracle) ReadEthereumEventLogs(from, to int64) interface{} {
	log.Debug(o.ethereumConnection.Node.WSEndpoint())

	client, err := ethclient.Dial("http://localhost:9091")
	if err != nil {
		log.Fatal(err)
	}

	query := eth.FilterQuery{
		FromBlock: big.NewInt(from),
		ToBlock:   big.NewInt(to),
		Addresses: []common.Address{
			o.contractAddress,
		},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("reading logs ...")
	for _, event := range logs {
		if o.vochainConnection != nil {
			log.Debug("handling logs ...")
			err := o.handleLogEntryVochain(event)
			if err != nil {
				return err
			}
		} else {
			log.Warn("vochain connection is nil")
		}
		if o.censusConnection != nil {
			err := o.handleLogEntryCensus(event)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *Oracle) handleLogEntryVochain(event ethtypes.Log) error {
	logGenesisChanged := []byte(o.ethereumEventList[0])
	logChainIDChanged := []byte(o.ethereumEventList[1])
	logProcessCreated := []byte(o.ethereumEventList[2])
	logProcessCanceled := []byte(o.ethereumEventList[3])
	logValidatorAdded := []byte(o.ethereumEventList[4])
	logValidatorRemoved := []byte(o.ethereumEventList[5])
	logOracleAdded := []byte(o.ethereumEventList[6])
	logOracleRemoved := []byte(o.ethereumEventList[7])
	logPrivateKeyPublished := []byte(o.ethereumEventList[8])
	logResultsPublished := []byte(o.ethereumEventList[9])

	HashLogGenesisChanged := crypto.Keccak256Hash(logGenesisChanged)
	HashLogChainIDChanged := crypto.Keccak256Hash(logChainIDChanged)
	HashLogProcessCreated := crypto.Keccak256Hash(logProcessCreated)
	HashLogProcessCanceled := crypto.Keccak256Hash(logProcessCanceled)
	HashLogValidatorAdded := crypto.Keccak256Hash(logValidatorAdded)
	HashLogValidatorRemoved := crypto.Keccak256Hash(logValidatorRemoved)
	HashLogOracleAdded := crypto.Keccak256Hash(logOracleAdded)
	HashLogOracleRemoved := crypto.Keccak256Hash(logOracleRemoved)
	HashLogPrivateKeyPublished := crypto.Keccak256Hash(logPrivateKeyPublished)
	HashLogResultsPublished := crypto.Keccak256Hash(logResultsPublished)

	switch event.Topics[0].Hex() {
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
	case HashLogChainIDChanged.Hex():
		// return nil
	case HashLogProcessCreated.Hex():
		log.Debug("new log: processCreated")
		var eventProcessCreated eventProcessCreated
		err := o.contractABI.Unpack(&eventProcessCreated, "ProcessCreated", event.Data)
		if err != nil {
			log.Fatal(err)
		}
		log.Debugf("processid: %s", hex.EncodeToString(eventProcessCreated.ProcessId[:]))
		log.Debugf("event process created: %+v", eventProcessCreated)
		var topics [4]string
		for i := range event.Topics {
			topics[i] = event.Topics[i].Hex()
		}
		log.Debugf("topics: %s", topics)
		processIdx, err := o.processHandle.GetProcessIndex(eventProcessCreated.ProcessId)
		if err != nil {
			log.Error("cannot get process index from smartcontract")
		}
		log.Debugf("Process index loaded: %v", processIdx)

		processTx, err := o.processHandle.GetProcessTxArgs(eventProcessCreated.ProcessId)

		if err != nil {
			log.Errorf("Error getting process metadata: %s", err)
		} else {
			log.Infof("Process meta: %+v", processTx)
		}

		log.Debugf("signing with key: %s", o.signingKeys.EthAddrString())
		processTx.Signature, err = o.signingKeys.SignJSON(processTx)
		if err != nil {
			log.Errorf("cannot sign oracle tx: %s", err)
		}
		log.Debugf("processTx before json.Marshal HandleLogVochainEntry: %+v", processTx)
		tx, err := json.Marshal(processTx)
		if err != nil {
			log.Errorf("error marshaling process tx: %s", err)
		} else {
			log.Infof("Tx after json.Marshal HandleLogVochainEntry: %s", string(tx))
		}

		res, err := o.vochainConnection.BroadcastTxSync(tx)
		if err != nil {
			log.Warnf("result tx: %s", err)
		} else {
			log.Infof("transaction hash: %s", res.Hash)
		}

	case HashLogProcessCanceled.Hex():
		// stub
		// return nil
	case HashLogValidatorAdded.Hex():
		// stub
		// return nil
	case HashLogValidatorRemoved.Hex():
		// stub
		return nil
	case HashLogOracleAdded.Hex():
		log.Debug("new log event: AddOracle")
		var eventAddOracle oracleAdded
		log.Debugf("added event data: %v", event.Data)
		err := o.contractABI.Unpack(&eventAddOracle, "OracleAdded", event.Data)
		if err != nil {
			return err
		}
		log.Debugf("AddOracleEvent: %v", eventAddOracle.OraclePublicKey)
		// stub
		return nil
	case HashLogOracleRemoved.Hex():
		// stub
		// return nil
	case HashLogPrivateKeyPublished.Hex():
		// stub
		// return nil
	case HashLogResultsPublished.Hex():
		// stub
		// return nil
	}
	return nil
}

func (o *Oracle) handleLogEntryCensus(event ethtypes.Log) error {
	logGenesisChanged := []byte(o.ethereumEventList[0])
	logChainIDChanged := []byte(o.ethereumEventList[1])
	logProcessCreated := []byte(o.ethereumEventList[2])
	logProcessCanceled := []byte(o.ethereumEventList[3])
	logValidatorAdded := []byte(o.ethereumEventList[4])
	logValidatorRemoved := []byte(o.ethereumEventList[5])
	logOracleAdded := []byte(o.ethereumEventList[6])
	logOracleRemoved := []byte(o.ethereumEventList[7])
	logPrivateKeyPublished := []byte(o.ethereumEventList[8])
	logResultsPublished := []byte(o.ethereumEventList[9])

	HashLogGenesisChanged := crypto.Keccak256Hash(logGenesisChanged)
	HashLogChainIDChanged := crypto.Keccak256Hash(logChainIDChanged)
	HashLogProcessCreated := crypto.Keccak256Hash(logProcessCreated)
	HashLogProcessCanceled := crypto.Keccak256Hash(logProcessCanceled)
	HashLogValidatorAdded := crypto.Keccak256Hash(logValidatorAdded)
	HashLogValidatorRemoved := crypto.Keccak256Hash(logValidatorRemoved)
	HashLogOracleAdded := crypto.Keccak256Hash(logOracleAdded)
	HashLogOracleRemoved := crypto.Keccak256Hash(logOracleRemoved)
	HashLogPrivateKeyPublished := crypto.Keccak256Hash(logPrivateKeyPublished)
	HashLogResultsPublished := crypto.Keccak256Hash(logResultsPublished)

	switch event.Topics[0].Hex() {
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
	case HashLogChainIDChanged.Hex():
		// return nil
	case HashLogProcessCreated.Hex():
		log.Debug("new log: processCreated")
		var eventProcessCreated eventProcessCreated
		err := o.contractABI.Unpack(&eventProcessCreated, "ProcessCreated", event.Data)
		if err != nil {
			log.Fatal(err)
		}
		// log.Debugf("PROCESS EVENT, PROCESSID STRING: %v", hex.EncodeToString(eventProcessCreated.ProcessId[:]))

		var topics [4]string
		for i := range event.Topics {
			topics[i] = event.Topics[i].Hex()
		}
		log.Debugf("topics: %v", topics)
		processIdx, err := o.processHandle.GetProcessIndex(eventProcessCreated.ProcessId)
		if err != nil {
			log.Error("cannot get process index from smartcontract")
		}
		log.Debugf("Process index loaded: %v", processIdx)

		processInfo, err := o.processHandle.GetProcessMetadata(eventProcessCreated.ProcessId)
		if err != nil {
			log.Errorf("Error getting process metadata: %s", err)
		} else {
			log.Infof("Process meta: %s", &processInfo)
		}
		if o.censusConnection != nil {
			/*
				log.Infof("adding new namespace for published census %s", resp.Root)
				err = o.censusConnection.AddNamespace(resp.Root, r.PubKeys)
				if err != nil {
					log.Warnf("error creating local published census: %s", err)
				} else {
					log.Infof("import claims to new census")
					err = o.censusConnection.Trees[resp.Root].ImportDump(dump.ClaimsData)
					if err != nil {
						log.Warn(err)
					}
				}

				o.censusConnection.AddNamespace(processInfo)
			*/
		}

		if err != nil {
			log.Errorf("error fetching process metadata from chain module: %s", err)
		}
		log.Debugf("process info: %+v", processInfo)
		if err != nil {
			log.Warnf("cannot get process given the index: %v", err)
		}
		// log.Fatalf("PROCESS LOADED: %v", processInfo)
		_, err = o.processHandle.GetOracles()
		if err != nil {
			log.Errorf("error getting oracles: %s", err)
		}

		_, err = o.processHandle.GetValidators()
		if err != nil {
			log.Errorf("error getting validators: %s", err)
		}

	case HashLogProcessCanceled.Hex():
		// stub
		// return nil
	case HashLogValidatorAdded.Hex():
		// stub
		// return nil
	case HashLogValidatorRemoved.Hex():
		// stub
		return nil
	case HashLogOracleAdded.Hex():
		log.Debug("new log event: AddOracle")
		var eventAddOracle oracleAdded
		log.Debugf("added event data: %v", event.Data)
		err := o.contractABI.Unpack(&eventAddOracle, "OracleAdded", event.Data)
		if err != nil {
			return err
		}
		log.Debugf("AddOracleEvent: %v", eventAddOracle.OraclePublicKey)
		// stub
		return nil
	case HashLogOracleRemoved.Hex():
		// stub
		// return nil
	case HashLogPrivateKeyPublished.Hex():
		// stub
		// return nil
	case HashLogResultsPublished.Hex():
		// stub
		// return nil
	}
	return nil
}
