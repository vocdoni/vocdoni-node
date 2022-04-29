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
	"go.vocdoni.io/dvote/ethereum/contracts"
	ethereumhandler "go.vocdoni.io/dvote/ethereum/handler"
	"go.vocdoni.io/dvote/log"
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
	// CensusUpdated(bytes32 processId, uint32 namespace)
	"processesCensusUpdated": "0xb290b721dc95d65b8ca629743f4f2e385523708862c8237aa6601dd9a99c238e",
	// NewProcess(bytes32 processId, uint32 namespace)
	"processesNewProcess": "0x3b1cc0fc696cbe654bd83494847cc7890f2ae0e05a79dfbd6c1892061fbf3404",
	// ProcessPriceUpdated(uint256 processPrice)
	"processPriceUpdated": "0x340b7835e5cad9e69cc8bf06b0b3c3e729f0fe4fd314932f4e4284d6ffc03a71",
	// QuestionIndexUpdated(bytes32 processId, uint32 namespace, uint8 newIndex)
	"processesQuestionIndexUpdated": "0xc3c879bd28e24bfa8df84d17ef3cae71077c3610e6167d435cc7e669e4a6b97c",
	// StatusUpdated(bytes32 processId, uint32 namespace, uint8 status)
	"processesStatusUpdated": "0x55ab39d22f8c4c97fce480c015b739838aa5b8a4ad0a528159669842a7087b01",
	// Withdraw(address to, uint256 amount)
	"processesWithdraw": "0x884edad9ce6fa2440d8a54cc123490eb96d2768479d49ff9c7366125a9424364",

	// NAMESPACES

	// NamespaceRegistered(uint32 namespace)
	"namespaceRegistered": "0x6342a3b1a0f483c8ec694afd510f5f330e4792137228eb79e3e14458f78c5746",

	// TOKEN STORAGE PROOF

	// TokenRegistered(address indexed token, address indexed registrar)
	"tokenStorageProofTokenRegistered": "0x158412daecdc1456d01568828bcdb18464cc7f1ce0215ddbc3f3cfede9d1e63d",

	// GENESIS

	// ChainRegistered(uint32 chainId)
	"genesisChainRegistered": "0xced3baa88aa65d52234f5717c8b053dc44bb9df530b1f6784809640ed322b7e9",
	// GenesisUpdated(uint32 chainId)
	"genesisUpdated": "0x87f64dce9746fc7da2e672b4aacc82ad148ed5900411894ddcbe532618fa89fb",
	// OracleAdded(uint32 chainId, address oracleAddress)
	"genesisOracleAdded": "0x572d29453222865ac78ef8d936cb20aba9368900deb6db64997c1482fe2b30c9",
	// OracleRemoved(uint32 chainId, address oracleAddress)
	"genesisOracleRemoved": "0x4b8171862540a056f44154d626c1390fed806ee806026247dc8c19f47b09acbe",
	// ValidatorAdded(uint32 chainId, bytes validatorPublicKey)
	"genesisValidatorAdded": "0x3d8adf342e55e97b1f85be8e952d2b473ec50bb2004821559c2b440f0a589e4e",
	// ValidatorRemoved(uint32 chainId, bytes validatorPublicKey)
	"genesisValidatorRemoved": "0x7436794ad809d5819bbcbd64a94846c7da193b8280de35e7ce59d3b7b4e6bbe1",

	// RESULTS

	// ResultsAvailable(bytes32 processId)
	"resultsAvailable": "0x5aff397e0d9bfad4e73dfd9c2da1d146ce7fe8cfd1a795dbf6b95b417236fa4c",
}

// Number of blocks to add to the process start block to start as soon as possible the process.
// This start block addition takes into account that can be some delay on the Vochain commit operation.
const processStartBlockDelay = 10

