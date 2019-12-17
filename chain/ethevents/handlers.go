package ethevents

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

var ethereumEventList = []string{
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

// HandleVochainOracle handles the new process creation on ethereum for the Oracle.
// Once a new process is created, the Oracle sends a transaction on the Vochain to create such process
func HandleVochainOracle(event ethtypes.Log, e *EthereumEvents) error {
	logGenesisChanged := []byte(ethereumEventList[0])
	logChainIDChanged := []byte(ethereumEventList[1])
	logProcessCreated := []byte(ethereumEventList[2])
	logProcessCanceled := []byte(ethereumEventList[3])
	logValidatorAdded := []byte(ethereumEventList[4])
	logValidatorRemoved := []byte(ethereumEventList[5])
	logOracleAdded := []byte(ethereumEventList[6])
	logOracleRemoved := []byte(ethereumEventList[7])
	logPrivateKeyPublished := []byte(ethereumEventList[8])
	logResultsPublished := []byte(ethereumEventList[9])

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
		// return nil
	case HashLogChainIDChanged.Hex():
		// return nil
	case HashLogProcessCreated.Hex():
		// Get process metadata
		processTx, err := processMeta(&e.ContractABI, &event.Data, e.ProcessHandle)
		if err != nil {
			return err
		}
		log.Infof("process meta: %+v", processTx)

		log.Debugf("signing with key: %s", e.Signer.EthAddrString())
		processTx.Signature, err = e.Signer.SignJSON(processTx)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %s", err)
		}
		tx, err := json.Marshal(processTx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %s", err)
		}
		log.Debugf("broadcasting Vochain TX: %s", string(tx))

		res, err := e.VochainCLI.BroadcastTxSync(tx)
		if err != nil {
			log.Warnf("tx cannot be broadcasted: %s", err)
		} else {
			log.Infof("new transaction hash: %s", res.Hash)
		}

	case HashLogProcessCanceled.Hex():
		// stub
		// return nil
	case HashLogValidatorAdded.Hex():
		// stub
		// return nil
	case HashLogValidatorRemoved.Hex():
		// stub
	case HashLogOracleAdded.Hex():
		log.Debug("new log event: AddOracle")
		var eventAddOracle oracleAdded
		log.Debugf("added event data: %v", event.Data)
		err := e.ContractABI.Unpack(&eventAddOracle, "OracleAdded", event.Data)
		if err != nil {
			return err
		}
		log.Debugf("addOracleEvent: %v", eventAddOracle.OraclePublicKey)
		// stub
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

func processMeta(contractABI *abi.ABI, eventData *[]byte, ph *chain.ProcessHandle) (*types.NewProcessTx, error) {
	var eventProcessCreated eventProcessCreated
	err := contractABI.Unpack(&eventProcessCreated, "ProcessCreated", *eventData)
	if err != nil {
		return nil, err
	}
	log.Debugf("processid:%x entityAddress:%x mkTree:%s",
		eventProcessCreated.ProcessId,
		eventProcessCreated.EntityAddress,
		eventProcessCreated.MerkleTree,
	)
	return ph.ProcessTxArgs(eventProcessCreated.ProcessId)
}
