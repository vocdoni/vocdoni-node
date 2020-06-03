// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// EntityResolverABI is the input ABI used to generate the binding from.
const EntityResolverABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"interfaceID\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"value\",\"type\":\"string\"}],\"name\":\"pushListText\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"value\",\"type\":\"string\"}],\"name\":\"setText\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"}],\"name\":\"addr\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"}],\"name\":\"text\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"removeListIndex\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"}],\"name\":\"list\",\"outputs\":[{\"name\":\"\",\"type\":\"string[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"entityAddress\",\"type\":\"address\"}],\"name\":\"getEntityId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"index\",\"type\":\"uint256\"},{\"name\":\"value\",\"type\":\"string\"}],\"name\":\"setListText\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setAddr\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"node\",\"type\":\"bytes32\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"listText\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"key\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"ListItemChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"key\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"ListItemRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"indexedKey\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"key\",\"type\":\"string\"}],\"name\":\"TextChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"node\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"a\",\"type\":\"address\"}],\"name\":\"AddrChanged\",\"type\":\"event\"}]"

// EntityResolver is an auto generated Go binding around an Ethereum contract.
type EntityResolver struct {
	EntityResolverCaller     // Read-only binding to the contract
	EntityResolverTransactor // Write-only binding to the contract
	EntityResolverFilterer   // Log filterer for contract events
}

