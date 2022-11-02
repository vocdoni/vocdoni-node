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

// CreateAccount creates an account
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
		return ErrAccountAlreadyExists
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
	log.Debugf("creating account %s with infoURI %s balance %d and delegates %+v",
		accountAddress.String(),
		acc.InfoURI,
		acc.Balance,
		printPrettierDelegates(acc.DelegateAddrs),
	)
	return v.SetAccount(accountAddress, acc)
}

// ConsumeFaucetPayload consumes a given faucet payload
// storing its key to the FaucetNonce tree so it can
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

// SetAccountInfoTxCheck is an abstraction of ABCI checkTx for an SetAccountInfoTx transaction
// If createAccount on SetAccountInfoTxValues is set to true it means
// that the account does not exists and is going to be created if a faucet
// payload is provided
func SetAccountInfoTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*setAccountTxCheckValues, error) {
	if vtx == nil {
		return nil, ErrNilTx
	}
	setAccountTxValues := &setAccountTxCheckValues{
		Delegates:     make([]common.Address, 0),
		FaucetPayload: &models.FaucetPayload{},
	}
	tx := vtx.GetSetAccount()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil || tx.Nonce == nil || tx.InfoURI == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	// recover txSender address from signature
	pubKey, err := ethereum.PubKeyFromSignature(txBytes, signature)
	if err != nil {
		return nil, fmt.Errorf("cannot extract public key from signature: %w", err)
	}
	// set txSender
	setAccountTxValues.TxSender, err = ethereum.AddrFromPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("cannot extract address from public key: %w", err)
	}
	// set txAccount
	if len(tx.Account) == 0 || bytes.Equal(tx.Account, types.EthereumZeroAddress.Bytes()) {
		setAccountTxValues.TxAccount = setAccountTxValues.TxSender
	} else {
		setAccountTxValues.TxAccount = common.BytesToAddress(tx.Account)
	}
	// get txSender account
	txSenderAccount, err := state.GetAccount(setAccountTxValues.TxSender, false)
	if err != nil {
		return nil, fmt.Errorf("cannot check if account %s exists: %w", setAccountTxValues.TxSender, err)
	}
	// account exists
	if txSenderAccount != nil {
		// check txSender nonce
		if *tx.Nonce != txSenderAccount.Nonce {
			return nil, fmt.Errorf(
				"invalid nonce, expected %d got %d",
				txSenderAccount.Nonce,
				tx.Nonce,
			)
		}
		// get setAccount tx cost
		costSetAccountInfoURI, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO_URI, false)
		if err != nil {
			return nil, err
		}
		// check tx sender balance
		if txSenderAccount.Balance < costSetAccountInfoURI {
			return nil, fmt.Errorf("unauthorized: %s", ErrNotEnoughBalance)
		}
		// check info URI
		if len(*tx.InfoURI) == 0 {
			return nil, fmt.Errorf("invalid URI, cannot be empty")
		}
		if setAccountTxValues.TxSender == setAccountTxValues.TxAccount {
			if *tx.InfoURI == txSenderAccount.InfoURI {
				return nil, fmt.Errorf("invalid URI, must be different")
			}
			return setAccountTxValues, nil
		}
		// if txSender != txAccount only delegate operations
		// get tx account Account
		txAccountAccount, err := state.GetAccount(setAccountTxValues.TxAccount, false)
		if err != nil {
			return nil, fmt.Errorf("cannot get tx account: %w", err)
		}
		if txAccountAccount == nil {
			return nil, ErrAccountNotExist
		}
		if *tx.InfoURI == txAccountAccount.InfoURI {
			return nil, fmt.Errorf("invalid URI, must be different")
		}
		// check if delegate
		if !txAccountAccount.IsDelegate(setAccountTxValues.TxSender) {
			return nil, fmt.Errorf("tx sender is not a delegate")
		}
		return setAccountTxValues, nil
	}
	// account does not exist
	if tx.FaucetPackage == nil {
		return nil, fmt.Errorf("invalid faucet package provided")
	}
	if tx.FaucetPackage.Payload == nil {
		return nil, fmt.Errorf("invalid faucet package payload")
	}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, setAccountTxValues.FaucetPayload); err != nil {
		return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
	}
	if setAccountTxValues.FaucetPayload.Amount == 0 {
		return nil, fmt.Errorf("invalid faucet payload amount provided")
	}
	if setAccountTxValues.FaucetPayload.To == nil {
		return nil, fmt.Errorf("invalid to address provided")
	}
	if !bytes.Equal(setAccountTxValues.FaucetPayload.To, setAccountTxValues.TxSender.Bytes()) {
		return nil, fmt.Errorf("payload to and tx sender missmatch (%x != %x)",
			setAccountTxValues.FaucetPayload.To, setAccountTxValues.TxSender.Bytes())
	}
	// get issuer address from faucetPayload
	issuerAddress, err := ethereum.AddrFromSignature(tx.FaucetPackage.Payload, tx.FaucetPackage.Signature)
	if err != nil {
		return nil, err
	}
	// check issuer have enough funds
	issuerAcc, err := state.GetAccount(issuerAddress, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get faucet account: %w", err)
	}
	if issuerAcc == nil {
		return nil, fmt.Errorf("the account signing the faucet payload does not exist (%x)", issuerAddress)
	}
	// check issuer nonce not used
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, setAccountTxValues.FaucetPayload.Identifier)
	key := issuerAddress.Bytes()
	key = append(key, b...)
	used, err := state.FaucetNonce(crypto.Sha256(key), false)
	if err != nil {
		return nil, fmt.Errorf("cannot check faucet nonce: %w", err)
	}
	if used {
		return nil, fmt.Errorf("faucet package identifier %d already used", setAccountTxValues.FaucetPayload.Identifier)
	}
	cost, err := state.TxCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return nil, fmt.Errorf("cannot get %s tx cost: %w", models.TxType_COLLECT_FAUCET, err)
	}
	if issuerAcc.Balance < setAccountTxValues.FaucetPayload.Amount+cost {
		return nil, fmt.Errorf("faucet does not have enough balance %d, required %d", issuerAcc.Balance, setAccountTxValues.FaucetPayload.Amount+cost)
	}
	setAccountTxValues.FaucetPayloadSigner = issuerAddress
	setAccountTxValues.CreateAccount = true

	return setAccountTxValues, nil
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
func SetAccountDelegateTxCheck(vtx *models.Tx, txBytes, signature []byte, state *State) (*setAccountTxCheckValues, error) {
	if vtx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	tx := vtx.GetSetAccount()
	// check signature available
	if signature == nil || tx == nil || txBytes == nil || tx.Nonce == nil {
		return nil, fmt.Errorf("missing signature and/or transaction")
	}
	// check delegates
	delegates := make([]common.Address, 0)
	for _, del := range tx.Delegates {
		delAddr := common.BytesToAddress(del)
		if delAddr == types.EthereumZeroAddress {
			return nil, fmt.Errorf("invalid delegate address")
		}
		delegates = append(delegates, delAddr)
	}

	// get sender
	sigAddress, acc, err := state.AccountFromSignature(txBytes, signature)
	if err != nil {
		return nil, err
	}
	// check not itself
	for _, del := range delegates {
		if *sigAddress == del {
			return nil, fmt.Errorf("cannot self add/del to delegates list")
		}
	}
	// check nonce
	if *tx.Nonce != acc.Nonce {
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
		toAdd := make([]common.Address, 0)
		for _, del := range delegates {
			if !acc.IsDelegate(del) {
				toAdd = append(toAdd, del)
			}
		}
		// if no delegates to add fail
		if len(toAdd) == 0 {
			return nil, fmt.Errorf("no delegates to add")
		}
		return &setAccountTxCheckValues{TxSender: *sigAddress, Delegates: toAdd}, nil
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		toDelete := make([]common.Address, 0)
		for _, del := range delegates {
			if acc.IsDelegate(del) {
				toDelete = append(toDelete, del)
			}
		}
		// if no delegates to delete fail
		if len(toDelete) == 0 {
			return nil, fmt.Errorf("no delegates to delete")
		}
		return &setAccountTxCheckValues{TxSender: *sigAddress, Delegates: toDelete}, nil
	default:
		return nil, fmt.Errorf("unsupported SetAccountDelegate operation")
	}
}

