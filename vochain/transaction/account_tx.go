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
	if err := vstate.CheckDuplicateDelegates(tx.GetDelegates(), &txSenderAddress); err != nil {
		return fmt.Errorf("invalid delegates: %w", err)
	}
	txCost, err := t.state.TxBaseCost(models.TxType_CREATE_ACCOUNT, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	if txCost == 0 {
		return nil
	}
	if tx.FaucetPackage == nil {
		return fmt.Errorf("invalid faucet package provided")
	}
	if tx.FaucetPackage.Payload == nil {
		return fmt.Errorf("invalid faucet package payload")
	}
	faucetPayload := &models.FaucetPayload{}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
		return fmt.Errorf("could not unmarshal faucet package: %w", err)
	}
	if faucetPayload.Amount == 0 {
		return fmt.Errorf("invalid faucet payload amount provided")
	}
	if faucetPayload.To == nil {
		return fmt.Errorf("invalid to address provided")
	}
	if !bytes.Equal(faucetPayload.To, txSenderAddress.Bytes()) {
		return fmt.Errorf("payload to and tx sender missmatch (%x != %x)",
			faucetPayload.To, txSenderAddress.Bytes())
	}
	issuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract issuer address from faucet package vtx.Signature: %w", err)
	}
	issuerAcc, err := t.state.GetAccount(issuerAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get faucet issuer address account: %w", err)
	}
	if issuerAcc == nil {
		return fmt.Errorf("the account signing the faucet payload does not exist (%s)", issuerAddress)
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	keyHash := ethereum.HashRaw(append(issuerAddress.Bytes(), b...))
	used, err := t.state.FaucetNonce(keyHash, false)
	if err != nil {
		return fmt.Errorf("cannot check if faucet payload already used: %w", err)
	}
	if used {
		return fmt.Errorf("faucet payload %x already used", keyHash)
	}
	if issuerAcc.Balance < faucetPayload.Amount+txCost {
		return fmt.Errorf(
			"issuer address does not have enough balance %d, required %d",
			issuerAcc.Balance,
			faucetPayload.Amount+txCost,
		)
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
	txSenderAccount, txSenderAddr, err := t.checkAccountCanPayCost(tx.Txtype, vtx)
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
	txSenderAccount, txSenderAddress, err := t.checkAccountCanPayCost(models.TxType_SET_ACCOUNT_INFO_URI, vtx)
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

// DelSIKTxCheck checks if a delete SIK tx is valid
func (t *TransactionHandler) DelSIKTxCheck(vtx *vochaintx.Tx) (common.Address, error) {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return common.Address{}, ErrNilTx
	}
	tx := vtx.Tx.GetDelSIK()
	if tx == nil {
		return common.Address{}, fmt.Errorf("invalid transaction")
	}
	// get the pubkey from the tx signature
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	// get the account address from the pubkey
	txAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// check if the address is already registered
	if _, err := t.state.GetAccount(txAddress, false); err != nil {
		return common.Address{}, fmt.Errorf("cannot get tx account: %w", err)
	}
	// check if the address already has a registered SIK
	currentSIK, err := t.state.SIKFromAddress(txAddress)
	if err != nil {
		return common.Address{}, fmt.Errorf("cannot get current address SIK: %w", err)
	}
	// check that the registered SIK is still valid
	if !currentSIK.Valid() {
		return common.Address{}, fmt.Errorf("the SIK has already been deleted")
	}
	return txAddress, nil
}

// SetSIKTxCheck checks if a set SIK tx is valid and it is if the current
// address has not a SIK registered or the SIK registered had been already
// deleted.
func (t *TransactionHandler) SetSIKTxCheck(vtx *vochaintx.Tx) (common.Address, vstate.SIK, error) {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return common.Address{}, nil, ErrNilTx
	}
	tx := vtx.Tx.GetSetSIK()
	if tx == nil {
		return common.Address{}, nil, fmt.Errorf("invalid transaction")
	}
	_, txAddress, err := t.checkAccountCanPayCost(models.TxType_SET_ACCOUNT_SIK, vtx)
	if err != nil {
		return common.Address{}, nil, err
	}
	newSIK := vtx.Tx.GetSetSIK().GetSIK()
	if newSIK == nil {
		return common.Address{}, nil, fmt.Errorf("no sik value provided")
	}
	// check if the address already has invalidated sik to ensure that it is
	// not updated after reach the correct height to avoid double voting
	if currentSIK, err := t.state.SIKFromAddress(*txAddress); err == nil {
		maxEndBlock, err := t.state.ProcessBlockRegistry.MaxEndBlock(t.state.CurrentHeight())
		if err != nil {
			if errors.Is(err, arbo.ErrKeyNotFound) {
				return *txAddress, newSIK, nil
			}
			return common.Address{}, nil, err
		}
		if height := currentSIK.DecodeInvalidatedHeight(); height >= maxEndBlock {
			return *txAddress, nil, fmt.Errorf("the sik could not be changed yet")
		}
	}
	return *txAddress, newSIK, nil
}

