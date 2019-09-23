package chain

import (
	"context"
	"math/big"
	"strings"

	"fmt"

	voclient "gitlab.com/vocdoni/go-dvote/vochain/client"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"gitlab.com/vocdoni/go-dvote/chain"
	contract "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/log"
	app "gitlab.com/vocdoni/go-dvote/vochain"
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
	EntityAddress  [20]byte
	ProcessId      [32]byte
	MerkleTree     string
	StartBlock     uint64
	NumberOfBlocks uint64
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

			processInfo, err := o.processHandle.GetProcessData(eventProcessCreated.ProcessId)

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
			log.Debugf("process info: %+v", processInfo)
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
