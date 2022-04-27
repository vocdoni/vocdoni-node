package vochain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/tendermint/crypto"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/crypto/ethereum"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/types"
	"go.vocdoni.io/proto/build/go/models"
	"google.golang.org/protobuf/proto"
)

// Account represents an amount of tokens, usually attached to an address.
// Account includes a Nonce which needs to be incremented by 1 on each transfer,
// an external URI link for metadata and a list of delegated addresses allowed
// to use the account on its behalf (in addition to himself).
type Account struct {
	models.Account
}

// Marshal encodes the Account and returns the serialized bytes.
func (a *Account) Marshal() ([]byte, error) {
	return proto.Marshal(a)
}

// Unmarshal decode a set of bytes.
func (a *Account) Unmarshal(data []byte) error {
	return proto.Unmarshal(data, a)
}

// Transfer moves amount from the origin Account to the dest Account.
func (a *Account) Transfer(dest *Account, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot transfer zero amount")
	}
	if dest == nil {
		return fmt.Errorf("destination account nil")
	}
	if a.Balance < amount {
		return ErrNotEnoughBalance
	}
	if dest.Balance+amount <= dest.Balance {
		return ErrBalanceOverflow
	}
	dest.Balance += amount
	a.Balance -= amount
	return nil
}

// IsDelegate checks if an address is a delegate for an account
func (a *Account) IsDelegate(addr common.Address) bool {
	for _, d := range a.DelegateAddrs {
		if bytes.Equal(addr.Bytes(), d) {
			return true
		}
	}
	return false
}

// AddDelegate adds an address to the list of delegates for an account
func (a *Account) AddDelegate(addr common.Address) error {
	if a.IsDelegate(addr) {
		return fmt.Errorf("address %s is already a delegate", addr.Hex())
	}
	a.DelegateAddrs = append(a.DelegateAddrs, addr.Bytes())
	return nil
}

// DelDelegate removes an address from the list of delegates for an account
func (a *Account) DelDelegate(addr common.Address) {
	for i, d := range a.DelegateAddrs {
		if bytes.Equal(addr.Bytes(), d) {
			a.DelegateAddrs[i] = a.DelegateAddrs[len(a.DelegateAddrs)-1]
			a.DelegateAddrs = a.DelegateAddrs[:len(a.DelegateAddrs)-1]
		}
	}
}

// TransferBalance transfers balance from origin address to destination address,
// and updates the state with the new values (including nonce).
// If origin address acc is not enough, ErrNotEnoughBalance is returned.
func (v *State) TransferBalance(from, to common.Address, amount uint64) error {
	accFrom, err := v.GetAccount(from, false)
	if err != nil {
		return err
	}
	if accFrom == nil {
		return ErrAccountNotExist
	}
	accTo, err := v.GetAccount(to, false)
	if err != nil {
		return err
	}
	if accTo == nil {
		return ErrAccountNotExist
	}
	if err := accFrom.Transfer(accTo, amount); err != nil {
		return err
	}
	log.Debugf("transferring %d tokens from %s to %s", amount, from.String(), to.String())
	if err := v.SetAccount(from, accFrom); err != nil {
		return err
	}
	if err := v.SetAccount(to, accTo); err != nil {
		return err
	}
	return nil
}

// MintBalance increments the existing acc of address by amount
func (v *State) MintBalance(address common.Address, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("cannot mint a zero amount balance")
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return fmt.Errorf("mintBalance: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if acc.Balance+amount <= acc.Balance {
		return ErrBalanceOverflow
	}
	acc.Balance += amount
	log.Debugf("minting %d tokens to account %s", amount, address.String())
	return v.SetAccount(address, acc)
}

// GetAccount retrives the Account for an address.
// Returns a nil account and no error if the account does not exist.
// committed is relative to the state on which the function is executed
func (v *State) GetAccount(address common.Address, committed bool) (*Account, error) {
	var acc Account
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(committed).DeepGet(address.Bytes(), AccountsCfg)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &acc, acc.Unmarshal(raw)
}

