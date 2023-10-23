package transaction

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// ExtractNonceAndSender extracts the nonce and sender address from a given Vochain transaction.
// The function uses the signature of the transaction to derive the sender's public key and subsequently
// the Ethereum address. The nonce is extracted based on the specific payload type of the transaction.
// If the transaction does not contain signature or nonce, it returns the default values (nil and 0).
func (t *TransactionHandler) ExtractNonceAndSender(vtx *vochaintx.Tx) (*common.Address, uint32, error) {
	var ptx interface {
		GetNonce() uint32
	}

	switch payload := vtx.Tx.Payload.(type) {
	case *models.Tx_NewProcess:
		ptx = payload.NewProcess
	case *models.Tx_SetProcess:
		ptx = payload.SetProcess
	case *models.Tx_SendTokens:
		ptx = payload.SendTokens
	case *models.Tx_SetAccount:
		if payload.SetAccount.Txtype == models.TxType_CREATE_ACCOUNT {
			// create account tx is a special case where the nonce is not relevant
			return nil, 0, nil
		}
		ptx = payload.SetAccount
	case *models.Tx_CollectFaucet:
		ptx = payload.CollectFaucet
	case *models.Tx_Vote, *models.Tx_Admin, *models.Tx_SetKeykeeper,
		*models.Tx_SetTransactionCosts, *models.Tx_DelSIK, *models.Tx_RegisterKey, *models.Tx_SetSIK,
		*models.Tx_RegisterSIK:
		// these tx does not have incremental nonce
		return nil, 0, nil
	default:
		log.Errorf("unknown payload type on extract nonce: %T", payload)
	}

	if ptx == nil {
		return nil, 0, fmt.Errorf("payload is nil")
	}

	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	addr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot extract address from public key: %w", err)
	}

	return &addr, ptx.GetNonce(), nil
}

// checkAccountNonce checks if the nonce of the given transaction matches the nonce of the sender account.
// If the transactions does not require a nonce, it returns nil.
// The check is performed against the current (not committed) state.
func (t *TransactionHandler) checkAccountNonce(vtx *vochaintx.Tx) error {
	addr, nonce, err := t.ExtractNonceAndSender(vtx)
	if err != nil {
		return err
	}
	if addr == nil && nonce == 0 {
		// no nonce required
		return nil
	}
	if addr == nil {
		return fmt.Errorf("could not check nonce, address is nil")
	}
	account, err := t.state.GetAccount(*addr, false)
	if err != nil {
		return fmt.Errorf("could not check nonce, error getting account: %w", err)
	}
	if account == nil {
		return fmt.Errorf("could not check nonce, account does not exist")
	}
	if account.Nonce != nonce {
		return fmt.Errorf("nonce mismatch, expected %d, got %d", account.Nonce, nonce)
	}
	return nil
}
