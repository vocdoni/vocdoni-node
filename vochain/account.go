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
	for _, l := range v.eventListeners {
		l.OnTransferTokens(from.Bytes(), to.Bytes(), amount)
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

// BurnTxCost burns the cost of a transaction
// if cost is set to 0 just return
func (v *State) BurnTxCost(from common.Address, cost uint64) error {
	if cost != 0 {
		return v.TransferBalance(from, BurnAddress, cost)
	}
	return nil
}

// GetAccount retrives the Account for an address.
// Returns a nil account and no error if the account does not exist.
// Committed is relative to the state on which the function is executed.
func (v *State) GetAccount(address common.Address, committed bool) (*Account, error) {
	var acc Account
	if !committed {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(committed).DeepGet(address.Bytes(), StateTreeCfg(TreeAccounts))
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
	if acc.InfoURI == infoURI {
		return fmt.Errorf("same infoURI")
	}
	if infoURI == "" || len(infoURI) > types.MaxURLLength {
		return fmt.Errorf("invalid infoURI")
	}
	acc.InfoURI = infoURI
	log.Debugf("setting account %s infoURI %s", accountAddress.String(), infoURI)
	return v.SetAccount(accountAddress, acc)
}

// IncrementAccountProcessIndex increments the process index by one and stores the value
func (v *State) IncrementAccountProcessIndex(accountAddress common.Address) error {
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return err
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	// safety check for overflow protection, we allow a maximum of 4M of processes per account
	if acc.ProcessIndex > 1<<22 {
		acc.ProcessIndex = 0
	}
	acc.ProcessIndex++
	log.Debugf("setting account %s process index to %d", accountAddress.String(), acc.ProcessIndex)
	return v.SetAccount(accountAddress, acc)
}

// ConsumeFaucetPayload consumes a given faucet payload storing
// its key to the FaucetNonce tree so it can
// only be used once
func (v *State) ConsumeFaucetPayload(from common.Address, faucetPayload *models.FaucetPayload) error {
	// check faucet payload
	if faucetPayload == nil {
		return fmt.Errorf("invalid faucet payload")
	}
	// check from account
	if from == types.EthereumZeroAddress {
		return fmt.Errorf("invalid from address")
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
	log.Debugf("consuming faucet payload created by %s with amount %d and identifier %d (keyHash: %x)",
		from.String(),
		faucetPayload.Amount,
		faucetPayload.Identifier,
		keyHash,
	)
	return nil
}

// CreateAccountTxCheck is an abstraction of the ABCI CheckTx method for creating an account
func CreateAccountTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil || txBytes == nil || signature == nil || state == nil {
		return fmt.Errorf("invalid parameters provided, cannot check create account tx")
	}
	tx := vtx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Txtype != models.TxType_CREATE_ACCOUNT {
		return fmt.Errorf("invalid tx type, expected %s, got %s", models.TxType_CREATE_ACCOUNT, tx.Txtype)
	}
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txSenderAcc, err := state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get account: %w", err)
	}
	if txSenderAcc != nil {
		return ErrAccountAlreadyExists
	}
	infoURI := tx.GetInfoURI()
	if len(infoURI) > types.MaxURLLength {
		return ErrInvalidURILength
	}
	if len(tx.Delegates) != 0 {
		delegatesMap := make(map[common.Address]bool)
		for _, v := range tx.Delegates {
			delegateAddress := common.BytesToAddress(v)
			if delegateAddress == types.EthereumZeroAddress {
				return ErrInvalidAddress
			}
			if _, ok := delegatesMap[delegateAddress]; !ok {
				delegatesMap[delegateAddress] = true
			} else {
				return fmt.Errorf("duplicate delegate address %s", delegateAddress)
			}
		}
	}
	txCost, err := state.TxCost(models.TxType_CREATE_ACCOUNT, false)
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
		return fmt.Errorf("cannot extract issuer address from faucet package signature: %w", err)
	}
	issuerAcc, err := state.GetAccount(issuerAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get faucet issuer address account: %w", err)
	}
	if issuerAcc == nil {
		return fmt.Errorf("the account signing the faucet payload does not exist (%s)", issuerAddress.String())
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	key := issuerAddress.Bytes()
	key = append(key, b...)
	keyHash := crypto.Sha256(key)
	used, err := state.FaucetNonce(keyHash, false)
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

// CreateAccount creates an account
func (v *State) CreateAccount(accountAddress common.Address,
	infoURI string,
	delegates [][]byte) error {
	return v.createAccount(accountAddress, infoURI, delegates, 0)
}

func (v *State) createAccount(accountAddress common.Address,
	infoURI string,
	delegates [][]byte,
	initialBalance uint64,
) error {
	newAccount := &Account{}
	if infoURI != "" && len(infoURI) <= types.MaxURLLength {
		newAccount.InfoURI = infoURI
	}
	if len(delegates) > 0 {
		newAccount.DelegateAddrs = append(newAccount.DelegateAddrs, delegates...)
	}
	newAccount.Balance = initialBalance
	log.Debugf("creating account %s with infoURI %s balance %d and delegates %+v",
		accountAddress.String(),
		newAccount.InfoURI,
		newAccount.Balance,
		printPrettierDelegates(newAccount.DelegateAddrs),
	)
	return v.SetAccount(accountAddress, newAccount)
}

// SetAccountInfoTxCheck is an abstraction of ABCI checkTx for an SetAccountInfoTx transaction
func SetAccountInfoTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil || signature == nil || txBytes == nil || state == nil {
		return fmt.Errorf("invalid transaction parameters provided")
	}
	tx := vtx.GetSetAccount()
	if tx == nil {
		return fmt.Errorf("invalid transaction")
	}
	if tx.Nonce == nil || tx.InfoURI == nil {
		return fmt.Errorf("missing signature and/or transaction")
	}
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	var txAccountAddress common.Address
	if len(tx.Account) == 0 || bytes.Equal(tx.Account, types.EthereumZeroAddress.Bytes()) {
		txAccountAddress = txSenderAddress
	} else {
		txAccountAddress = common.BytesToAddress(tx.Account)
	}
	txSenderAccount, err := state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot check if account %s exists: %w", txSenderAddress, err)
	}
	if txSenderAccount == nil {
		return ErrAccountNotExist
	}
	// check txSender nonce
	if tx.GetNonce() != txSenderAccount.Nonce {
		return fmt.Errorf(
			"invalid nonce, expected %d got %d",
			txSenderAccount.Nonce,
			tx.Nonce,
		)
	}
	// get setAccount tx cost
	costSetAccountInfoURI, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO_URI, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	// check tx sender balance
	if txSenderAccount.Balance < costSetAccountInfoURI {
		return fmt.Errorf("unauthorized: %s", ErrNotEnoughBalance)
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
	txAccountAccount, err := state.GetAccount(txAccountAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get tx account: %w", err)
	}
	if txAccountAccount == nil {
		return ErrAccountNotExist
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

// SetAccount sets the given account data to the state
func (v *State) SetAccount(accountAddress common.Address, account *Account) error {
	accBytes, err := proto.Marshal(account)
	if err != nil {
		return err
	}
	log.Debugf(`setAccount: {
			address %s,
			nonce %d,
			infoURI %s,
			balance: %d,
			delegates: %+v,
			processIndex: %d
		}`,
		accountAddress.String(),
		account.Nonce,
		account.InfoURI,
		account.Balance,
		printPrettierDelegates(account.DelegateAddrs),
		account.ProcessIndex,
	)
	for _, l := range v.eventListeners {
		l.OnSetAccount(accountAddress.Bytes(), account)
	}
	v.Tx.Lock()
	defer v.Tx.Unlock()
	return v.Tx.DeepSet(accountAddress.Bytes(), accBytes, StateTreeCfg(TreeAccounts))
}

// BurnTxCostIncrementNonce reduces the transaction cost from the account balance and increments nonce
func (v *State) BurnTxCostIncrementNonce(accountAddress common.Address, txType models.TxType) error {
	// get tx cost
	cost, err := v.TxCost(txType, false)
	if err != nil {
		return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
	}
	// get account
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if cost != 0 {
		// send cost to burn address
		burnAcc, err := v.GetAccount(BurnAddress, false)
		if err != nil {
			return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
		}
		if burnAcc == nil {
			return fmt.Errorf("burnTxCostIncrementNonce: burn account does not exist")
		}
		if err := acc.Transfer(burnAcc, cost); err != nil {
			return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
		}
		log.Debugf("burning fee for tx %s with cost %d from account %s", txType.String(), cost, accountAddress.String())
		if err := v.SetAccount(BurnAddress, burnAcc); err != nil {
			return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
		}
	}
	acc.Nonce++
	if err := v.SetAccount(accountAddress, acc); err != nil {
		return fmt.Errorf("burnTxCostIncrementNonce: %w", err)
	}
	return nil
}

// MintTokensTxCheck checks if a given MintTokensTx and its data are valid
func MintTokensTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil || txBytes == nil || signature == nil || state == nil {
		return fmt.Errorf("invalid parameters")
	}
	tx := vtx.GetMintTokens()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Value <= 0 {
		return fmt.Errorf("invalid value")
	}
	if len(tx.To) == 0 || bytes.Equal(tx.To, types.EthereumZeroAddress.Bytes()) {
		return fmt.Errorf("invalid To address")
	}
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	treasurer, err := state.Treasurer(false)
	if err != nil {
		return err
	}
	treasurerAddress := common.BytesToAddress(treasurer.Address)
	if treasurerAddress != txSenderAddress {
		return fmt.Errorf(
			"address recovered not treasurer: expected %s got %s",
			treasurerAddress.String(),
			txSenderAddress.String(),
		)
	}
	if tx.Nonce != treasurer.Nonce {
		return fmt.Errorf("invalid nonce %d, expected: %d", tx.Nonce, treasurer.Nonce)
	}
	toAddr := common.BytesToAddress(tx.To)
	toAcc, err := state.GetAccount(toAddr, false)
	if err != nil {
		return fmt.Errorf("cannot get to account: %w", err)
	}
	if toAcc == nil {
		return ErrAccountNotExist
	}
	return nil
}

// SendTokensTxCheck checks if a given SendTokensTx and its data are valid
func SendTokensTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil || signature == nil || txBytes == nil || state == nil {
		return fmt.Errorf("invalid parameters")
	}
	tx := vtx.GetSendTokens()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	if tx.Value == 0 {
		return fmt.Errorf("invalid value")
	}
	if len(tx.From) == 0 || bytes.Equal(tx.From, types.EthereumZeroAddress.Bytes()) {
		return fmt.Errorf("invalid from address")
	}
	if len(tx.To) == 0 || bytes.Equal(tx.To, types.EthereumZeroAddress.Bytes()) {
		return fmt.Errorf("invalid to address")
	}
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	txFromAddress := common.BytesToAddress(tx.From)
	if txFromAddress != txSenderAddress {
		return fmt.Errorf("from (%s) field and extracted signature (%s) mismatch",
			txFromAddress.String(),
			txSenderAddress.String(),
		)
	}
	txToAddress := common.BytesToAddress(tx.To)
	toTxAccount, err := state.GetAccount(txToAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get to account: %w", err)
	}
	if toTxAccount == nil {
		return ErrAccountNotExist
	}
	acc, err := state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot get from account: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if tx.Nonce != acc.Nonce {
		return fmt.Errorf("invalid nonce, expected %d got %d", acc.Nonce, tx.Nonce)
	}
	cost, err := state.TxCost(models.TxType_SEND_TOKENS, false)
	if err != nil {
		return err
	}
	if (tx.Value + cost) > acc.Balance {
		return ErrNotEnoughBalance
	}
	return nil
}

// SetAccountDelegateTxCheck checks if a SetAccountDelegateTx and its data are valid
func SetAccountDelegateTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil || signature == nil || txBytes == nil || state == nil {
		return fmt.Errorf("invalid parameters")
	}
	tx := vtx.GetSetAccount()
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
	txSenderAddress, txSenderAccount, err := state.AccountFromSignature(txBytes, signature)
	if err != nil {
		return err
	}
	delegatesToSetMap := make(map[common.Address]bool)
	for _, delegate := range tx.Delegates {
		delegateAddress := common.BytesToAddress(delegate)
		if delegateAddress == types.EthereumZeroAddress {
			return ErrInvalidAddress
		}
		if delegateAddress == *txSenderAddress {
			return fmt.Errorf("delegate cannot be the same as the sender")
		}
		if _, ok := delegatesToSetMap[delegateAddress]; !ok {
			delegatesToSetMap[delegateAddress] = true
			continue
		}
		return fmt.Errorf("duplicate delegate address %s", delegateAddress)
	}
	if tx.GetNonce() != txSenderAccount.Nonce {
		return fmt.Errorf("invalid nonce, expected %d got %d", txSenderAccount.Nonce, tx.Nonce)
	}
	cost, err := state.TxCost(tx.Txtype, false)
	if err != nil {
		return fmt.Errorf("cannot get tx cost: %w", err)
	}
	if txSenderAccount.Balance < cost {
		return ErrNotEnoughBalance
	}
	switch tx.Txtype {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range txSenderAccount.DelegateAddrs {
			delegateAddress := common.BytesToAddress(delegate)
			if _, ok := delegatesToSetMap[delegateAddress]; ok {
				return fmt.Errorf("delegate %s already exists", delegateAddress.String())
			}
		}
		return nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		for _, delegate := range txSenderAccount.DelegateAddrs {
			delegateAddress := common.BytesToAddress(delegate)
			if _, ok := delegatesToSetMap[delegateAddress]; !ok {
				return fmt.Errorf("delegate %s does not exist", delegateAddress.String())
			}
		}
		return nil
	default:
		// should never happen
		return fmt.Errorf("invalid tx type")
	}
}