// AccountFromSignature extracts an address from a signed message and returns an account if exists
func (v *State) AccountFromSignature(message, signature []byte) (*common.Address, *Account, error) {
	pubKey, err := ethereum.PubKeyFromSignature(message, signature)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	address, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	if address == types.EthereumZeroAddress {
		return &common.Address{}, nil, fmt.Errorf("invalid address")
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot get account: %w", err)
	}
	if acc == nil {
		return &common.Address{}, nil, ErrAccountNotExist
	}
	return &address, acc, nil
}

// SetAccountInfoURI sets a given account infoURI
func (v *State) SetAccountInfoURI(accountAddress common.Address, infoURI string) error {
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return err
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	acc.InfoURI = infoURI
	log.Debugf("setting account %s infoURI %s", accountAddress.String(), infoURI)
	return v.SetAccount(accountAddress, acc)
}

// Create account creates an account
func (v *State) CreateAccount(accountAddress common.Address,
	infoURI string,
	delegates []common.Address,
	initBalance uint64,
) error {
	// check valid address
	if accountAddress == types.EthereumZeroAddress {
		return fmt.Errorf("invalid address")
	}
	// check not created
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return fmt.Errorf("cannot create account %s: %w", accountAddress.String(), err)
	}
	if acc != nil {
		return fmt.Errorf("account %s already exists", accountAddress.String())
	}
	acc = &Account{}
	// account not found, creating it
	// check valid infoURI, must be set on creation
	acc.InfoURI = infoURI
	acc.Balance = initBalance
	if len(delegates) > 0 {
		acc.DelegateAddrs = make([][]byte, len(delegates))
		for _, v := range delegates {
			if !bytes.Equal(v.Bytes(), types.EthereumZeroAddress[:]) {
				acc.DelegateAddrs = append(acc.DelegateAddrs, v.Bytes())
			}
		}
	}
	log.Debugf("creating account %s with infoURI %s balance %d nonce %d and delegates %+v",
		accountAddress.String(),
		acc.InfoURI,
		acc.Balance,
		acc.Nonce,
		printPrettierDelegates(acc.DelegateAddrs),
	)
	return v.SetAccount(accountAddress, acc)
}

// ConsumeFaucetPayload consumes a given faucet payload and sends the given amount of tokens to
// the address pointed by the payload.
func (v *State) ConsumeFaucetPayload(from common.Address, faucetPayload *models.FaucetPayload, txType models.TxType) error {
	// check faucet payload
	if faucetPayload == nil {
		return fmt.Errorf("faucet payload is nil")
	}
	// check from account
	if from == types.EthereumZeroAddress {
		return fmt.Errorf("invalid from account")
	}
	var accFrom, accTo *Account
	var err error
	// get from account
	accFrom, err = v.GetAccount(from, false)
	if err != nil {
		return err
	}
	if accFrom == nil {
		return ErrAccountNotExist
	}
	// get to account
	accToAddr := common.BytesToAddress(faucetPayload.To)
	accTo, err = v.GetAccount(accToAddr, false)
	if err != nil {
		return err
	}
	if accTo == nil {
		return ErrAccountNotExist
	}

	// get burn account
	burnAcc, err := v.GetAccount(BurnAddress, false)
	if err != nil {
		return err
	}
	if burnAcc == nil {
		return ErrAccountNotExist
	}

	// transfer amout to faucetPayload.To
	if err := accFrom.Transfer(accTo, faucetPayload.Amount); err != nil {
		return fmt.Errorf("cannot transfer balance to burn account: %w", err)
	}

	// burn the tx fee (by sending to burn address)
	// SetAccountInfo: burn tokens
	// CollectFaucetTx: tokens are burned elsewhere, so just increment the sender's nonce
	if txType == models.TxType_SET_ACCOUNT_INFO {
		collectFaucetCost, err := v.TxCost(models.TxType_COLLECT_FAUCET, false)
		if err != nil {
			return err
		}
		if err := accFrom.Transfer(burnAcc, collectFaucetCost); err != nil {
			return fmt.Errorf("cannot transfer balance to burn account: %w", err)
		}
	} else {
		accTo.Nonce++
	}

	// store faucet identifier
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	key := from.Bytes()
	key = append(key, b...)
	keyHash := crypto.Sha256(key)
	if err := v.SetFaucetNonce(keyHash); err != nil {
		return err
	}

	log.Debugf("account %s consuming faucet payload created by %s with amount %d and identifier %d (keyHash: %x)",
		accToAddr.String(),
		from.String(),
		faucetPayload.Amount,
		faucetPayload.Identifier,
		keyHash,
	)

	// set accounts
	if err := v.SetAccount(from, accFrom); err != nil {
		return err
	}

	if err := v.SetAccount(accToAddr, accTo); err != nil {
		return err
	}
	if err := v.SetAccount(BurnAddress, burnAcc); err != nil {
		return err
	}
	return nil
}