// SetAccountDelegate sets a delegate for a given account
func (v *State) SetAccountDelegate(accountAddr common.Address, delegateAddrs []common.Address, txType models.TxType) error {
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
		log.Debugf("adding delegates %+v for account %s", delegateAddrs, accountAddr.String())
		for _, delegate := range delegateAddrs {
			if err := acc.AddDelegate(delegate); err != nil {
				return fmt.Errorf("cannot add delegate, AddDelegate: %w", err)
			}
		}
		return v.SetAccount(accountAddr, acc)
	case models.TxType_DEL_DELEGATE_FOR_ACCOUNT:
		log.Debugf("deleting delegates %+v for account %s", delegateAddrs, accountAddr.String())
		for _, delegate := range delegateAddrs {
			acc.DelDelegate(delegate)
		}
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

	faucetPayload := &models.FaucetPayload{}
	if err := proto.Unmarshal(tx.FaucetPackage.Payload, faucetPayload); err != nil {
		return nil, fmt.Errorf("could not unmarshal faucet package: %w", err)
	}

	if faucetPayload.Amount == 0 {
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
	faucetPackageBytes, err := proto.Marshal(faucetPayload)
	if err != nil {
		return nil, fmt.Errorf("cannot extract faucet package payload: %w", err)
	}
	fromAddr, err := ethereum.AddrFromSignature(faucetPackageBytes, tx.FaucetPackage.Signature)
	if err != nil {
		return nil, err
	}
	// check issuer nonce not used
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	key := fromAddr.Bytes()
	key = append(key, b...)
	used, err := state.FaucetNonce(crypto.Sha256(key), false)
	if err != nil {
		return nil, fmt.Errorf("cannot check faucet nonce: %w", err)
	}
	if used {
		return nil, fmt.Errorf("nonce %d already used", faucetPayload.Identifier)
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
	if issuerAcc.Balance < faucetPayload.Amount+cost {
		return nil, fmt.Errorf("faucet does not have enough balance %d, required %d", issuerAcc.Balance, faucetPayload.Amount+cost)
	}
	// check tx sender is the same as the one contained in the payload
	if !bytes.Equal(toAddr.Bytes(), faucetPayload.To) {
		return nil, fmt.Errorf("txSender %x and faucet payload To %x mismatch",
			toAddr,
			common.BytesToAddress(faucetPayload.To),
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
		Payload:   payloadBytes,
		Signature: payloadSignature,
	}, nil
}