// SetAccountDelegate sets a set of delegates for a given account
func (v *State) SetAccountDelegate(accountAddr common.Address,
	delegateAddrs [][]byte,
	txType models.TxType) error {
	acc, err := v.GetAccount(accountAddr, false)
	if err != nil {
		return err
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	switch txType {
	case models.TxType_ADD_DELEGATE_FOR_ACCOUNT:
		log.Debugf("adding delegates %+v for account %s", delegateAddrs, accountAddr.String())
		for _, delegate := range delegateAddrs {
			if err := acc.AddDelegate(common.BytesToAddress(delegate)); err != nil {
				return fmt.Errorf("cannot add delegate, AddDelegate: %w", err)
			}
		}
		return v.SetAccount(accountAddr, acc)
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		log.Debugf("deleting delegates %+v for account %s", delegateAddrs, accountAddr.String())
		for _, delegate := range delegateAddrs {
			acc.DelDelegate(common.BytesToAddress(delegate))
		}
		return v.SetAccount(accountAddr, acc)
	default:
		return fmt.Errorf("invalid tx type")
	}
}

// CollectFaucetTxCheck checks if a CollectFaucetTx and its data are valid
func CollectFaucetTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) error {
	if vtx == nil || signature == nil || txBytes == nil || state == nil {
		return fmt.Errorf("invalid parameters")
	}
	tx := vtx.GetCollectFaucet()
	if tx == nil {
		return fmt.Errorf("invalid tx")
	}
	faucetPkg := tx.GetFaucetPackage()
	if faucetPkg == nil {
		return fmt.Errorf("nil faucet package")
	}
	if faucetPkg.Signature == nil {
		return fmt.Errorf("invalid faucet package signature")
	}
	if faucetPkg.Payload == nil {
		return fmt.Errorf("invalid faucet package payload")
	}
	faucetPayload := &models.FaucetPayload{}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
		return fmt.Errorf("could not unmarshal faucet package: %w", err)
	}
	if faucetPayload.Amount == 0 {
		return fmt.Errorf("invalid faucet package payload amount")
	}
	if len(faucetPayload.To) == 0 {
		return fmt.Errorf("invalid faucet package payload to")
	}
	payloadToAddress := common.BytesToAddress(faucetPayload.To)
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	txSenderAddress, err := ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("cannot extract address from public key: %w", err)
	}
	if txSenderAddress != payloadToAddress {
		return fmt.Errorf("txSender %s and faucet payload to %s mismatch",
			txSenderAddress,
			payloadToAddress,
		)
	}
	txSenderAccount, err := state.GetAccount(txSenderAddress, false)
	if err != nil {
		return fmt.Errorf("cannot check if account %s exists: %w", txSenderAddress.String(), err)
	}
	if txSenderAccount == nil {
		return ErrAccountNotExist
	}
	if txSenderAccount.Nonce != tx.Nonce {
		return fmt.Errorf("invalid nonce")
	}
	fromAddr, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
	if err != nil {
		return fmt.Errorf("cannot extract address from faucet package signature: %w", err)
	}
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	key := fromAddr.Bytes()
	key = append(key, b...)
	used, err := state.FaucetNonce(crypto.Sha256(key), false)
	if err != nil {
		return fmt.Errorf("cannot check faucet nonce: %w", err)
	}
	if used {
		return fmt.Errorf("faucet payload already used")
	}
	issuerAcc, err := state.GetAccount(fromAddr, false)
	if err != nil {
		return fmt.Errorf("cannot get faucet account: %w", err)
	}
	if issuerAcc == nil {
		return fmt.Errorf("the account signing the faucet payload does not exist")
	}
	cost, err := state.TxCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return fmt.Errorf("cannot get %s tx cost: %w", models.TxType_COLLECT_FAUCET, err)
	}
	if issuerAcc.Balance < faucetPayload.Amount+cost {
		return fmt.Errorf("faucet does not have enough balance %d, required %d", issuerAcc.Balance, faucetPayload.Amount+cost)
	}
	return nil
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
		Payload:   payloadBytes,
		Signature: payloadSignature,
	}, nil
}