// SetAccountInfoTxCheck is an abstraction of ABCI checkTx for an SetAccountInfoTx transaction
// If the bool returned is true means that the account does not exist and is going to be created
func SetAccountInfoTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*setAccountInfoTxCheckValues, error) {
	if vtx == nil {
		return nil, ErrNilTx
	}
	returnValues := &setAccountInfoTxCheckValues{}
	tx := vtx.GetSetAccountInfo()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	// check infoURI
	infoURI := tx.GetInfoURI()
	if infoURI == "" {
		return nil, fmt.Errorf("invalid URI, cannot be empty")
	}
	// recover txSender address from signature
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	returnValues.TxSender, err = ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// get txSender account
	txSender, err := state.GetAccount(returnValues.TxSender, false)
	if err != nil {
		return nil, fmt.Errorf("cannot check if account %s exists: %w", returnValues.TxSender.String(), err)
	}
	// check tx.Account exists
	returnValues.Account = common.BytesToAddress(tx.Account)
	acc, err := state.GetAccount(returnValues.Account, false)
	if err != nil {
		return nil, fmt.Errorf("cannot check if account %s exists: %w", tx.Account, err)
	}
	if acc == nil {
		returnValues.Account = returnValues.TxSender
	}
	// if not exist create new one
	if txSender == nil {
		returnValues.CreateAccount = true
		if tx.FaucetPackage == nil {
			return returnValues, nil
		}
		returnValues.CreateAccountWithFaucet = true
		if tx.FaucetPackage.Payload == nil {
			return nil, fmt.Errorf("faucet payload is nil")
		}
		if !bytes.Equal(tx.FaucetPackage.Payload.To, returnValues.TxSender.Bytes()) {
			return nil, fmt.Errorf("payload to and tx sender missmatch")
		}
		// get issuer address from faucetPayload
		faucetPkgPayload := tx.FaucetPackage.GetPayload()
		faucetPackageBytes, err := proto.Marshal(faucetPkgPayload)
		if err != nil {
			return nil, fmt.Errorf("cannot extract faucet package payload: %w", err)
		}
		issuerAddress, err := ethereum.AddrFromSignature(faucetPackageBytes, tx.FaucetPackage.Signature)
		if err != nil {
			return nil, err
		}
		// check issuer nonce not used
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, faucetPkgPayload.Identifier)
		key := issuerAddress.Bytes()
		key = append(key, b...)
		used, err := state.FaucetNonce(crypto.Sha256(key), false)
		if err != nil {
			return nil, fmt.Errorf("cannot check faucet nonce: %w", err)
		}
		if used {
			return nil, fmt.Errorf("faucet package identifier %d already used", faucetPkgPayload.Identifier)
		}
		// check issuer have enough funds
		issuerAcc, err := state.GetAccount(issuerAddress, false)
		if err != nil {
			return nil, fmt.Errorf("cannot get faucet account: %w", err)
		}
		if issuerAcc == nil {
			return nil, fmt.Errorf("the account signing the faucet payload does not exist")
		}
		cost, err := state.TxCost(models.TxType_COLLECT_FAUCET, false)
		if err != nil {
			return nil, fmt.Errorf("cannot get %s tx cost: %w", models.TxType_COLLECT_FAUCET, err)
		}
		if issuerAcc.Balance < faucetPkgPayload.Amount+cost {
			return nil, fmt.Errorf("faucet does not have enough balance %d < %d", issuerAcc.Balance, faucetPkgPayload.Amount+cost)
		}
		returnValues.FaucetPayloadSigner = issuerAddress
		return returnValues, nil
	}
	// check if delegate
	if returnValues.TxSender != returnValues.Account {
		if !acc.IsDelegate(returnValues.TxSender) {
			return nil, fmt.Errorf("tx sender is not a delegate")
		}
	}
	// check txSender nonce
	if tx.Nonce != txSender.Nonce {
		return nil, fmt.Errorf("invalid nonce, expected %d got %d", txSender.Nonce, tx.Nonce)
	}
	// get tx cost
	cost, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO, false)
	if err != nil {
		return nil, err
	}
	// check txSender balance
	if txSender.Balance < cost {
		return nil, fmt.Errorf("unauthorized: %s", ErrNotEnoughBalance)
	}
	return returnValues, nil
}