// EntityResolverCaller is an auto generated read-only Go binding around an Ethereum contract.
type EntityResolverCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EntityResolverTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EntityResolverTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EntityResolverFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EntityResolverFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EntityResolverSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EntityResolverSession struct {
	Contract     *EntityResolver   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EntityResolverCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EntityResolverCallerSession struct {
	Contract *EntityResolverCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// EntityResolverTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EntityResolverTransactorSession struct {
	Contract     *EntityResolverTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// EntityResolverRaw is an auto generated low-level Go binding around an Ethereum contract.
type EntityResolverRaw struct {
	Contract *EntityResolver // Generic contract binding to access the raw methods on
}

// EntityResolverCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EntityResolverCallerRaw struct {
	Contract *EntityResolverCaller // Generic read-only contract binding to access the raw methods on
}

// EntityResolverTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EntityResolverTransactorRaw struct {
	Contract *EntityResolverTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEntityResolver creates a new instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolver(address common.Address, backend bind.ContractBackend) (*EntityResolver, error) {
	contract, err := bindEntityResolver(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &EntityResolver{EntityResolverCaller: EntityResolverCaller{contract: contract}, EntityResolverTransactor: EntityResolverTransactor{contract: contract}, EntityResolverFilterer: EntityResolverFilterer{contract: contract}}, nil
}

// NewEntityResolverCaller creates a new read-only instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolverCaller(address common.Address, caller bind.ContractCaller) (*EntityResolverCaller, error) {
	contract, err := bindEntityResolver(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EntityResolverCaller{contract: contract}, nil
}

// NewEntityResolverTransactor creates a new write-only instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolverTransactor(address common.Address, transactor bind.ContractTransactor) (*EntityResolverTransactor, error) {
	contract, err := bindEntityResolver(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EntityResolverTransactor{contract: contract}, nil
}

// NewEntityResolverFilterer creates a new log filterer instance of EntityResolver, bound to a specific deployed contract.
func NewEntityResolverFilterer(address common.Address, filterer bind.ContractFilterer) (*EntityResolverFilterer, error) {
	contract, err := bindEntityResolver(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EntityResolverFilterer{contract: contract}, nil
}

// bindEntityResolver binds a generic wrapper to an already deployed contract.
func bindEntityResolver(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(EntityResolverABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EntityResolver *EntityResolverRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _EntityResolver.Contract.EntityResolverCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EntityResolver *EntityResolverRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EntityResolver.Contract.EntityResolverTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EntityResolver *EntityResolverRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EntityResolver.Contract.EntityResolverTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_EntityResolver *EntityResolverCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _EntityResolver.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_EntityResolver *EntityResolverTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _EntityResolver.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_EntityResolver *EntityResolverTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _EntityResolver.Contract.contract.Transact(opts, method, params...)
}

// Addr is a free data retrieval call binding the contract method 0x3b3b57de.
//
// Solidity: function addr(bytes32 node) constant returns(address)
func (_EntityResolver *EntityResolverCaller) Addr(opts *bind.CallOpts, node [32]byte) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _EntityResolver.contract.Call(opts, out, "addr", node)
	return *ret0, err
}

// Addr is a free data retrieval call binding the contract method 0x3b3b57de.
//
// Solidity: function addr(bytes32 node) constant returns(address)
func (_EntityResolver *EntityResolverSession) Addr(node [32]byte) (common.Address, error) {
	return _EntityResolver.Contract.Addr(&_EntityResolver.CallOpts, node)
}

// Addr is a free data retrieval call binding the contract method 0x3b3b57de.
//
// Solidity: function addr(bytes32 node) constant returns(address)
func (_EntityResolver *EntityResolverCallerSession) Addr(node [32]byte) (common.Address, error) {
	return _EntityResolver.Contract.Addr(&_EntityResolver.CallOpts, node)
}

// GetEntityId is a free data retrieval call binding the contract method 0x997fde61.
//
// Solidity: function getEntityId(address entityAddress) constant returns(bytes32)
func (_EntityResolver *EntityResolverCaller) GetEntityId(opts *bind.CallOpts, entityAddress common.Address) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _EntityResolver.contract.Call(opts, out, "getEntityId", entityAddress)
	return *ret0, err
}

// GetEntityId is a free data retrieval call binding the contract method 0x997fde61.
//
// Solidity: function getEntityId(address entityAddress) constant returns(bytes32)
func (_EntityResolver *EntityResolverSession) GetEntityId(entityAddress common.Address) ([32]byte, error) {
	return _EntityResolver.Contract.GetEntityId(&_EntityResolver.CallOpts, entityAddress)
}

// GetEntityId is a free data retrieval call binding the contract method 0x997fde61.
//
// Solidity: function getEntityId(address entityAddress) constant returns(bytes32)
func (_EntityResolver *EntityResolverCallerSession) GetEntityId(entityAddress common.Address) ([32]byte, error) {
	return _EntityResolver.Contract.GetEntityId(&_EntityResolver.CallOpts, entityAddress)
}

// List is a free data retrieval call binding the contract method 0x74c756ee.
//
// Solidity: function list(bytes32 node, string key) constant returns(string[])
func (_EntityResolver *EntityResolverCaller) List(opts *bind.CallOpts, node [32]byte, key string) ([]string, error) {
	var (
		ret0 = new([]string)
	)
	out := ret0
	err := _EntityResolver.contract.Call(opts, out, "list", node, key)
	return *ret0, err
}

// List is a free data retrieval call binding the contract method 0x74c756ee.
//
// Solidity: function list(bytes32 node, string key) constant returns(string[])
func (_EntityResolver *EntityResolverSession) List(node [32]byte, key string) ([]string, error) {
	return _EntityResolver.Contract.List(&_EntityResolver.CallOpts, node, key)
}

// List is a free data retrieval call binding the contract method 0x74c756ee.
//
// Solidity: function list(bytes32 node, string key) constant returns(string[])
func (_EntityResolver *EntityResolverCallerSession) List(node [32]byte, key string) ([]string, error) {
	return _EntityResolver.Contract.List(&_EntityResolver.CallOpts, node, key)
}

// ListText is a free data retrieval call binding the contract method 0xfdf720c6.
//
// Solidity: function listText(bytes32 node, string key, uint256 index) constant returns(string)
func (_EntityResolver *EntityResolverCaller) ListText(opts *bind.CallOpts, node [32]byte, key string, index *big.Int) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _EntityResolver.contract.Call(opts, out, "listText", node, key, index)
	return *ret0, err
}

// ListText is a free data retrieval call binding the contract method 0xfdf720c6.
//
// Solidity: function listText(bytes32 node, string key, uint256 index) constant returns(string)
func (_EntityResolver *EntityResolverSession) ListText(node [32]byte, key string, index *big.Int) (string, error) {
	return _EntityResolver.Contract.ListText(&_EntityResolver.CallOpts, node, key, index)
}

// ListText is a free data retrieval call binding the contract method 0xfdf720c6.
//
// Solidity: function listText(bytes32 node, string key, uint256 index) constant returns(string)
func (_EntityResolver *EntityResolverCallerSession) ListText(node [32]byte, key string, index *big.Int) (string, error) {
	return _EntityResolver.Contract.ListText(&_EntityResolver.CallOpts, node, key, index)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceID) constant returns(bool)
func (_EntityResolver *EntityResolverCaller) SupportsInterface(opts *bind.CallOpts, interfaceID [4]byte) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _EntityResolver.contract.Call(opts, out, "supportsInterface", interfaceID)
	return *ret0, err
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceID) constant returns(bool)
func (_EntityResolver *EntityResolverSession) SupportsInterface(interfaceID [4]byte) (bool, error) {
	return _EntityResolver.Contract.SupportsInterface(&_EntityResolver.CallOpts, interfaceID)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceID) constant returns(bool)
