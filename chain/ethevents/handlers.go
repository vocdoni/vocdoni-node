package ethevents

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	models "github.com/vocdoni/dvote-protobuf/build/go/models"
	"gitlab.com/vocdoni/go-dvote/chain"
	"gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/vochain"
	"google.golang.org/protobuf/proto"
)

var ethereumEventList = map[string]string{
	// PROCESSES

	// Activated(uint256 blockNumber)
	"processesActivated": "0x3ec796be1be7d03bff3a62b9fa594a60e947c1809bced06d929f145308ae57ce",
	// ActivatedSuccessor(uint256 blockNumber, address successor)
	"processesActivatedSuccessor": "0x1f8bdb9825a71b7560200e2279fd4b503ac6814e369318e761928502882ee11a",
	// CensusUpdated(bytes32 processId, uint16 namespace)
	"processesCensusUpdated": "0xe54b983ab80f8982da0bb83c59ca327de698b5d0780451eba9706b4ffe069211",
	// NamespaceAddressUpdated(address namespaceAddr)
	"processesNamespaceAddressUpdated": "0x215ba443e028811c105c1bb484176ce9d9eec24ea7fb85c67a6bff78a04302b3",
	// NewProcess(bytes32 processId, uint16 namespace)
	"processesNewProcess": "0x2399440b5a42cbc7ba215c9c176f7cd16b511a8727c1f277635f3fce4649156e",
	// QuestionIndexUpdated(bytes32 processId, uint16 namespace, uint8 newIndex)
	"processesQuestionIndexUpdated": "0x2e4d6a3a868975a1e47c2ddc05451ebdececff07e59871dbc6cbaf9364aa06c6",
	// ResultsAvailable(bytes32 processId)
	"processesResultsAvailable": "0x5aff397e0d9bfad4e73dfd9c2da1d146ce7fe8cfd1a795dbf6b95b417236fa4c",
	// StatusUpdated(bytes32 processId, uint16 namespace, uint8 status)
	"processesStatusUpdated": "0xe64955704069c81c54f3fcca4da180a400f40da1bac10b68a9b42c753aa7a7f8",

	// NAMESPACE

	// ChainIdUpdated(string chainId, uint16 namespace)
	"namespaceChainIdUpdated": "0xe3d9869f91cf391b3bf911c3a1467e4195d49417ea46a46edc8ffb59edb2faa1",
	// GenesisUpdated(string genesis, uint16 namespace)
	"namespaceGenegisUpdated": "0x09b915de2907fa8b732e1b8549d1d8748d1f6365789bacd8bfc1c2b13321f1e9",
	// NamespaceUpdated(uint16 namespace)
	"namespaceUpdated": "0x06500a9a8bac2497581b3067d4076b05a0485705bdc05a53983cdbb9185fc8f1",
	// OracleAdded(address oracleAddress, uint16 namespace)
	"namespaceOracleAdded": "0x46046a89d1b1ddc11139d795a177db8e9b123e25c07e8d7b3b537aefc994b6ad",
	// OracleRemoved(address oracleAddress, uint16 namespace)
	"namespaceOracleRemoved": "0xeb7308698004c0bfb1007fb03df3d23b5ec8704e43aaeca3bfce122db656e09f",
	//  ValidatorAdded(string validatorPublicKey, uint16 namespace)
	"namespaceValidatorAdded": "0xaa457f0c02f923a1498e47a5c9d4b832e998fcf5b391974fc0c6a946794a8134",
	// ValidatorRemoved(string validatorPublicKey, uint16 namespace)
	"namespaceValidatorRemoved": "0x443f0e063aa676cbc61e749911d0c2652869c9ec48c4bb503eed9f19a44c250f",

	// TOKEN STORAGE PROOF

	// TokenRegistered(address indexed token, address indexed registrar)
	"tokenStorageProofTokenRegistered": "0x487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf0",
}