// SetAccount sets the given account data to the state
func (v *State) SetAccount(accountAddress common.Address, account *Account) error {
	accBytes, err := proto.Marshal(account)
	if err != nil {
		return err
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	log.Debugf("setAccount: {address %s, nonce %d, infoURI %s, balance: %d, delegates: %+v}",
		accountAddress.String(),
		account.Nonce,
		account.InfoURI,
		account.Balance,
		printPrettierDelegates(account.DelegateAddrs),
	)
	return v.Tx.DeepSet(accountAddress.Bytes(), accBytes, AccountsCfg)
}

// SubtractCostIncrementNonce
func (v *State) SubtractCostIncrementNonce(accountAddress common.Address, txType models.TxType) error {
	// get account
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return fmt.Errorf("subtractCostIncrementNonce: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	// get tx cost
	cost, err := v.TxCost(txType, false)
	if err != nil {
		return fmt.Errorf("subtractCostIncrementNonce: %w", err)
	}
	// increment nonce
	if txType != models.TxType_COLLECT_FAUCET {
		acc.Nonce++
	}
	// send cost to burn address
	burnAcc, err := v.GetAccount(BurnAddress, false)
	if err != nil {
		return fmt.Errorf("subtractCostIncrementNonce: %w", err)
	}
	if burnAcc == nil {
		return fmt.Errorf("subtractCostIncrementNonce: burn account does not exist")
	}
	if err := acc.Transfer(burnAcc, cost); err != nil {
		return fmt.Errorf("subtractCostIncrementNonce: %w", err)
	}
	log.Debugf("burning fee for tx %s with cost %d from account %s", txType.String(), cost, accountAddress.String())
	// set accounts
	if err := v.SetAccount(accountAddress, acc); err != nil {
		return fmt.Errorf("subtractCostIncrementNonce: %w", err)
	}
	if err := v.SetAccount(BurnAddress, burnAcc); err != nil {
		return fmt.Errorf("subtractCostIncrementNonce: %w", err)
	}
	return nil
}

// MintTokensTxCheck checks if a given MintTokensTx and its data are valid
func MintTokensTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (common.Address, uint64, error) {
	tx := vtx.GetMintTokens()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return common.Address{}, 0, fmt.Errorf("missing signature and/or transaction")
	}
	// check value
	if tx.Value <= 0 {
		return common.Address{}, 0, fmt.Errorf("invalid value")
	}
	// check to
	if len(tx.To) != types.EntityIDsize || bytes.Equal(tx.To, types.EthereumZeroAddress[:]) {
		return common.Address{}, 0, fmt.Errorf("invalid To address")
	}
	// get treasurer
	treasurer, err := state.Treasurer(false)
	if err != nil {
		return common.Address{}, 0, err
	}
	// check nonce
	if tx.Nonce != treasurer.Nonce {
		return common.Address{}, 0, fmt.Errorf("invalid nonce %d, expected: %d", tx.Nonce, treasurer.Nonce)
	}
	// check to acc exist
	toAddr := common.BytesToAddress(tx.To)
	toAcc, err := state.GetAccount(toAddr, false)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("MintTokensTxCheck: %w", err)
	}
	if toAcc == nil {
		return common.Address{}, 0, fmt.Errorf("MintTokensTxCheck: %w", ErrAccountNotExist)
	}
	// get address from signature
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	sigAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return common.Address{}, 0, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// check signature recovered address
	treasurerAddress := common.BytesToAddress(treasurer.Address)
	if treasurerAddress != sigAddress {
		return common.Address{}, 0, fmt.Errorf(
			"address recovered not treasurer: expected %s got %s",
			treasurerAddress.String(),
			sigAddress.String(),
		)
	}
	return toAddr, tx.Value, nil
}

