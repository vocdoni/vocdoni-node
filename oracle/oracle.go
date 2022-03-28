package oracle

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/scrutinizer"
	"go.vocdoni.io/dvote/vochain/scrutinizer/indexertypes"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type Oracle struct {
	VochainApp *vochain.BaseApplication
	signer     *ethereum.SignKeys
}

type OracleResults struct {
	ChainID       string         `json:"chainId"`
	EntityID      types.HexBytes `json:"entityId"`
	OracleAddress common.Address `json:"oracleAddress"`
	ProcessID     types.HexBytes `json:"processId"`
	Results       [][]string     `json:"results"`
}

func NewOracle(app *vochain.BaseApplication, signer *ethereum.SignKeys) (*Oracle, error) {
	return &Oracle{VochainApp: app, signer: signer}, nil
}

func (o *Oracle) EnableResults(scr *scrutinizer.Scrutinizer) {
	log.Infof("oracle results enabled")
	scr.AddEventListener(o)
}

func (o *Oracle) NewProcess(process *models.Process) error {
	// Sanity checks
	if process == nil {
		return fmt.Errorf("process is nil")
	}
	if process.Status != models.ProcessStatus_READY && process.Status != models.ProcessStatus_PAUSED {
		return fmt.Errorf("invalid process status on process creation: %d", process.Status)
	}
	if len(process.ProcessId) != types.ProcessIDsize {
		return fmt.Errorf("processId size is wrong")
	}
	if len(process.EntityId) != types.EntityIDsize {
		return fmt.Errorf("entityId size is wrong")
	}
	if _, ok := vochain.CensusOrigins[process.CensusOrigin]; !ok {
		return fmt.Errorf("census origin: %d not supported", process.CensusOrigin)
	}
	if vochain.CensusOrigins[process.CensusOrigin].NeedsURI && process.CensusURI == nil {
		return fmt.Errorf("census %s needs URI but none has been provided",
			vochain.CensusOrigins[process.CensusOrigin].Name)
	}
	if process.BlockCount < types.ProcessesContractMinBlockCount {
		return fmt.Errorf("block count is too low")
	}
	if vochain.CensusOrigins[process.CensusOrigin].NeedsIndexSlot && process.EthIndexSlot == nil {
		return fmt.Errorf("censusOrigin needs index slot (not provided)")
	}

	// Check if process already exist
	if _, err := o.VochainApp.State.Process(process.ProcessId, true); err != nil {
		if err != vochain.ErrProcessNotFound {
			return err
		}
	} else {
		log.Infof("process %x already exists, skipping", process.ProcessId)
		return nil
	}

	// get oracle account
	acc, err := o.VochainApp.State.GetAccount(o.signer.Address(), false)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("oracle account does not exist")
	}
	// Create, sign a send NewProcess transaction
	processTx := &models.NewProcessTx{
		Process: process,
		Nonce:   acc.Nonce,
		Txtype:  models.TxType_NEW_PROCESS,
	}

	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: processTx,
		},
	})
	if err != nil {
		return fmt.Errorf("cannot marshal newProcess tx: %w", err)
	}
	stx.Signature, err = o.signer.SignVocdoniTx(stx.Tx, o.VochainApp.ChainID())
	if err != nil {
		return fmt.Errorf("cannot sign oracle tx: %w", err)
	}
	txb, err := proto.Marshal(stx)
	if err != nil {
		return fmt.Errorf("error marshaling process tx: %w", err)
	}
	log.Debugf("broadcasting tx: %s", log.FormatProto(processTx))

	res, err := o.VochainApp.SendTx(txb)
	if err != nil || res == nil {
		return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
	}
	log.Infof("newProcess transaction sent, hash: %x", res.Hash)
	return nil
}

// OnComputeResults is called once a process result is computed by the scrutinizer.
// The Oracle will build and send a RESULTS transaction to the Vochain.
// The transaction includes the final results for the process.
func (o *Oracle) OnComputeResults(results *indexertypes.Results, proc *indexertypes.Process, h uint32) {
	log.Infof("launching on computeResults callback for process %x", results.ProcessID)
	// check vochain process status
	vocProcessData, err := o.VochainApp.State.Process(results.ProcessID, true)
	if err != nil {
		log.Errorf("error fetching process %x from the Vochain: %v", results.ProcessID, err)
		return
	}

	// check process status
	switch vocProcessData.Status {
	case models.ProcessStatus_READY:
		if o.VochainApp.Height() < vocProcessData.StartBlock+vocProcessData.BlockCount {
			log.Warnf("process %x is in READY state and not yet finished, cannot publish results",
				results.ProcessID)
			return
		}
	case models.ProcessStatus_ENDED, models.ProcessStatus_RESULTS:
		break
	default:
		log.Infof("process %x: invalid status %s for setting the results, skipping",
			results.ProcessID, vocProcessData.Status)
		return
	}
	// get oracle account
	acc, err := o.VochainApp.State.GetAccount(o.signer.Address(), false)
	if err != nil {
		log.Errorf("error fetching oracle account: %s", err)
		return
	}
	if acc == nil {
		log.Errorf("oracle account does not exist")
		return
	}
	// create setProcessTx
	setprocessTxArgs := &models.SetProcessTx{
		ProcessId: results.ProcessID,
		Results:   scrutinizer.BuildProcessResult(results, vocProcessData.EntityId),
		Status:    models.ProcessStatus_RESULTS.Enum(),
		Txtype:    models.TxType_SET_PROCESS_RESULTS,
		Nonce:     acc.Nonce,
	}

	// add the signature to the results and own address
	setprocessTxArgs.Results.OracleAddress = o.signer.Address().Bytes()
	signedResultsPayload := OracleResults{
		ChainID:   o.VochainApp.ChainID(),
		EntityID:  vocProcessData.EntityId,
		ProcessID: results.ProcessID,
		Results:   vochain.GetFriendlyResults(setprocessTxArgs.Results.GetVotes()),
	}
	resultsPayload, err := json.Marshal(signedResultsPayload)
	if err != nil {
		log.Warnf("cannot marshal signed results: %v", err)
		return
	}
	setprocessTxArgs.Results.Signature, err = o.signer.SignEthereum(resultsPayload)
	if err != nil {
		log.Warnf("cannot sign results: %v", err)
	}

	// sign and send the transaction
	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{
			SetProcess: setprocessTxArgs,
		},
	}); err != nil {
		log.Errorf("cannot marshal setProcessResults tx: %v", err)
		return
	}
	if stx.Signature, err = o.signer.SignVocdoniTx(stx.Tx, o.VochainApp.ChainID()); err != nil {
		log.Errorf("cannot sign oracle tx: %v", err)
		return
	}

	txb, err := proto.Marshal(stx)
	if err != nil {
		log.Errorf("error marshaling setProcessResults tx: %v", err)
		return
	}
	log.Debugf("broadcasting Vochain Tx: %s", log.FormatProto(setprocessTxArgs))

	res, err := o.VochainApp.SendTx(txb)
	if err != nil || res == nil {
		log.Errorf("cannot broadcast tx: %v, res: %+v", err, res)
		return
	}
	log.Infof("oracle transaction sent, hash:%x", res.Hash)
}

// OnOracleResults does nothing. Required for implementing the scrutinizer EventListener interface
func (o *Oracle) OnOracleResults(procResults *models.ProcessResult, pid []byte, height uint32) {
}