// HandleVochainOracle handles the events on ethereum for the Oracle.
func HandleVochainOracle(ctx context.Context, event *ethtypes.Log, e *EthereumEvents) error {
	switch event.Topics[0].Hex() {

	case ethereumEventList["processesNewProcess"]:
		// Get process metadata
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		processTx, err := newProcessMeta(tctx, &e.ContractsABI[0], event.Data, e.VotingHandle)
		if err != nil {
			return err
		}
		if processTx.Process == nil {
			return fmt.Errorf("process is nil")
		}
		// Check if process already exist
		log.Infof("found new process on Ethereum\n\t%+s", processTx.Process.String)
		_, err = e.VochainApp.State.Process(processTx.Process.ProcessId, true)
		if err != nil {
			if err != vochain.ErrProcessNotFound {
				return err
			}
		} else {
			log.Infof("process already exist, skipping")
			return nil
		}
		vtx := models.Tx{}
		processTxBytes, err := proto.Marshal(processTx)
		if err != nil {
			return fmt.Errorf("cannot marshal new process tx: %w", err)
		}
		signature, err := e.Signer.Sign(processTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Signature, err = hex.DecodeString(signature)
		if err != nil {
			return fmt.Errorf("cannot decode signature: %w", err)
		}
		vtx.Payload = &models.Tx_NewProcess{NewProcess: processTx}
		txb, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: (%s)", err)
		}
		log.Debugf("broadcasting Vochain Tx: %s", processTx.String())

		res, err := e.VochainApp.SendTX(txb)
		if err != nil || res == nil {
			log.Warnf("cannot broadcast tx: (%s)", err)
			return fmt.Errorf("cannot broadcast tx: (%s), res: (%+v)", err, res)
		}
		log.Infof("oracle transaction sent, hash:%s", res.Hash)

	case ethereumEventList["processesStatusUpdated"]:
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		setProcessTx, err := processStatusUpdatedMeta(tctx, &e.ContractsABI[0], event.Data, e.VotingHandle)
		if err != nil {
			return err
		}
		log.Infof("found process status update on ethereum\n\t%+v", setProcessTx)
		p, err := e.VochainApp.State.Process(setProcessTx.ProcessId, true)
		if err != nil {
			return err
		}
		if p.Status == models.ProcessStatus_CANCELED || p.Status == models.ProcessStatus_ENDED {
			log.Infof("process already canceled or ended, skipping")
			return nil
		}
		vtx := models.Tx{}
		setStatusTxBytes, err := proto.Marshal(setProcessTx)
		if err != nil {
			return fmt.Errorf("cannot marshal setProcess tx: %w", err)
		}
		signature, err := e.Signer.Sign(setStatusTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Signature, err = hex.DecodeString(signature)
		if err != nil {
			return fmt.Errorf("cannot decode signature: %w", err)
		}
		vtx.Payload = &models.Tx_SetProcess{SetProcess: setProcessTx}
		tx, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %w", err)
		}
		log.Debugf("broadcasting Vochain tx\n\t%s", setProcessTx.String())

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			log.Warnf("cannot broadcast tx: (%s)", err)
			return fmt.Errorf("cannot broadcast tx: (%s), res: (%+v)", err, res)
		}
		log.Infof("oracle transaction sent, hash:%s", res.Hash)
	}
	return nil
}

func newProcessMeta(ctx context.Context, contractABI *abi.ABI, eventData []byte, ph *chain.VotingHandle) (*models.NewProcessTx, error) {
	pc, err := contractABI.Unpack("NewProcess", eventData)
	if len(pc) == 0 {
		return nil, fmt.Errorf("cannot parse processMeta log")
	}
	eventProcessCreated := pc[0].(contracts.VotingProcessNewProcess)
	if err != nil {
		return nil, err
	}
	return ph.NewProcessTxArgs(ctx, eventProcessCreated.ProcessId, eventProcessCreated.Namespace)
}

func processStatusUpdatedMeta(ctx context.Context, contractABI *abi.ABI, eventData []byte, ph *chain.VotingHandle) (*models.SetProcessTx, error) {
	pc, err := contractABI.Unpack("StatusUpdated", eventData)
	if len(pc) == 0 {
		return nil, fmt.Errorf("cannot parse processMeta log")
	}
	eventProcessStatusUpdated := pc[0].(contracts.VotingProcessStatusUpdated)
	if err != nil {
		return nil, err
	}
	return ph.SetStatusTxArgs(ctx, eventProcessStatusUpdated.ProcessId, eventProcessStatusUpdated.Namespace, eventProcessStatusUpdated.Status)
}