// SendTokensTxCheck checks if a given SendTokensTx and its data are valid
func SendTokensTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*sendTokensTxCheckValues, error) {
	if vtx == nil {
		return nil, ErrNilTx
	}
	tx := vtx.GetSendTokens()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	// check value
	if tx.Value == 0 {
		return nil, fmt.Errorf("invalid value")
	}
	// check from
	if tx.From == nil {
		return nil, fmt.Errorf("from field not found")
	}
	if bytes.Equal(tx.From, types.EthereumZeroAddress[:]) {
		return nil, fmt.Errorf("invalid from address")
	}
	// check to
	if tx.To == nil {
		return nil, fmt.Errorf("to field not found")
	}
	if bytes.Equal(tx.To, types.EthereumZeroAddress[:]) {
		return nil, fmt.Errorf("invalid to address")
	}
	// get address from signature
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	sigAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// check from
	txFromAddress := common.BytesToAddress(tx.From)
	if txFromAddress != sigAddress {
		return nil, fmt.Errorf("from (%s) field and extracted signature (%s) mismatch",
			txFromAddress.String(),
			sigAddress.String(),
		)
	}
	// check to
	txToAddress := common.BytesToAddress(tx.To)
	toTxAccount, err := state.GetAccount(txToAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get to account info: %w", err)
	}
	if toTxAccount == nil {
		return nil, ErrAccountNotExist
	}
	// check nonce
	acc, err := state.GetAccount(sigAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get account info: %w", err)
	}
	if acc == nil {
		return nil, ErrAccountNotExist
	}
	if tx.Nonce != acc.Nonce {
		return nil, fmt.Errorf("invalid nonce, expected %d got %d", acc.Nonce, tx.Nonce)
	}
	// get tx cost
	cost, err := state.TxCost(models.TxType_SEND_TOKENS, false)
	if err != nil {
		return nil, err
	}
	// check value
	if (tx.Value + cost) > acc.Balance {
		return nil, ErrNotEnoughBalance
	}
	return &sendTokensTxCheckValues{sigAddress, txToAddress, tx.Value, tx.Nonce}, nil
}

// SetAccountDelegateTxCheck checks if a SetAccountDelegateTx and its data are valid
func SetAccountDelegateTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*setAccountDelegateTxCheckValues, error) {
	if vtx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	tx := vtx.GetSetAccountDelegateTx()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	// check delegate
	delAcc := common.BytesToAddress(tx.Delegate)
	if delAcc == types.EthereumZeroAddress {
		return nil, fmt.Errorf("invalid delegate address")
	}
	// get sender
	sigAddress, acc, err := state.AccountFromSignature(txBytes, signature)
	if err != nil {
		return nil, err
	}
	// check delegate to add is not itself
	if delAcc == *sigAddress {
		return nil, fmt.Errorf("cannot add self to delegates list")
	}
	// check nonce
	if tx.Nonce != acc.Nonce {
		return nil, fmt.Errorf("invalid nonce, expected %d got %d", acc.Nonce, tx.Nonce)
	}
	// check tx cost
	cost, err := state.TxCost(tx.Txtype, false)
	if err != nil {
		return nil, err
	}
	if acc.Balance < cost {
		return nil, ErrNotEnoughBalance
	}
	// check tx type
	switch tx.Txtype {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		if acc.IsDelegate(delAcc) {
			return nil, fmt.Errorf("already added")
		}
		return &setAccountDelegateTxCheckValues{From: *sigAddress, Delegate: delAcc}, nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		for i := 0; i < len(acc.DelegateAddrs); i++ {
			delegateToCmp := common.BytesToAddress(acc.DelegateAddrs[i])
			if delegateToCmp == delAcc {
				return &setAccountDelegateTxCheckValues{From: *sigAddress, Delegate: delAcc}, nil
			}
		}
		return nil, fmt.Errorf("cannot remove a non existent delegate")
	default:
		return nil, fmt.Errorf("unsupported SetAccountDelegate operation")
	}
}