// HandleVochainOracle handles the events on ethereum for the Oracle.
func HandleVochainOracle(ctx context.Context, event *ethtypes.Log, e *EthereumEvents) error {
	log.Debugf("executing oracle event %+v", event)
	tctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if len(e.EthereumWhiteListAddrs) != 0 {
		log.Debugf("executing EtehreumCreator whitelist: %v", e.EthereumWhiteListAddrs)
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
		log.Infof("executing NewProcess event")
		// Get process metadata
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		processTx, err := newProcessMeta(tctx, &e.ContractsInfo[ethereumhandler.ContractNameProcesses].ABI,
			event.Data, e.VotingHandle)
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
			log.Infof("process already exists, skipping")
			return nil
		}

		// Due to uncertainty of when an EVM based blockchain Tx
		// will be mined/sealed, let the user choose to put
		// startBlock = 1 with the autostart mode in order
		// to start the process as soon as the tx is mined
		// on the EVM based blockchain plus some delay
		// required by the Vochain.
		// startBlock = 1 is used because the processes
		// smart contract checks that startBlock > 0 which is required
		// for other type of processes. Given that the Vochain will be
		// already initialized when a process is created, using 1 and not 0
		// will have the same effect for the purpose of this check and
		// also maintains compatibility with the smartcontract code.
		if processTx.Process.Status == models.ProcessStatus_READY &&
			processTx.Process.StartBlock == 1 &&
			processTx.Process.Mode.AutoStart {
			processTx.Process.StartBlock = e.VochainApp.Height() + processStartBlockDelay
		}

		oracle, err := e.getAccount(e.Signer.Address())
		if err != nil {
			return fmt.Errorf("newProcess handle: %w", err)
		}
		processTx.Nonce = oracle.GetNonce()
		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_NewProcess{NewProcess: processTx}})
		if err != nil {
			return fmt.Errorf("cannot marshal newProcess tx: %w", err)
		}
		stx.Signature, err = e.Signer.SignVocdoniTx(stx.Tx, e.VochainApp.ChainID())
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		txb, err := proto.Marshal(stx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %s", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(processTx))

		res, err := e.VochainApp.SendTx(txb)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)

	case ethereumEventList["processesStatusUpdated"]:
		log.Infof("executing StatusUpdate event")
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		setProcessTx, err := processStatusUpdatedMeta(tctx,
			&e.ContractsInfo["processes"].ABI, event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain update status data for creating the transaction: %w", err)
		}
		log.Infof("found process %x status update on ethereum, new status is %s",
			setProcessTx.ProcessId, setProcessTx.Status)
		p, err := e.VochainApp.State.Process(setProcessTx.ProcessId, true)
		if err != nil {
			return fmt.Errorf("cannot fetch the process from the Vochain: %w", err)
		}
		if p.Status == models.ProcessStatus_CANCELED || p.Status == models.ProcessStatus_ENDED {
			log.Infof("process already canceled or ended, skipping")
			return nil
		}
		oracle, err := e.getAccount(e.Signer.Address())
		if err != nil {
			return fmt.Errorf("set process handle: %w", err)
		}
		setProcessTx.Nonce = oracle.GetNonce()
		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: setProcessTx}})
		if err != nil {
			return fmt.Errorf("cannot marshal setProcess tx: %w", err)
		}
		stx.Signature, err = e.Signer.SignVocdoniTx(stx.Tx, e.VochainApp.ChainID())
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		txb, err := proto.Marshal(stx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %w", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(setProcessTx))

		res, err := e.VochainApp.SendTx(txb)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)

	case ethereumEventList["processesCensusUpdated"]:
		log.Infof("executing CensusUpdate event")
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		setProcessTx, err := processCensusUpdatedMeta(tctx,
			&e.ContractsInfo["processes"].ABI, event.Data, e.VotingHandle)
		if err != nil {
			return fmt.Errorf("cannot obtain census update data for creating the transaction: %w", err)
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
		if !vochain.CensusOrigins[p.CensusOrigin].AllowCensusUpdate {
			return fmt.Errorf("process census origin %s does not accept census updates",
				p.CensusOrigin.String())
		}

		oracle, err := e.getAccount(e.Signer.Address())
		if err != nil {
			return fmt.Errorf("set census handle: %w", err)
		}
		setProcessTx.Nonce = oracle.GetNonce()
		stx := &models.SignedTx{}
		stx.Tx, err = proto.Marshal(&models.Tx{Payload: &models.Tx_SetProcess{SetProcess: setProcessTx}})
		if err != nil {
			return fmt.Errorf("cannot marshal setProcess tx: %w", err)
		}
		stx.Signature, err = e.Signer.SignVocdoniTx(stx.Tx, e.VochainApp.ChainID())
		if err != nil {
			return fmt.Errorf("cannot sign oracle tx: %w", err)
		}
		tx, err := proto.Marshal(stx)
		if err != nil {
			return fmt.Errorf("error marshaling process tx: %w", err)
		}
		log.Debugf("broadcasting tx: %s", log.FormatProto(setProcessTx))

		res, err := e.VochainApp.SendTx(tx)
		if err != nil || res == nil {
			return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
		}
		log.Infof("oracle transaction sent, hash: %x", res.Hash)
	default:
		log.Debugf("no event configured for %s", event.Topics[0].Hex())
	}
	return nil
}

func newProcessMeta(ctx context.Context, contractABI *abi.ABI, eventData []byte,
	ph *ethereumhandler.EthereumHandler) (*models.NewProcessTx, error) {
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
	ph *ethereumhandler.EthereumHandler,
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
	ph *ethereumhandler.EthereumHandler,
) (*models.SetProcessTx, error) {
	structuredData := &contracts.ProcessesCensusUpdated{}
	if err := contractABI.UnpackIntoInterface(structuredData, "CensusUpdated", eventData); err != nil {
		return nil, fmt.Errorf("cannot unpack CensusUpdated event: %w", err)
	}
	log.Debugf("processCensusUpdated eventData: %+v", structuredData)
	return ph.SetCensusTxArgs(ctx, structuredData.ProcessId, structuredData.Namespace)
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

func (e *EthereumEvents) getAccount(addr common.Address) (*vochain.Account, error) {
	acc, err := e.VochainApp.State.GetAccount(e.Signer.Address(), false)
	if err != nil {
		return nil, fmt.Errorf("cannot get account")
	}
	if acc == nil {
		return nil, fmt.Errorf("account does not exist")
	}
	return acc, nil
}
