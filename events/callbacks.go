package events

import (
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// SCRUTINIZER

// checkResults checks the results received from the event source
func checkResults(results *models.ProcessResult) error {
	if len(results.EntityId) != types.EntityIDsize {
		return fmt.Errorf("invalid entityId size")
	}
	if len(results.ProcessId) != types.ProcessIDsize {
		return fmt.Errorf("invalid processId size")
	}
	if len(results.Votes) <= 0 {
		return fmt.Errorf("invalid votes length, results.Votes cannot be empty")
	}
	return nil
}

// onComputeResults is called once a process result is computed by the scrutinizer.
func onComputeResults(
	app *vochain.BaseApplication,
	signer *ethereum.SignKeys,
	data proto.Message,
) error {
	results := data.(*models.ProcessResult)
	// create setProcessTx
	setprocessTxArgs := &models.SetProcessTx{
		ProcessId: results.ProcessId,
		Results:   results,
		Status:    models.ProcessStatus_RESULTS.Enum(),
		Txtype:    models.TxType_SET_PROCESS_RESULTS,
	}
	vtx := models.Tx{}
	resultsTxBytes, err := proto.Marshal(setprocessTxArgs)
	if err != nil {
		return fmt.Errorf("cannot marshal set process results tx: %w", err)
	}
	vtx.Signature, err = signer.Sign(resultsTxBytes)
	if err != nil {
		return fmt.Errorf("cannot sign oracle tx: %w", err)
	}

	vtx.Payload = &models.Tx_SetProcess{SetProcess: setprocessTxArgs}
	txb, err := proto.Marshal(&vtx)
	if err != nil {
		return fmt.Errorf("error marshaling set process results tx: %w", err)
	}
	log.Debugf("broadcasting Vochain Tx: %s", setprocessTxArgs.String())

	res, err := app.SendTX(txb)
	if err != nil || res == nil {
		return fmt.Errorf("cannot broadcast tx: %w, res: %+v", err, res)
	}
	log.Infof("transaction sent, hash:%s", res.Hash)
	return nil
}

// KEYKEEPER

// ETHEREUM

// VOCHAIN

// CENSUS