func (_EntityResolver *EntityResolverCallerSession) SupportsInterface(interfaceID [4]byte) (bool, error) {
	return _EntityResolver.Contract.SupportsInterface(&_EntityResolver.CallOpts, interfaceID)
}

// Text is a free data retrieval call binding the contract method 0x59d1d43c.
//
// Solidity: function text(bytes32 node, string key) constant returns(string)
func (_EntityResolver *EntityResolverCaller) Text(opts *bind.CallOpts, node [32]byte, key string) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _EntityResolver.contract.Call(opts, out, "text", node, key)
	return *ret0, err
}

// Text is a free data retrieval call binding the contract method 0x59d1d43c.
//
// Solidity: function text(bytes32 node, string key) constant returns(string)
func (_EntityResolver *EntityResolverSession) Text(node [32]byte, key string) (string, error) {
	return _EntityResolver.Contract.Text(&_EntityResolver.CallOpts, node, key)
}

// Text is a free data retrieval call binding the contract method 0x59d1d43c.
//
// Solidity: function text(bytes32 node, string key) constant returns(string)
func (_EntityResolver *EntityResolverCallerSession) Text(node [32]byte, key string) (string, error) {
	return _EntityResolver.Contract.Text(&_EntityResolver.CallOpts, node, key)
}

// PushListText is a paid mutator transaction binding the contract method 0x043a728d.
//
// Solidity: function pushListText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactor) PushListText(opts *bind.TransactOpts, node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "pushListText", node, key, value)
}

// PushListText is a paid mutator transaction binding the contract method 0x043a728d.
//
// Solidity: function pushListText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverSession) PushListText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.PushListText(&_EntityResolver.TransactOpts, node, key, value)
}

// PushListText is a paid mutator transaction binding the contract method 0x043a728d.
//
// Solidity: function pushListText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactorSession) PushListText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.PushListText(&_EntityResolver.TransactOpts, node, key, value)
}

// RemoveListIndex is a paid mutator transaction binding the contract method 0x6f473720.
//
// Solidity: function removeListIndex(bytes32 node, string key, uint256 index) returns()
func (_EntityResolver *EntityResolverTransactor) RemoveListIndex(opts *bind.TransactOpts, node [32]byte, key string, index *big.Int) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "removeListIndex", node, key, index)
}

// RemoveListIndex is a paid mutator transaction binding the contract method 0x6f473720.
//
// Solidity: function removeListIndex(bytes32 node, string key, uint256 index) returns()
func (_EntityResolver *EntityResolverSession) RemoveListIndex(node [32]byte, key string, index *big.Int) (*types.Transaction, error) {
	return _EntityResolver.Contract.RemoveListIndex(&_EntityResolver.TransactOpts, node, key, index)
}

// RemoveListIndex is a paid mutator transaction binding the contract method 0x6f473720.
//
// Solidity: function removeListIndex(bytes32 node, string key, uint256 index) returns()
func (_EntityResolver *EntityResolverTransactorSession) RemoveListIndex(node [32]byte, key string, index *big.Int) (*types.Transaction, error) {
	return _EntityResolver.Contract.RemoveListIndex(&_EntityResolver.TransactOpts, node, key, index)
}

// SetAddr is a paid mutator transaction binding the contract method 0xd5fa2b00.
//
// Solidity: function setAddr(bytes32 node, address addr) returns()
func (_EntityResolver *EntityResolverTransactor) SetAddr(opts *bind.TransactOpts, node [32]byte, addr common.Address) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setAddr", node, addr)
}

// SetAddr is a paid mutator transaction binding the contract method 0xd5fa2b00.
//
// Solidity: function setAddr(bytes32 node, address addr) returns()
func (_EntityResolver *EntityResolverSession) SetAddr(node [32]byte, addr common.Address) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAddr(&_EntityResolver.TransactOpts, node, addr)
}

// SetAddr is a paid mutator transaction binding the contract method 0xd5fa2b00.
//
// Solidity: function setAddr(bytes32 node, address addr) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetAddr(node [32]byte, addr common.Address) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetAddr(&_EntityResolver.TransactOpts, node, addr)
}

// SetListText is a paid mutator transaction binding the contract method 0xa2df33e1.
//
// Solidity: function setListText(bytes32 node, string key, uint256 index, string value) returns()
func (_EntityResolver *EntityResolverTransactor) SetListText(opts *bind.TransactOpts, node [32]byte, key string, index *big.Int, value string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setListText", node, key, index, value)
}