// RegisterSIKTxCheck checks if the provided RegisterSIKTx is valid ensuring
// that the proof included on it is valid for the address of the transaction
// signer.
func (t *TransactionHandler) RegisterSIKTxCheck(vtx *vochaintx.Tx) (common.Address, vstate.SIK, []byte, bool, error) {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return common.Address{}, nil, nil, false, ErrNilTx
	}
	// parse transaction
	tx := vtx.Tx.GetRegisterSIK()
	if tx == nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("invalid transaction")
	}
	// get the SIK provided
	newSIK := tx.GetSIK()
	if newSIK == nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("no sik value provided")
	}
	// get census proof and election information provided
	censusProof := tx.GetCensusProof()
	if censusProof == nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("no proof provided")
	}
	pid := tx.GetElectionId()
	if pid == nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("no election provided")
	}
	// get the pubkey from the tx signature
	pubKey, err := ethereum.PubKeyFromSignature(vtx.SignedBody, vtx.Signature)
	if err != nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("cannot extract public key from vtx.Signature: %w", err)
	}
	// get the account address from the pubkey
	txAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// check if the address is already registered
	if _, err := t.state.GetAccount(txAddress, false); err != nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("cannot get the account for the tx address: %w", err)
	}
	// check if the address already has a registered SIK
	if _, err := t.state.SIKFromAddress(txAddress); err == nil {
		return txAddress, nil, nil, false, fmt.Errorf("this address already has a SIK")
	}
	// get the process data by its ID
	process, err := t.state.Process(pid, false)
	if err != nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("error getting the process info: %w", err)
	}
	// ensure that the process is in READY state
	if process.GetStatus() != models.ProcessStatus_READY {
		return common.Address{}, nil, nil, false, fmt.Errorf("this process is not READY")
	}
	// check if the number of registered SIK via RegisterSIKTx associated to
	// this process reaches the MaxCensusSize
	numberOfRegisterSIK, err := t.state.CountRegisterSIK(pid)
	if err != nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("error getting the counter of RegisterSIKTx: %w", err)
	}
	if process.GetMaxCensusSize() <= uint64(numberOfRegisterSIK) {
		return common.Address{}, nil, nil, false, fmt.Errorf("process MaxCensusSize reached")
	}
	// verify the proof for the transaction signer address
	valid, _, err := VerifyProof(process, censusProof, vstate.NewVoterID(vstate.VoterIDTypeZkSnark, txAddress.Bytes()))
	if err != nil {
		return common.Address{}, nil, nil, false, fmt.Errorf("error verifying the proof: %w", err)
	}
	if !valid {
		return common.Address{}, nil, nil, false, fmt.Errorf("proof not valid: %x", process.CensusRoot)
	}
	return txAddress, newSIK, pid, process.GetTempSIKs(), nil
}

// SetAccountValidatorTxCheck upgrades an account to a validator.
func (t *TransactionHandler) SetAccountValidatorTxCheck(vtx *vochaintx.Tx) error {
	if vtx == nil || vtx.Signature == nil || vtx.SignedBody == nil || vtx.Tx == nil {
		return ErrNilTx
	}
	_, _, err := t.checkAccountCanPayCost(models.TxType_SET_ACCOUNT_VALIDATOR, vtx)
	if err != nil {
		return err
	}
	validatorPubKey := vtx.Tx.GetSetAccount().GetPublicKey()
	if validatorPubKey == nil {
		return fmt.Errorf("invalid nil public key")
	}
	validatorAddress, err := ethereum.AddrFromPublicKey(validatorPubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	validatorAccount, err := t.state.Validator(validatorAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get validator: %w", err)
	}
	if validatorAccount != nil {
		return fmt.Errorf("account is already a validator")
	}
	return nil
}
