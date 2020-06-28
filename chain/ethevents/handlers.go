package ethevents

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
	"gitlab.com/vocdoni/go-dvote/vochain"
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
	eventProcessCreated struct {
		EntityAddress [20]byte
		ProcessId     [32]byte
		MerkleTree    string
	}
	eventProcessCanceled struct {
		EntityAddress [20]byte
		ProcessId     [32]byte
	}
	oracleAdded struct {
		OraclePublicKey string
	}
	privateKeyPublished struct {
		ProcessId  [32]byte
		PrivateKey string
	}
	resultsPublished struct {
		ProcessId [32]byte
		Results   string
	}
)

// HandleVochainOracle handles the new process creation on ethereum for the Oracle.
// Once a new process is created, the Oracle sends a transaction on the Vochain to create such process
func HandleVochainOracle(event *ethtypes.Log, e *EthereumEvents) error {
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
		processTx, err := processMeta(&e.ContractABI, event.Data, e.ProcessHandle)
		if err != nil {
			return err
		}

		// Check if process already exist
		log.Infof("found new process on Ethereum\n\t%+v", *processTx)
		_, err = e.VochainApp.State.Process(processTx.ProcessID, true)
		if err != nil {
			if err != vochain.ErrProcessNotFound {
				return err
			}
		} else {
			log.Infof("process already exist, skipping")
			return nil
		}

		processTx.Signature, err = e.Signer.SignJSON(processTx)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: (%s)", err)
		}
		tx, err := json.Marshal(processTx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: (%s)", err)
		}
		log.Debugf("broadcasting Vochain TX: %s", tx)

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			log.Warnf("cannot broadcast tx: (%s)", err)
			return fmt.Errorf("cannot broadcast tx: (%s), res: (%+v)", err, res)
		} else {
			log.Infof("oracle transaction sent, hash:%s", res.Hash)
		}

	case HashLogProcessCanceled.Hex():
		cancelProcessTx, err := cancelProcessMeta(&e.ContractABI, event.Data, e.ProcessHandle)
		if err != nil {
			return err
		}

		log.Infof("found new cancel process order from ethereum\n\t%+v", *cancelProcessTx)
		p, err := e.VochainApp.State.Process(cancelProcessTx.ProcessID, true)
		if err != nil {
			return err
		}
		if p.Canceled {
			log.Infof("process already canceled, skipping")
			return nil
		}
		cancelProcessTx.Signature, err = e.Signer.SignJSON(cancelProcessTx)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: (%s)", err)
		}
		tx, err := json.Marshal(cancelProcessTx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %s", err)
		}
		log.Debugf("broadcasting Vochain tx\n\t%s", string(tx))

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			log.Warnf("cannot broadcast tx: (%s)", err)
			return fmt.Errorf("cannot broadcast tx: (%s), res: (%+v)", err, res)
		} else {
			log.Infof("oracle transaction sent, hash:%s", res.Hash)
		}
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
		var _ privateKeyPublished
		// stub
		// return nil
	case HashLogResultsPublished.Hex():
		var _ resultsPublished
		// stub
		// return nil
	}
	return nil
}

// HandleCensus handles the import of census merkle trees published in Ethereum
func HandleCensus(event *ethtypes.Log, e *EthereumEvents) error {
	logProcessCreated := []byte(ethereumEventList[2])
	// Only handle processCreated event
	if event.Topics[0].Hex() != crypto.Keccak256Hash(logProcessCreated).Hex() {
		return nil
	}
	// Get process metadata
	processTx, err := processMeta(&e.ContractABI, event.Data, e.ProcessHandle)
	if err != nil {
		return err
	}
	if processTx == nil {
		return fmt.Errorf("cannot fetch process metadata")
	}
	// Import remote census
	if !strings.HasPrefix(processTx.MkURI, e.Census.Data().URIprefix()) || len(processTx.MkRoot) == 0 {
		return fmt.Errorf("process not valid => %+v", processTx)
	}
	e.Census.AddToImportQueue(processTx.MkRoot, processTx.MkURI)
	return nil
}

func processMeta(contractABI *abi.ABI, eventData []byte, ph *chain.ProcessHandle) (*types.NewProcessTx, error) {
	var eventProcessCreated eventProcessCreated
	err := contractABI.Unpack(&eventProcessCreated, "ProcessCreated", eventData)
	if err != nil {
		return nil, err
	}
	return ph.ProcessTxArgs(eventProcessCreated.ProcessId)
}

func cancelProcessMeta(contractABI *abi.ABI, eventData []byte, ph *chain.ProcessHandle) (*types.CancelProcessTx, error) {
	var eventProcessCanceled eventProcessCanceled
	err := contractABI.Unpack(&eventProcessCanceled, "ProcessCanceled", eventData)
	if err != nil {
		return nil, err
	}
	return ph.CancelProcessTxArgs(eventProcessCanceled.ProcessId)
}
