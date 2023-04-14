package oracle

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/dvote/vochain/indexer"
	"go.vocdoni.io/dvote/vochain/indexer/indexertypes"
	"go.vocdoni.io/dvote/vochain/results"
	"go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type Oracle struct {
	VochainApp *vochain.BaseApplication
	signer     *ethereum.SignKeys

	resultQueue map[string]*models.ProcessResult
	rqLock      sync.RWMutex
}

type OracleResults struct {
	ChainID       string            `json:"chainId"`
	EntityID      types.HexBytes    `json:"entityId"`
	OracleAddress common.Address    `json:"oracleAddress"`
	ProcessID     types.HexBytes    `json:"processId"`
	Results       [][]*types.BigInt `json:"results"`
}

func NewOracle(app *vochain.BaseApplication, signer *ethereum.SignKeys) (*Oracle, error) {
	return &Oracle{VochainApp: app, signer: signer}, nil
}

func (o *Oracle) EnableResults(scr *indexer.Indexer) {
	go o.resultQueueHandler()

	log.Infof("oracle results enabled")
	scr.AddEventListener(o)
}

// enqueueProcessResult pushes procresults to o.resultQueue, to be handled by resultQueueHandler
func (o *Oracle) enqueueProcessResult(processID types.HexBytes, procresults *models.ProcessResult) {
	o.rqLock.Lock()
	o.resultQueue[processID.String()] = procresults
	o.rqLock.Unlock()
}

// resultQueueHandler runs forever, every 15 seconds inspects the whole resultQueue.
// if a process reached RESULTS status, the item is removed from the queue
// otherwise, it broadcasts a TxSetProcess to the network, with a fresh Nonce.
func (o *Oracle) resultQueueHandler() {
	o.resultQueue = make(map[string]*models.ProcessResult)

	for {
		o.rqLock.Lock()
		for key, procresults := range o.resultQueue {
			processID := types.HexStringToHexBytes(key)

			// check vochain process status
			vocProcessData, err := o.VochainApp.State.Process(processID, true)
			if err != nil {
				log.Errorf("error fetching process %x from the Vochain: %v", processID, err)
				continue
			}
			log.Debugw("resultQueue loop", "pid", processID, "status", vocProcessData.Status)

			if vocProcessData.Status == models.ProcessStatus_RESULTS {
				delete(o.resultQueue, key)
				continue
			}

			err = o.sendTxSetProcessResults(processID, procresults)
			if err != nil {
				log.Error(err)
			}
		}
		o.rqLock.Unlock()
		time.Sleep(time.Second * 15)
	}
}

// OnComputeResults is called once a process result is computed by the indexer.
// The Oracle will build and send a RESULTS transaction to the Vochain.
// The transaction includes the final results for the process.
func (o *Oracle) OnComputeResults(results *results.Results, proc *indexertypes.Process, h uint32) {
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

	procresults := indexer.BuildProcessResult(results, vocProcessData.EntityId)

	// add the signature to the results and own address
	signedResultsPayload := OracleResults{
		ChainID:   o.VochainApp.ChainID(),
		EntityID:  vocProcessData.EntityId,
		ProcessID: results.ProcessID,
		Results:   state.GetFriendlyResults(procresults.GetVotes()),
	}
	resultsPayload, err := json.Marshal(signedResultsPayload)
	if err != nil {
		log.Warnf("cannot marshal signed results: %v", err)
		return
	}
	procresults.Signature, err = o.signer.SignEthereum(resultsPayload)
	if err != nil {
		log.Warnf("cannot sign results: %v", err)
	}
	procresults.OracleAddress = o.signer.Address().Bytes()

	o.enqueueProcessResult(results.ProcessID, procresults)
}

// sendTxSetProcessResults crafts a SetProcessTx containing the passed procresults,
// with the most up-to-date oracle Nonce available,
// signs it and broadcasts it with SendTx
func (o *Oracle) sendTxSetProcessResults(processID types.HexBytes, procresults *models.ProcessResult) error {
	// get oracle account
	acc, err := o.VochainApp.State.GetAccount(o.signer.Address(), false)
	if err != nil {
		return fmt.Errorf("error fetching oracle account: %s", err)
	}
	if acc == nil {
		return fmt.Errorf("oracle account does not exist")
	}
	// create setProcessTx
	setprocessTxArgs := &models.SetProcessTx{
		ProcessId: processID,
		Results:   procresults,
		Status:    models.ProcessStatus_RESULTS.Enum(),
		Txtype:    models.TxType_SET_PROCESS_RESULTS,
		Nonce:     acc.Nonce,
	}

	// sign and send the transaction
	stx := &models.SignedTx{}
	if stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_SetProcess{
			SetProcess: setprocessTxArgs,
		},
	}); err != nil {
		return fmt.Errorf("cannot marshal setProcessResults tx: %v", err)
	}
	if stx.Signature, err = o.signer.SignVocdoniTx(stx.Tx, o.VochainApp.ChainID()); err != nil {
		return fmt.Errorf("cannot sign oracle tx: %v", err)
	}

	txb, err := proto.Marshal(stx)
	if err != nil {
		return fmt.Errorf("error marshaling setProcessResults tx: %v", err)
	}
	log.Debugf("broadcasting Vochain Tx: %s", log.FormatProto(setprocessTxArgs))

	res, err := o.VochainApp.SendTx(txb)
	if err != nil || res == nil {
		return fmt.Errorf("cannot broadcast tx: %v, res: %+v", err, res)
	}
	log.Infof("oracle transaction sent, hash: %x", res.Hash)
	return nil
}

// OnOracleResults does nothing. Required for implementing the indexer EventListener interface
func (o *Oracle) OnOracleResults(procResults *models.ProcessResult, pid []byte, height uint32) {
}
