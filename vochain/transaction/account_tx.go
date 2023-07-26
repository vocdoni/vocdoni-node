package transaction

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/tree/arbo"
	"go.vocdoni.io/dvote/types"
	vstate "go.vocdoni.io/dvote/vochain/state"
	"go.vocdoni.io/dvote/vochain/transaction/vochaintx"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// CreateAccountTxCheck checks if an account creation tx is valid
func (t *TransactionHandler) CreateAccountTxCheck(vtx *vochaintx.Tx) (vstate.SIK, error) {
	if vtx == nil || vtx.SignedBody == nil || vtx.Signature == nil || vtx.Tx == nil {
		return nil, ErrNilTx
	}
	tx := vtx.Tx.GetSetAccount()
	if tx == nil {
		return nil, fmt.Errorf("invalid tx")
	}
	if tx.Txtype != models.TxType_CREATE_ACCOUNT {
		return nil, fmt.Errorf("invalid tx type, expected %s, got %s", models.TxType_CREATE_ACCOUNT, tx.Txtype)
	}
	sik := tx.GetSik()
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txSenderAcc, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	if txSenderAcc != nil {
		return nil, vstate.ErrAccountAlreadyExists
	}
	infoURI := tx.GetInfoURI()
	if len(infoURI) > types.MaxURLLength {
		return nil, ErrInvalidURILength
	}
	if err := vstate.CheckDuplicateDelegates(tx.GetDelegates(), &txSenderAddress); err != nil {
		return nil, fmt.Errorf("invalid delegates: %w", err)
	}
	txCost, err := t.state.TxBaseCost(models.TxType_CREATE_ACCOUNT, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get tx cost: %w", err)
	}
	if txCost == 0 {
		return nil, nil
	}
	if tx.FaucetPackage == nil {
		return nil, fmt.Errorf("invalid faucet package provided")
	}
	if tx.FaucetPackage.Payload == nil {
		return nil, fmt.Errorf("invalid faucet package payload")
	}
	faucetPayload := &models.FaucetPayload{}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
		return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
	}
	if faucetPayload.Amount == 0 {
		return nil, fmt.Errorf("invalid faucet payload amount provided")
	}
	if faucetPayload.To == nil {
		return nil, fmt.Errorf("invalid to address provided")
	}
	if !bytes.Equal(faucetPayload.To, txSenderAddress.Bytes()) {
		return nil, fmt.Errorf("payload to and tx sender missmatch (%x != %x)",
			faucetPayload.To, txSenderAddress.Bytes())
	}
	issuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract issuer address from faucet package vtx.Signature: %w", err)
	}
	issuerAcc, err := t.state.GetAccount(issuerAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get faucet issuer address account: %w", err)
	}
	if issuerAcc == nil {
		return nil, fmt.Errorf("the account signing the faucet payload does not exist (%s)", issuerAddress.String())
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	keyHash := ethereum.HashRaw(append(issuerAddress.Bytes(), b...))
	used, err := t.state.FaucetNonce(keyHash, false)
	if err != nil {
		return nil, fmt.Errorf("cannot check if faucet payload already used: %w", err)
	}
	if used {
		return nil, fmt.Errorf("faucet payload %x already used", keyHash)
	}
	if issuerAcc.Balance < faucetPayload.Amount+txCost {
		return nil, fmt.Errorf(
			"issuer address does not have enough balance %d, required %d",
			issuerAcc.Balance,
			faucetPayload.Amount+txCost,
		)
	}
	return sik, nil
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
	if tx.Nonce == nil {
		return fmt.Errorf("invalid nonce")
	}
	if len(tx.Delegates) == 0 {
		return fmt.Errorf("invalid delegates")
	}
	txSenderAddress, txSenderAccount, err := t.state.AccountFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return err
	}
	if err := vstate.CheckDuplicateDelegates(tx.Delegates, txSenderAddress); err != nil {
		return fmt.Errorf("checkDuplicateDelegates: %w", err)
	}
	if tx.GetNonce() != txSenderAccount.Nonce {
		return fmt.Errorf("invalid nonce, expected %d got %d", txSenderAccount.Nonce, tx.Nonce)
	}
	cost, err := t.state.TxBaseCost(tx.Txtype, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	if txSenderAccount.Balance < cost {
		return vstate.ErrNotEnoughBalance
	}
	switch tx.Txtype {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range tx.Delegates {
			delegateAddress := common.BytesToAddress(delegate)
			if txSenderAccount.IsDelegate(delegateAddress) {
				return fmt.Errorf("delegate %s already exists", delegateAddress.String())
			}
		}
		return nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range tx.Delegates {
			delegateAddress := common.BytesToAddress(delegate)
			if !txSenderAccount.IsDelegate(delegateAddress) {
				return fmt.Errorf("delegate %s does not exist", delegateAddress.String())
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
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txAccountAddress := common.BytesToAddress(tx.GetAccount())
	if txAccountAddress == (common.Address{}) {
		txAccountAddress = txSenderAddress
	}
	txSenderAccount, err := t.state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot check if account %s exists: %w", txSenderAddress, err)
	}
	if txSenderAccount == nil {
		return vstate.ErrAccountNotExist
	}
	// check txSender nonce
	if tx.GetNonce() != txSenderAccount.Nonce {
		return fmt.Errorf(
			"invalid nonce, expected %d got %d",
			txSenderAccount.Nonce,
			tx.GetNonce(),
		)
	}
	// get setAccount tx cost
	costSetAccountInfoURI, err := t.state.TxBaseCost(models.TxType_SET_ACCOUNT_INFO_URI, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	// check tx sender balance
	if txSenderAccount.Balance < costSetAccountInfoURI {
		return fmt.Errorf("unauthorized: %s", vstate.ErrNotEnoughBalance)
	}
	// check info URI
	infoURI := tx.GetInfoURI()
	if len(infoURI) == 0 || len(infoURI) > types.MaxURLLength {
		return fmt.Errorf("invalid URI, cannot be empty")
	}
	if txSenderAddress == txAccountAddress {
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
	if !txAccountAccount.IsDelegate(txSenderAddress) {
		return fmt.Errorf("tx sender is not a delegate")
	}
	return nil
}

// DelSIKTxCheck checks if a delete SIK tx is valid
func (t *TransactionHandler) DelSIKTxCheck(vtx *vochaintx.Tx) (common.Address, error) {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return common.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetDelSik()
	if tx == nil {
		return common.Address{}, fmt.Errorf("invalid transaction")
	}

	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract address from public key: %w", err)
	}

	if _, err := t.state.GetAccount(txAddress, false); err != nil {
		return common.Address{}, fmt.Errorf("cannot get tx account: %w", err)
	}
	return txAddress, nil
}

// SetSIKTxCheck checks if a set sik tx is valid
func (t *TransactionHandler) SetSIKTxCheck(vtx *vochaintx.Tx) (common.Address, vstate.SIK, error) {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return common.Address{}, nil, ErrNilTx
	}
	tx := vtx.Tx.GetSetSik()
	if tx == nil {
		return common.Address{}, nil, fmt.Errorf("invalid transaction")
	}
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}

	if _, err := t.state.GetAccount(txAddress, false); err != nil {
		return common.Address{}, nil, fmt.Errorf("cannot get tx account: %w", err)
	}

	newSIK := vtx.Tx.GetSetSik().GetSik()
	if newSIK == nil {
		return common.Address{}, nil, fmt.Errorf("no sik value provided")
	}
	// check if the address already has invalidated sik to ensure that it is
	// not updated after reach the correct height to avoid double voting
	if currentSIK, err := t.state.SIKFromAddress(txAddress); err == nil {
		maxEndBlock, err := t.state.ProcessBlockRegistry.MaxEndBlock(t.state.CurrentHeight(), false)
		if err != nil {
			if errors.Is(err, arbo.ErrKeyNotFound) {
				return txAddress, newSIK, nil
			}
			return common.Address{}, nil, err
		}
		if height := currentSIK.DecodeInvalidatedHeight(); height >= maxEndBlock {
			return txAddress, nil, fmt.Errorf("the sik could not be changed yet")
		}
	}
	return txAddress, newSIK, nil
}

// RegisterSIKTxCheck checks if the provided RegisterSIKTx is valid ensuring
// that the proof included on it is valid for the address of the transaction
// signer.
func (t *TransactionHandler) RegisterSIKTxCheck(vtx *vochaintx.Tx) (common.Address, vstate.SIK, error) {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return common.Address{}, nil, ErrNilTx
	}
	tx := vtx.Tx.GetRegisterSik()
	if tx == nil {
		return common.Address{}, nil, fmt.Errorf("invalid transaction")
	}
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	txAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	if _, err := t.state.GetAccount(txAddress, false); err != nil {
		return common.Address{}, nil, fmt.Errorf("cannot get the account for the tx address: %w", err)
	}
	newSIK := tx.GetSik()
	if newSIK == nil {
		return common.Address{}, nil, fmt.Errorf("no sik value provided")
	}
	// check if the address already has invalidated sik to ensure that it is
	// not updated after reach the correct height to avoid double voting
	if currentSIK, err := t.state.SIKFromAddress(txAddress); err == nil {
		maxEndBlock, err := t.state.ProcessBlockRegistry.MaxEndBlock(t.state.CurrentHeight(), false)
		if err != nil {
			return common.Address{}, nil, err
		}
		if height := currentSIK.DecodeInvalidatedHeight(); height >= maxEndBlock {
			return common.Address{}, nil, fmt.Errorf("the sik could not be changed yet")
		}
	}
	// check if the address already has a valid sik
	if _, err := t.state.SIKFromAddress(txAddress); err == nil {
		return txAddress, nil, fmt.Errorf("this address already has a valid sik")
	}
	// get census proof and election information provided
	censusProof := tx.GetCensusProof()
	if censusProof == nil {
		return common.Address{}, nil, fmt.Errorf("no proof provided")
	}
	electionId := tx.GetElectionId()
	if electionId == nil {
		return common.Address{}, nil, fmt.Errorf("no proof provided")
	}
	election, err := t.state.Process(electionId, false)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("error getting the election info: %w", err)
	}
	// verify the proof for the transaction signer address
	valid, _, err := VerifyProof(election, censusProof, vstate.NewVoterID(vstate.VoterIDTypeZkSnark, txAddress.Bytes()))
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("error verifying the proof: %w", err)
	}
	if !valid {
		return common.Address{}, nil, fmt.Errorf("proof not valid")
	}
	return txAddress, newSIK, nil
}