// SetListText is a paid mutator transaction binding the contract method 0xa2df33e1.
//
// Solidity: function setListText(bytes32 node, string key, uint256 index, string value) returns()
func (_EntityResolver *EntityResolverSession) SetListText(node [32]byte, key string, index *big.Int, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetListText(&_EntityResolver.TransactOpts, node, key, index, value)
}

// SetListText is a paid mutator transaction binding the contract method 0xa2df33e1.
//
// Solidity: function setListText(bytes32 node, string key, uint256 index, string value) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetListText(node [32]byte, key string, index *big.Int, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetListText(&_EntityResolver.TransactOpts, node, key, index, value)
}

// SetText is a paid mutator transaction binding the contract method 0x10f13a8c.
//
// Solidity: function setText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactor) SetText(opts *bind.TransactOpts, node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.contract.Transact(opts, "setText", node, key, value)
}

// SetText is a paid mutator transaction binding the contract method 0x10f13a8c.
//
// Solidity: function setText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverSession) SetText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetText(&_EntityResolver.TransactOpts, node, key, value)
}

// SetText is a paid mutator transaction binding the contract method 0x10f13a8c.
//
// Solidity: function setText(bytes32 node, string key, string value) returns()
func (_EntityResolver *EntityResolverTransactorSession) SetText(node [32]byte, key string, value string) (*types.Transaction, error) {
	return _EntityResolver.Contract.SetText(&_EntityResolver.TransactOpts, node, key, value)
}

