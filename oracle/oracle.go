package oracle

import (
	"bytes"
	"fmt"

	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/dvote/util"
	"go.vocdoni.io/dvote/vochain"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

type Oracle struct {
	VochainApp *vochain.BaseApplication
	signer     *ethereum.SignKeys
}

func NewOracle(app *vochain.BaseApplication, signer *ethereum.SignKeys) (*Oracle, error) {
	oracles, err := app.State.Oracles(true)
	if err != nil {
		return nil, err
	}
	found := false
	for _, oc := range oracles {
		if !bytes.Equal(signer.Address().Bytes(), oc.Bytes()) {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("this node is not an oracle")
	}
	return &Oracle{VochainApp: app, signer: signer}, nil
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
		return fmt.Errorf("census %s needs URI but non has been provided",
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
		log.Infof("process %x already exist, skipping", process.ProcessId)
		return nil
	}

	// Create, sign a send NewProcess transaction
	processTx := &models.NewProcessTx{
		Process: process,
		Nonce:   util.RandomBytes(32),
		Txtype:  models.TxType_NEW_PROCESS,
	}
	var err error
	stx := &models.SignedTx{}
	stx.Tx, err = proto.Marshal(&models.Tx{
		Payload: &models.Tx_NewProcess{
			NewProcess: processTx,
		},
	})
	if err != nil {
		return fmt.Errorf("cannot marshal new process tx: %w", err)
	}
	stx.Signature, err = o.signer.Sign(stx.Tx)
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
