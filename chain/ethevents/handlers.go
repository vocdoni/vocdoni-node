package ethevents

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.vocdoni.io/dvote/chain"
	"go.vocdoni.io/dvote/chain/contracts"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	models "go.vocdoni.io/proto/build/go/models"
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
	tctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if len(e.EthereumWhiteListAddrs) != 0 {
		if err := checkEthereumTxCreator(tctx,
			event.TxHash,
			event.BlockHash,
			event.TxIndex,
			e.EthereumWhiteListAddrs,
			e.VotingHandle.EthereumClient,
		); err != nil {
			return fmt.Errorf("cannot process event, error checking Ethereum tx creator: %w", err)
		}
	}

	switch event.Topics[0].Hex() {

	case ethereumEventList["processesNewProcess"]:
		// Get process metadata
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		processTx, err := newProcessMeta(tctx, &e.ContractsABI[0], event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain process data for creating the transaction: %w", err)
		}
		if processTx.Process == nil {
			return fmt.Errorf("process obtained from ethereum storage and logs is nil")
		}
		// Check if process already exist
		log.Infof("found new process on Ethereum: %s", log.FormatProto(processTx.Process))
		_, err = e.VochainApp.State.Process(processTx.Process.ProcessId, true)
		if err != nil {
			if err != vochain.ErrProcessNotFound {
				return fmt.Errorf("process not found on the Vochain")
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
		vtx.Signature, err = e.Signer.Sign(processTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Payload = &models.Tx_NewProcess{NewProcess: processTx}
		txb, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %s", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(processTx))

		res, err := e.VochainApp.SendTX(txb)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)

	case ethereumEventList["processesStatusUpdated"]:
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		setProcessTx, err := processStatusUpdatedMeta(tctx, &e.ContractsABI[0], event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain update status data for creating the transaction: %w", err)
		}
		log.Infof("found process %x status update on ethereum, new status is %s", setProcessTx.ProcessId, setProcessTx.Status)
		p, err := e.VochainApp.State.Process(setProcessTx.ProcessId, true)
		if err != nil {
			return fmt.Errorf("cannot fetch the process from the Vochain: %w", err)
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
		vtx.Signature, err = e.Signer.Sign(setStatusTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Payload = &models.Tx_SetProcess{SetProcess: setProcessTx}
		tx, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %w", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(setProcessTx))

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)

	case ethereumEventList["processesCensusUpdated"]:
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		setProcessTx, err := processCensusUpdatedMeta(tctx, &e.ContractsABI[0], event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain census uptade data for creating the transaction: %w", err)

		}
		log.Infof("found process %x census update on ethereum", setProcessTx.ProcessId)
		p, err := e.VochainApp.State.Process(setProcessTx.ProcessId, true)
		if err != nil {
			return fmt.Errorf("cannot fetch the process from the Vochain: %w", err)
		}

		// process censusRoot
		if bytes.Equal(p.CensusRoot, setProcessTx.CensusRoot) {
			return fmt.Errorf("censusRoot cannot be the same")
		}
		// check dynamic census enabled
		if !p.Mode.DynamicCensus {
			return fmt.Errorf("process needs dynamic census in order to update its census")
		}
		// check status
		if (p.Status != models.ProcessStatus_READY) && (p.Status != models.ProcessStatus_PAUSED) {
			return fmt.Errorf("process status %s does not accept census updates", p.Status.String())
		}
		// check census origin
		if !types.CensusOrigins[p.CensusOrigin].AllowCensusUpdate {
			return fmt.Errorf("process census origin %s does not accept census updates", p.CensusOrigin.String())
		}

		vtx := models.Tx{}
		setCensusTxBytes, err := proto.Marshal(setProcessTx)
		if err != nil {
			return fmt.Errorf("cannot marshal setProcess tx: %w", err)
		}
		vtx.Signature, err = e.Signer.Sign(setCensusTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Payload = &models.Tx_SetProcess{SetProcess: setProcessTx}
		tx, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %w", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(setProcessTx))

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)

	case ethereumEventList["namespaceOracleAdded"]:
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		addOracleTx, err := namespaceOracleAddedMeta(tctx, &e.ContractsABI[1], event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain add oracle data for creating the transaction: %w", err)
		}
		log.Infof("found add oracle %x namespace event on ethereum", addOracleTx.Address)
		oracles, err := e.VochainApp.State.Oracles(true)
		if err != nil {
			return fmt.Errorf("cannot fetch the oracle list from the Vochain: %w", err)
		}
		// check oracle not already on the Vochain
		for idx, o := range oracles {
			if o == common.BytesToAddress(addOracleTx.Address) {
				return fmt.Errorf("cannot add oracle, already exists on the Vochain at position: %d", idx)
			}
		}
		// create admin tx
		vtx := models.Tx{}
		addOracleTxBytes, err := proto.Marshal(addOracleTx)
		if err != nil {
			return fmt.Errorf("cannot marshal admin tx (addOracle): %w", err)
		}
		vtx.Signature, err = e.Signer.Sign(addOracleTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Payload = &models.Tx_Admin{Admin: addOracleTx}
		tx, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling admin tx (addOracle) tx: %w", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(addOracleTx))

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)
	case ethereumEventList["namespaceOracleRemoved"]:
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		removeOracleTx, err := namespaceOracleRemovedMeta(tctx, &e.ContractsABI[1], event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain remove oracle data for creating the transaction: %w", err)
		}
		log.Infof("found remove oracle %x namespace event on ethereum", removeOracleTx.Address)
		oracles, err := e.VochainApp.State.Oracles(true)
		if err != nil {
			return fmt.Errorf("cannot fetch the oracle list from the Vochain: %w", err)
		}
		// check oracle is on the Vochain
		var found bool
		for idx, o := range oracles {
			if o == common.BytesToAddress(removeOracleTx.Address) {
				found = true
				log.Debugf("found oracle at position %d, creating remove tx", idx)
				break
			}
		}
		if !found {
			return fmt.Errorf("oracle not found, cannot remove")
		}
		// create admin tx
		vtx := models.Tx{}
		addOracleTxBytes, err := proto.Marshal(removeOracleTx)
		if err != nil {
			return fmt.Errorf("cannot marshal admin tx (removeOracle): %w", err)
		}
		vtx.Signature, err = e.Signer.Sign(addOracleTxBytes)
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		vtx.Payload = &models.Tx_Admin{Admin: removeOracleTx}
		tx, err := proto.Marshal(&vtx)
		if err != nil {
			return fmt.Errorf("error marshaling admin tx (removeOracle) tx: %w", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(removeOracleTx))

		res, err := e.VochainApp.SendTX(tx)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)
	}
	return nil
}

func newProcessMeta(
	ctx context.Context,
	contractABI *abi.ABI,
	eventData []byte,
	ph *chain.VotingHandle,
) (*models.NewProcessTx, error) {
	structuredData := &contracts.ProcessesNewProcess{}
	if err := contractABI.UnpackIntoInterface(structuredData, "NewProcess", eventData); err != nil {
		return nil, fmt.Errorf("cannot unpack NewProcess event: %w", err)
	}
	log.Debugf("newProcessMeta eventData: %+v", structuredData)
	return ph.NewProcessTxArgs(ctx, structuredData.ProcessId, structuredData.Namespace)
}

func processStatusUpdatedMeta(
	ctx context.Context,
	contractABI *abi.ABI,
	eventData []byte,
	ph *chain.VotingHandle,
) (*models.SetProcessTx, error) {
	structuredData := &contracts.ProcessesStatusUpdated{}
	if err := contractABI.UnpackIntoInterface(structuredData, "StatusUpdated", eventData); err != nil {
		return nil, fmt.Errorf("cannot unpack StatusUpdated event: %w", err)
	}
	log.Debugf("processStatusUpdated eventData: %+v", structuredData)
	return ph.SetStatusTxArgs(
		ctx,
		structuredData.ProcessId,
		structuredData.Namespace,
		structuredData.Status,
	)
}

func processCensusUpdatedMeta(
	ctx context.Context,
	contractABI *abi.ABI,
	eventData []byte,
	ph *chain.VotingHandle,
) (*models.SetProcessTx, error) {
	structuredData := &contracts.ProcessesCensusUpdated{}
	if err := contractABI.UnpackIntoInterface(structuredData, "CensusUpdated", eventData); err != nil {
		return nil, fmt.Errorf("cannot unpack CensusUpdated event: %w", err)
	}
	log.Debugf("processCensusUpdated eventData: %+v", structuredData)
	return ph.SetCensusTxArgs(ctx, structuredData.ProcessId, structuredData.Namespace)
}

func namespaceOracleAddedMeta(
	ctx context.Context,
	contractABI *abi.ABI,
	eventData []byte,
	ph *chain.VotingHandle,
) (*models.AdminTx, error) {
	structuredData := &contracts.NamespacesOracleAdded{}
	if err := contractABI.UnpackIntoInterface(structuredData, "OracleAdded", eventData); err != nil {
		return nil, fmt.Errorf("cannot unpack OracleAdded event: %w", err)
	}
	log.Debugf("namespacesOracleAdded eventData: %+v", structuredData)
	return ph.AddOracleTxArgs(ctx, structuredData.OracleAddress, structuredData.Namespace)
}

func namespaceOracleRemovedMeta(
	ctx context.Context,
	contractABI *abi.ABI,
	eventData []byte,
	ph *chain.VotingHandle,
) (*models.AdminTx, error) {
	structuredData := contracts.NamespacesOracleRemoved{}
	if err := contractABI.UnpackIntoInterface(structuredData, "OracleRemoved", eventData); err != nil {
		return nil, fmt.Errorf("cannot unpack OracleRemoved event: %w", err)
	}
	log.Debugf("namespaceOracleRemoved eventData: %+v", structuredData)
	return ph.RemoveOracleTxArgs(ctx, structuredData.OracleAddress, structuredData.Namespace)
}

func checkEthereumTxCreator(
	ctx context.Context,
	txHash common.Hash,
	blockHash common.Hash,
	txIndex uint,
	ethereumWhiteList map[common.Address]bool,
	ethclient *ethclient.Client,
) error {
	// get tx from ethereum
	tx, _, err := ethclient.TransactionByHash(ctx, txHash)
	if err != nil {
		return fmt.Errorf("cannot fetch tx by hash: %w", err)
	}
	// get from
	sender, err := ethclient.TransactionSender(ctx, tx, blockHash, txIndex)
	if err != nil {
		return fmt.Errorf("cannot fetch tx sender: %w", err)
	}
	log.Debugf("recovered sender for tx hash: %q is: %s", sender.String())
	// check from is whitelisted
	if !ethereumWhiteList[sender] {
		return fmt.Errorf("recovered address not in ethereum whitelist")
	}
	return nil
}
