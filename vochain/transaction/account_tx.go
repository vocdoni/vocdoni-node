package transaction

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/types"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
)

// CreateAccountTxCheck checks if an account creation tx is valid
func (t *TransactionHandler) CreateAccountTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.SignedBody == nil || vtx.Signature == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Txtype != models.TxType_CREATE_ACCOUNT {
		return fmt.Errorf("invalid tx type, expected %s, got %s", models.TxType_CREATE_ACCOUNT, tx.Txtype)
	}
	// check account does not exist
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txSenderAcc, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get account: %w", err)
	}
	if txSenderAcc != nil {
		return vstate.ErrAccountAlreadyExists
	}

	infoURI := tx.GetInfoURI()
	if len(infoURI) > types.MaxURLLength {
		return ErrInvalidURILength
	}
	if len(tx.GetSIK()) > 0 {
		return fmt.Errorf("secret identity keys are not supported")
	}
	if err := vstate.CheckDuplicateDelegates(tx.GetDelegates(), &txSenderAddress); err != nil {
		return fmt.Errorf("invalid delegates: %w", err)
	}
	txCost, err := t.state.TxBaseCost(models.TxType_CREATE_ACCOUNT, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	// Check the faucet package
	canPayForTx, err := t.checkFaucetPackageAndTransfer(tx.FaucetPackage, txCost, txSenderAddress, vtx.TxID[:], false)
	if err != nil {
		return err
	}
	if !canPayForTx {
		return fmt.Errorf("faucet package is not enough for paying for the transaction cost")
	}
	return nil
}

// SetAccountDelegateTxCheck checks if a SetAccountDelegateTx and its data are valid
func (t *TransactionHandler) SetAccountDelegateTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Txtype != models.TxType_ADD_DELEGATE_FOR_ACCOUNT &&
		tx.Txtype != models.TxType_DEL_DELEGATE_FOR_ACCOUNT {
		return fmt.Errorf("invalid tx type")
	}
	if len(tx.Delegates) == 0 {
		return fmt.Errorf("invalid delegates")
	}

	txSenderAccount, txSenderAddr, err := t.checkAccountCanPayCost(tx.Txtype, vtx, 0)
	if err != nil {
		return err
	}
	if err := vstate.CheckDuplicateDelegates(tx.Delegates, txSenderAddr); err != nil {
		return fmt.Errorf("checkDuplicateDelegates: %w", err)
	}

	switch tx.Txtype {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range tx.Delegates {
			delegateAddress := common.BytesToAddress(delegate)
			if txSenderAccount.IsDelegate(delegateAddress) {
				return fmt.Errorf("delegate %s already exists", delegateAddress)
			}
		}
		return nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range tx.Delegates {
			delegateAddress := common.BytesToAddress(delegate)
			if !txSenderAccount.IsDelegate(delegateAddress) {
				return fmt.Errorf("delegate %s does not exist", delegateAddress)
			}
		}
		return nil
	default:
		// should never happen
		return fmt.Errorf("invalid tx type")
	}
}

// SetAccountInfoTxCheck checks if a set account info tx is valid
func (t *TransactionHandler) SetAccountInfoTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid transaction")
	}
	txSenderAccount, txSenderAddress, err := t.checkAccountCanPayCost(models.TxType_SET_ACCOUNT_INFO_URI, vtx, 0)
	if err != nil {
		return err
	}
	txAccountAddress := common.BytesToAddress(tx.GetAccount())
	if txAccountAddress == (common.Address{}) {
		txAccountAddress = *txSenderAddress
	}
	// check info URI
	infoURI := tx.GetInfoURI()
	if len(infoURI) == 0 || len(infoURI) > types.MaxURLLength {
		return fmt.Errorf("invalid URI, cannot be empty")
	}
	if bytes.Equal(txSenderAddress.Bytes(), txAccountAddress.Bytes()) {
		if infoURI == txSenderAccount.InfoURI {
			return fmt.Errorf("invalid URI, must be different")
		}
		return nil
	}
	// if txSender != txAccount only delegate operations
	// get tx account Account
	txAccountAccount, err := t.state.GetAccount(txAccountAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get tx account: %w", err)
	}
	if txAccountAccount == nil {
		return vstate.ErrAccountNotExist
	}
	if infoURI == txAccountAccount.InfoURI {
		return fmt.Errorf("invalid URI, must be different")
	}
	// check if delegate
	if !txAccountAccount.IsDelegate(*txSenderAddress) {
		return fmt.Errorf("tx sender is not a delegate")
	}
	return nil
}


