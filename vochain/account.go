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
// If provided nonce does not match origin address nonce+1, ErrAccountNonceInvalid is returned.
func (v *State) TransferBalance(from, to common.Address, amount uint64, nonce uint32) error {
	var accFrom, accTo *Account
	var err error
	if accFrom, err = v.GetAccount(from, false); err != nil {
		return err
	}
	if accFrom == nil {
		return ErrAccountNotExist
	}
	if accFrom.Nonce != nonce {
		return ErrAccountNonceInvalid
	}
	if accTo, err = v.GetAccount(to, false); err != nil || accTo == nil {
		return err
	}
	if accTo == nil {
		return ErrAccountNotExist
	}
	if err := accFrom.Transfer(accTo, amount); err != nil {
		return err
	}
	accFrom.Nonce++
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
		return fmt.Errorf("mintBalance (%w)", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	if acc.Balance+amount <= acc.Balance {
		return ErrBalanceOverflow
	}
	acc.Balance += amount
	return v.SetAccount(address, acc)
}

// GetAccount retrives the Account for an address.
// Returns a nil account and no error if the account does not exist.
func (v *State) GetAccount(address common.Address, isQuery bool) (*Account, error) {
	var acc Account
	if !isQuery {
		v.Tx.RLock()
		defer v.Tx.RUnlock()
	}
	raw, err := v.mainTreeViewer(isQuery).DeepGet(address.Bytes(), AccountsCfg)
	if errors.Is(err, arbo.ErrKeyNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &acc, acc.Unmarshal(raw)
}

// VerifyAccountBalance extracts an account address from a signed message, and verifies if
// there is enough balance to cover an amount expense
func (v *State) VerifyAccountBalance(message, signature []byte, amount uint64) (bool, common.Address, error) {
	address, err := ethereum.AddrFromSignature(message, signature)
	if err != nil {
		return false, address, err
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return false, address, fmt.Errorf("VerifyAccountWithAmmount: %v", err)
	}
	if acc == nil {
		return false, address, nil
	}
	return acc.Balance >= amount, address, nil
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
	if address == types.EthereumZeroAddressBytes {
		return &common.Address{}, nil, fmt.Errorf("invalid address")
	}
	acc, err := v.GetAccount(address, false)
	if err != nil {
		return &common.Address{}, nil, fmt.Errorf("cannot get account: %v", err)
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
	return v.SetAccount(accountAddress, acc)
}

// Create account creates an account
func (v *State) CreateAccount(accountAddress common.Address,
	infoURI string,
	delegates []common.Address,
	initBalance uint64,
) error {
	// check valid address
	if bytes.Equal(accountAddress.Bytes(), types.EthereumZeroAddressBytes[:]) {
		return fmt.Errorf("invalid address")
	}
	// check not created
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return fmt.Errorf("cannot create account %s: %v", accountAddress.String(), err)
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
			if !bytes.Equal(v.Bytes(), types.EthereumZeroAddressBytes[:]) {
				acc.DelegateAddrs = append(acc.DelegateAddrs, v.Bytes())
			}
		}
	}
	return v.SetAccount(accountAddress, acc)
}

// ConsumeFaucetPayload consumes a given faucet payload and sends the given amount of tokens to
// the address pointed by the payload.
func (v *State) ConsumeFaucetPayload(from common.Address, faucetPayload *models.FaucetPayload, isNewAccount bool) error {
	// check faucet payload
	if faucetPayload == nil {
		return fmt.Errorf("faucet payload is nil")
	}
	// check from account
	if bytes.Equal(from.Bytes(), types.EthereumZeroAddressBytes[:]) {
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
		return fmt.Errorf("cannot transfer balance: %w", err)
	}

	// transfer tx cost to burn address
	collectFaucetCost, err := v.TxCost(models.TxType_COLLECT_FAUCET, false)
	if err != nil {
		return err
	}
	if err := accFrom.Transfer(burnAcc, collectFaucetCost); err != nil {
		return fmt.Errorf("cannot transfer balance: %w", err)
	}

	// store faucet identifier
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, faucetPayload.Identifier)
	key := from.Bytes()
	key = append(key, b...)
	if err := v.Tx.DeepSet(crypto.Sha256(key), nil, FaucetNonceCfg); err != nil {
		return err
	}

	// set accounts
	if err := v.SetAccount(from, accFrom); err != nil {
		return err
	}
	// if account is already created increment nonce
	if !isNewAccount {
		accTo.Nonce++
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
	// get tx cost
	cost, err := state.TxCost(models.TxType_SET_ACCOUNT_INFO, false)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("cannot check if account %s exists: %v", returnValues.TxSender.String(), err)
	}
	// get tx.Account, if not valid set to txSender address
	accountAddressBytes := tx.GetAccount()
	if accountAddressBytes == nil ||
		len(accountAddressBytes) != types.EntityIDsize ||
		bytes.Equal(accountAddressBytes, types.EthereumZeroAddressBytes[:]) {
		returnValues.Account = returnValues.TxSender
	} else {
		returnValues.Account = common.BytesToAddress(accountAddressBytes)
	}
	// if not exist create new one
	if txSender == nil {
		if tx.FaucetPackage != nil {
			if tx.FaucetPackage.Payload != nil {
				if bytes.Equal(tx.FaucetPackage.Payload.To, returnValues.TxSender.Bytes()) {
					// get issuer address from faucetPayload
					faucetPkgPayload := tx.FaucetPackage.GetPayload()
					faucetPackageBytes, err := proto.Marshal(faucetPkgPayload)
					if err != nil {
						return nil, fmt.Errorf("cannot extract faucet package payload: %v", err)
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
						return nil, fmt.Errorf("cannot check faucet nonce: %v", err)
					}
					if used {
						return nil, fmt.Errorf("nonce %d already used", faucetPkgPayload.Identifier)
					}
					// check issuer have enough funds
					issuerAcc, err := state.GetAccount(issuerAddress, false)
					if err != nil {
						return nil, fmt.Errorf("cannot get faucet account: %v", err)
					}
					if issuerAcc == nil {
						return nil, fmt.Errorf("the account signing the faucet payload does not exist")
					}
					cost, err := state.TxCost(models.TxType_COLLECT_FAUCET, false)
					if err != nil {
						return nil, fmt.Errorf("cannot get %s tx cost: %w", models.TxType_COLLECT_FAUCET, err)
					}
					if (issuerAcc.Balance) < faucetPkgPayload.Amount+cost {
						return nil, fmt.Errorf("faucet does not have enough balance %d < %d", issuerAcc.Balance, faucetPkgPayload.Amount+cost)
					}
					returnValues.FaucetPayloadSigner = issuerAddress
					returnValues.FaucetPayload = tx.GetFaucetPackage().Payload
				} else {
					return nil, fmt.Errorf("payload to and tx sender missmatch")
				}
			} else {
				return nil, fmt.Errorf("faucet payload is nil")
			}
		}
		returnValues.Create = true
		return returnValues, nil
	}

	// check tx.Account exists
	acc, err := state.GetAccount(returnValues.Account, false)
	if err != nil {
		return nil, fmt.Errorf("cannot check if account %s exists: %v", returnValues.Account.String(), err)
	}
	if acc == nil {
		return nil, fmt.Errorf("tx.Account account does not exist")
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
	// check txSender balance
	if txSender.Balance < cost {
		return nil, fmt.Errorf("unauthorized: %s", ErrNotEnoughBalance)
	}
	// check not the same infoURI
	if acc.InfoURI == infoURI {
		return nil, fmt.Errorf("same infoURI: %s", infoURI)
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
	return v.Tx.DeepSet(accountAddress.Bytes(), accBytes, AccountsCfg)
}

// SubstractCostIncrementNonce
func (v *State) SubstractCostIncrementNonce(accountAddress common.Address, txType models.TxType) error {
	// get account
	acc, err := v.GetAccount(accountAddress, false)
	if err != nil {
		return fmt.Errorf("substractCostIncrementNonce: %w", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}
	// get tx cost
	cost, err := v.TxCost(txType, false)
	if err != nil {
		return fmt.Errorf("substractCostIncrementNonce: %w", err)
	}
	// increment nonce
	acc.Nonce++
	// send cost to burn address
	burnAcc, err := v.GetAccount(BurnAddress, false)
	if err != nil {
		return fmt.Errorf("substractCostIncrementNonce: %w", err)
	}
	if burnAcc == nil {
		return fmt.Errorf("substractCostIncrementNonce: burn account does not exist")
	}
	if err := acc.Transfer(burnAcc, cost); err != nil {
		return fmt.Errorf("substractCostIncrementNonce: %w", err)
	}
	// set accounts
	if err := v.SetAccount(accountAddress, acc); err != nil {
		return fmt.Errorf("substractCostIncrementNonce: %w", err)
	}
	if err := v.SetAccount(BurnAddress, burnAcc); err != nil {
		return fmt.Errorf("substractCostIncrementNonce: %w", err)
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
	if len(tx.To) != types.EntityIDsize || bytes.Equal(tx.To, types.EthereumZeroAddressBytes[:]) {
		return common.Address{}, 0, fmt.Errorf("invalid To address")
	}
	// get treasurer
	treasurer, err := state.Treasurer(false)
	if err != nil {
		return common.Address{}, 0, err
	}
	// check nonce
	if tx.Nonce != treasurer.Nonce {
		return common.Address{}, 0, fmt.Errorf("invalid nonce %d, expected: %d", tx.Nonce, treasurer.Nonce+1)
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

// GenerateFaucetPackage generates a faucet package
func GenerateFaucetPackage(from *ethereum.SignKeys, to common.Address, value uint64) (*models.FaucetPackage, error) {
	rand.Seed(time.Now().UnixNano())
	payload := &models.FaucetPayload{
		Identifier: rand.Uint64(),
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