// SetDelegate sets a delegate for a given account
func (v *State) SetAccountDelegate(accountAddr, delegateAddr common.Address, txType models.TxType) error {
	// get account
	acc, err := v.GetAccount(accountAddr, false)
	if err != nil {
		return err
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	switch txType {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		log.Debugf("adding delegate %s for account %s", delegateAddr.String(), accountAddr.String())
		if err := acc.AddDelegate(delegateAddr); err != nil {
			return fmt.Errorf("cannot add delegate, AddDelegate: %w", err)
		}
		return v.SetAccount(accountAddr, acc)
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		log.Debugf("deleting delegate %s for account %s", delegateAddr.String(), accountAddr.String())
		acc.DelDelegate(delegateAddr)
		return v.SetAccount(accountAddr, acc)
	default:
		return fmt.Errorf("invalid setDelegate tx type")
	}
}

// CollectFaucetTxCheck checks if a CollectFaucetTx and its data are valid
func CollectFaucetTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*common.Address, error) {
	if vtx == nil {
		return nil, ErrNilTx
	}
	tx := vtx.GetCollectFaucet()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	faucetPkg := tx.GetFaucetPackage()
	// check faucet pkg content
	if faucetPkg == nil {
		return nil, fmt.Errorf("nil faucet package")
	}
	if faucetPkg.Signature == nil {
		return nil, fmt.Errorf("invalid faucet package signature")
	}
	if faucetPkg.Payload == nil {
		return nil, fmt.Errorf("invalid faucet package payload")
	}
	if faucetPkg.Payload.Amount == 0 {
		return nil, fmt.Errorf("invalid faucet package payload amount")
	}
	// recover txSender address from signature
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	toAddr, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// extract the account that generated the payload
	faucetPackageBytes, err := proto.Marshal(faucetPkg.Payload)
	if err != nil {
		return nil, fmt.Errorf("cannot extract faucet package payload: %w", err)
	}
	fromAddr, err := ethereum.AddrFromSignature(faucetPackageBytes, tx.FaucetPackage.Signature)
	if err != nil {
		return nil, err
	}
	// check issuer nonce not used
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPkg.Payload.Identifier)
	key := fromAddr.Bytes()
	key = append(key, b...)
	used, err := state.FaucetNonce(crypto.Sha256(key), false)
	if err != nil {
		return nil, fmt.Errorf("cannot check faucet nonce: %w", err)
	}
	if used {
		return nil, fmt.Errorf("nonce %d already used", faucetPkg.Payload.Identifier)
	}
	// check issuer have enough funds
	issuerAcc, err := state.GetAccount(fromAddr, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get faucet account: %w", err)
	}
	if issuerAcc == nil {
		return nil, fmt.Errorf("the account signing the faucet payload does not exist")
	}
	cost, err := state.TxCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get %s tx cost: %w", models.TxType_COLLECT_FAUCET, err)
	}
	if issuerAcc.Balance < faucetPkg.Payload.Amount+cost {
		return nil, fmt.Errorf("faucet does not have enough balance %d < %d", issuerAcc.Balance, faucetPkg.Payload.Amount+cost)
	}
	// check tx sender is the same as the one contained in the payload
	if !bytes.Equal(toAddr.Bytes(), faucetPkg.Payload.To) {
		return nil, fmt.Errorf("txSender %x and faucet payload To %x mismatch",
			toAddr,
			common.BytesToAddress(faucetPkg.Payload.To),
		)
	}
	// get txSender account
	txSender, err := state.GetAccount(toAddr, false)
	if err != nil {
		return nil, fmt.Errorf("cannot check if account %s exists: %w", toAddr.String(), err)
	}
	if txSender == nil {
		return nil, ErrAccountNotExist
	}
	// check valid txSender nonce
	if txSender.Nonce != tx.GetNonce() {
		return nil, fmt.Errorf("invalid nonce")
	}
	return &fromAddr, nil
}

// GenerateFaucetPackage generates a faucet package
func GenerateFaucetPackage(from *ethereum.SignKeys, to common.Address, value, identifier uint64) (*models.FaucetPackage, error) {
	rand.Seed(time.Now().UnixNano())
	payload := &models.FaucetPayload{
		Identifier: identifier,
		To:         to.Bytes(),
		Amount:     value,
	}
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadSignature, err := from.SignEthereum(payloadBytes)
	if err != nil {
		return nil, err
	}
	return &models.FaucetPackage{
		Payload:   payload,
		Signature: payloadSignature,
	}, nil
}
