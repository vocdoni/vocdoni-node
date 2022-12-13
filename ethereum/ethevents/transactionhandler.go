package ethevents

import (
	"time"

	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// pendingTransaction is a transaction that is waiting to be sent to the blockchain
type pendingTransaction struct {
	tx interface{}
}

// transactionHandler is a goroutine that handles the transaction queue and asures
// that the transactions are sent in the correct order with the corresponding nonce.
func (ev *EthereumEvents) transactionHandler() {
	log.Infof("starting transaction queue handler for oracle")
	lastNonce := uint32(0)
	for {
		select {
		case ptx := <-ev.transactionPool:
			if ptx == nil {
				continue
			}

			// Check if the nonce is correct
			acc, err := ev.VochainApp.State.GetAccount(ev.Signer.Address(), false)
			if err != nil {
				log.Errorf("cannot get account: %v", err)
				continue
			}
			if acc == nil {
				log.Errorf("account not found")
				continue
			}

			// Build the transaction
			stx := &models.SignedTx{}
			switch ptx.tx.(type) {
			case *models.Tx_NewProcess:
				tx := ptx.tx.(*models.Tx_NewProcess)
				tx.NewProcess.Nonce = acc.GetNonce()
				lastNonce = tx.NewProcess.Nonce
				stx.Tx, err = proto.Marshal(&models.Tx{Payload: tx})
				if err != nil {
					log.Errorf("cannot marshal transaction: %v", err)
					continue
				}
			case *models.Tx_SetProcess:
				tx := ptx.tx.(*models.Tx_SetProcess)
				tx.SetProcess.Nonce = acc.GetNonce()
				lastNonce = tx.SetProcess.Nonce
				stx.Tx, err = proto.Marshal(&models.Tx{Payload: tx})
				if err != nil {
					log.Errorf("cannot marshal transaction: %v", err)
					continue
				}
			default:
				log.Errorf("unknown transaction type")
				continue
			}

			// Add the signature
			stx.Signature, err = ev.Signer.SignVocdoniTx(stx.Tx, ev.VochainApp.ChainID())
			if err != nil {
				log.Errorf("cannot sign transaction: %v", err)
				continue
			}

			// Build and send the transaction
			payload, err := proto.Marshal(stx)
			if err != nil {
				log.Errorf("cannot marshal transaction: %v", err)
				continue
			}
			res, err := ev.VochainApp.SendTx(payload)
			if err != nil {
				log.Errorf("cannot send transaction: %v (response: %v)", err, res)
			}
			log.Infof("oracle transaction %d sent, hash: %x", lastNonce, res.Hash)

			// Wait for the nonce to be incremented (means transaction was processed)
			startTime := time.Now()
			for {
				acc, err := ev.VochainApp.State.GetAccount(ev.Signer.Address(), true)
				if err != nil {
					log.Errorf("cannot get account: %v", err)
					break
				}
				if acc == nil {
					log.Errorf("account not found")
					break
				}
				if acc.GetNonce() == lastNonce+1 {
					break
				}
				if time.Since(startTime) > time.Minute {
					log.Errorf("nonce %d not incremented after 1 minute", lastNonce)
					break
				}
				time.Sleep(time.Second)
			}

		default:
			time.Sleep(time.Second * 5)
		}
	}
}