// EntityResolverAddrChangedIterator is returned from FilterAddrChanged and is used to iterate over the raw logs and unpacked data for AddrChanged events raised by the EntityResolver contract.
type EntityResolverAddrChangedIterator struct {
	Event *EntityResolverAddrChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverAddrChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverAddrChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverAddrChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverAddrChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverAddrChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverAddrChanged represents a AddrChanged event raised by the EntityResolver contract.
type EntityResolverAddrChanged struct {
	Node [32]byte
	A    common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterAddrChanged is a free log retrieval operation binding the contract event 0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2.
//
// Solidity: event AddrChanged(bytes32 indexed node, address a)
func (_EntityResolver *EntityResolverFilterer) FilterAddrChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverAddrChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "AddrChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverAddrChangedIterator{contract: _EntityResolver.contract, event: "AddrChanged", logs: logs, sub: sub}, nil
}

// WatchAddrChanged is a free log subscription operation binding the contract event 0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2.
//
// Solidity: event AddrChanged(bytes32 indexed node, address a)
func (_EntityResolver *EntityResolverFilterer) WatchAddrChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverAddrChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "AddrChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverAddrChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "AddrChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAddrChanged is a log parse operation binding the contract event 0x52d7d861f09ab3d26239d492e8968629f95e9e318cf0b73bfddc441522a15fd2.
//
// Solidity: event AddrChanged(bytes32 indexed node, address a)
func (_EntityResolver *EntityResolverFilterer) ParseAddrChanged(log types.Log) (*EntityResolverAddrChanged, error) {
	event := new(EntityResolverAddrChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "AddrChanged", log); err != nil {
		return nil, err
	}
	return event, nil
}

// EntityResolverListItemChangedIterator is returned from FilterListItemChanged and is used to iterate over the raw logs and unpacked data for ListItemChanged events raised by the EntityResolver contract.
type EntityResolverListItemChangedIterator struct {
	Event *EntityResolverListItemChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverListItemChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverListItemChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverListItemChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverListItemChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverListItemChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverListItemChanged represents a ListItemChanged event raised by the EntityResolver contract.
type EntityResolverListItemChanged struct {
	Node  [32]byte
	Key   string
	Index *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterListItemChanged is a free log retrieval operation binding the contract event 0xad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3.
//
// Solidity: event ListItemChanged(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) FilterListItemChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverListItemChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "ListItemChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverListItemChangedIterator{contract: _EntityResolver.contract, event: "ListItemChanged", logs: logs, sub: sub}, nil
}

// WatchListItemChanged is a free log subscription operation binding the contract event 0xad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3.
//
// Solidity: event ListItemChanged(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) WatchListItemChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverListItemChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "ListItemChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverListItemChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "ListItemChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseListItemChanged is a log parse operation binding the contract event 0xad6a325380d85c50dfaa38511cbbbc97c46b9a52e342c8d705bf5515d0ef45b3.
//
// Solidity: event ListItemChanged(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) ParseListItemChanged(log types.Log) (*EntityResolverListItemChanged, error) {
	event := new(EntityResolverListItemChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "ListItemChanged", log); err != nil {
		return nil, err
	}
	return event, nil
}

// EntityResolverListItemRemovedIterator is returned from FilterListItemRemoved and is used to iterate over the raw logs and unpacked data for ListItemRemoved events raised by the EntityResolver contract.
type EntityResolverListItemRemovedIterator struct {
	Event *EntityResolverListItemRemoved // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverListItemRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverListItemRemoved)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverListItemRemoved)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverListItemRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverListItemRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverListItemRemoved represents a ListItemRemoved event raised by the EntityResolver contract.
type EntityResolverListItemRemoved struct {
	Node  [32]byte
	Key   string
	Index *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterListItemRemoved is a free log retrieval operation binding the contract event 0x5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb49.
//
// Solidity: event ListItemRemoved(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) FilterListItemRemoved(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverListItemRemovedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "ListItemRemoved", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverListItemRemovedIterator{contract: _EntityResolver.contract, event: "ListItemRemoved", logs: logs, sub: sub}, nil
}

// WatchListItemRemoved is a free log subscription operation binding the contract event 0x5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb49.
//
// Solidity: event ListItemRemoved(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) WatchListItemRemoved(opts *bind.WatchOpts, sink chan<- *EntityResolverListItemRemoved, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "ListItemRemoved", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverListItemRemoved)
				if err := _EntityResolver.contract.UnpackLog(event, "ListItemRemoved", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseListItemRemoved is a log parse operation binding the contract event 0x5dbbe7a1e616b629a61480b13fd4e89dfe8b604d35802cdf7b0bb03687e3eb49.
//
// Solidity: event ListItemRemoved(bytes32 indexed node, string key, uint256 index)
func (_EntityResolver *EntityResolverFilterer) ParseListItemRemoved(log types.Log) (*EntityResolverListItemRemoved, error) {
	event := new(EntityResolverListItemRemoved)
	if err := _EntityResolver.contract.UnpackLog(event, "ListItemRemoved", log); err != nil {
		return nil, err
	}
	return event, nil
}

// EntityResolverTextChangedIterator is returned from FilterTextChanged and is used to iterate over the raw logs and unpacked data for TextChanged events raised by the EntityResolver contract.
type EntityResolverTextChangedIterator struct {
	Event *EntityResolverTextChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EntityResolverTextChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EntityResolverTextChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EntityResolverTextChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EntityResolverTextChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EntityResolverTextChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EntityResolverTextChanged represents a TextChanged event raised by the EntityResolver contract.
type EntityResolverTextChanged struct {
	Node       [32]byte
	IndexedKey string
	Key        string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterTextChanged is a free log retrieval operation binding the contract event 0xd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a7550.
//
// Solidity: event TextChanged(bytes32 indexed node, string indexedKey, string key)
func (_EntityResolver *EntityResolverFilterer) FilterTextChanged(opts *bind.FilterOpts, node [][32]byte) (*EntityResolverTextChangedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.FilterLogs(opts, "TextChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return &EntityResolverTextChangedIterator{contract: _EntityResolver.contract, event: "TextChanged", logs: logs, sub: sub}, nil
}

// WatchTextChanged is a free log subscription operation binding the contract event 0xd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a7550.
//
// Solidity: event TextChanged(bytes32 indexed node, string indexedKey, string key)
func (_EntityResolver *EntityResolverFilterer) WatchTextChanged(opts *bind.WatchOpts, sink chan<- *EntityResolverTextChanged, node [][32]byte) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _EntityResolver.contract.WatchLogs(opts, "TextChanged", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EntityResolverTextChanged)
				if err := _EntityResolver.contract.UnpackLog(event, "TextChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTextChanged is a log parse operation binding the contract event 0xd8c9334b1a9c2f9da342a0a2b32629c1a229b6445dad78947f674b44444a7550.
//
// Solidity: event TextChanged(bytes32 indexed node, string indexedKey, string key)
func (_EntityResolver *EntityResolverFilterer) ParseTextChanged(log types.Log) (*EntityResolverTextChanged, error) {
	event := new(EntityResolverTextChanged)
	if err := _EntityResolver.contract.UnpackLog(event, "TextChanged", log); err != nil {
		return nil, err
	}
	return event, nil
}
