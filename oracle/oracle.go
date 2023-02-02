package oracle

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/state"
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

func (o *Oracle) EnableResults(scr *indexer.Indexer) {
	log.Infof("oracle results enabled")
	scr.AddEventListener(o)
}

func (o *Oracle) NewProcess(process *models.Process) ([]byte, error) {
	// Sanity checks
	if process == nil {
		return nil, fmt.Errorf("process is nil")
	}
	if process.Status != models.ProcessStatus_READY && process.Status != models.ProcessStatus_PAUSED {
		return nil, fmt.Errorf("invalid process status on process creation: %d", process.Status)
	}
	if len(process.EntityId) != types.EntityIDsize {
		return nil, fmt.Errorf("entityId size is wrong")
	}
	if _, ok := state.CensusOrigins[process.CensusOrigin]; !ok {
		return nil, fmt.Errorf("census origin: %d not supported", process.CensusOrigin)
	}
	if state.CensusOrigins[process.CensusOrigin].NeedsURI && process.CensusURI == nil {
		return nil, fmt.Errorf("census %s needs URI but none has been provided",
			state.CensusOrigins[process.CensusOrigin].Name)
	}
	if process.BlockCount < types.ProcessesContractMinBlockCount {
		return nil, fmt.Errorf("block count is too low")
	}
	if state.CensusOrigins[process.CensusOrigin].NeedsIndexSlot && process.EthIndexSlot == nil {
		return nil, fmt.Errorf("censusOrigin needs index slot (not provided)")
	}

	// get oracle account
	acc, err := o.VochainApp.State.GetAccount(o.signer.Address(), false)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, fmt.Errorf("oracle account does not exist")
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
		return nil, fmt.Errorf("cannot marshal newProcess tx: %w", err)
	}
	stx.Signature, err = o.signer.SignVocdoniTx(stx.Tx, o.VochainApp.ChainID())
	if err != nil {
		return nil, fmt.Errorf("cannot sign oracle tx: %w", err)
	}
	txb, err := proto.Marshal(stx)
	if err != nil {
		return nil, fmt.Errorf("error marshaling process tx: %w", err)
	}
	log.Debugf("broadcasting tx: %s", log.FormatProto(processTx))

	res, err := o.VochainApp.SendTx(txb)
	if err != nil || res == nil {
		return nil, fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
	}
	log.Infof("newProcess transaction sent, processID: %x", res.Data.Bytes())
	return res.Data.Bytes(), nil
}

// OnComputeResults is called once a process result is computed by the indexer.
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
		Results:   indexer.BuildProcessResult(results, vocProcessData.EntityId),
		Status:    models.ProcessStatus_RESULTS.Enum(),
		Txtype:    models.TxType_SET_PROCESS_RESULTS,
		Nonce:     acc.Nonce,
	}

	// add the signature to the results and own address
	setprocessTxArgs.Results.OracleAddress = o.signer.Address().Bytes()
	resultsBigInt := state.GetFriendlyResults(setprocessTxArgs.Results.GetVotes())
	// convert results to string matrix
	resultsString := make([][]string, len(resultsBigInt))
	for i, v := range resultsBigInt {
		resultsString[i] = make([]string, len(v))
		for j, w := range v {
			resultsString[i][j] = w.String()
		}
	}
	signedResultsPayload := OracleResults{
		ChainID:   o.VochainApp.ChainID(),
		EntityID:  vocProcessData.EntityId,
		ProcessID: results.ProcessID,
		Results:   resultsString,
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
	log.Infof("oracle transaction sent, hash: %x", res.Hash)
}

// OnOracleResults does nothing. Required for implementing the indexer EventListener interface
func (o *Oracle) OnOracleResults(procResults *models.ProcessResult, pid []byte, height uint32) {
}
