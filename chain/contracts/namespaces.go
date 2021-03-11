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

// NamespacesABI is the input ABI used to generate the binding from.
const NamespacesABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"namespace\",\"type\":\"uint32\"}],\"name\":\"NamespaceRegistered\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"namespaceCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"name\":\"namespaces\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"namespaceId\",\"type\":\"uint32\"}],\"name\":\"processContractAt\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"register\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"result\",\"type\":\"uint32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// NamespacesBin is the compiled bytecode used for deploying new contracts.
var NamespacesBin = "0x608060405234801561001057600080fd5b50610200806100206000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c80631aa3a008146100515780636d3005ae1461006f578063b40ec02b1461008f578063f631514c14610097575b600080fd5b6100596100aa565b60405161006691906101b9565b60405180910390f35b61008261007d36600461017a565b610132565b60405161006691906101a5565b610059610153565b6100826100a536600461017a565b61015f565b6001805463ffffffff19811663ffffffff918216830182161780835581166000908152602081905260408082208054336001600160a01b03199091161790559254925190927f6342a3b1a0f483c8ec694afd510f5f330e4792137228eb79e3e14458f78c57469261011d929116906101b9565b60405180910390a15060015463ffffffff1690565b63ffffffff166000908152602081905260409020546001600160a01b031690565b60015463ffffffff1681565b6000602081905290815260409020546001600160a01b031681565b60006020828403121561018b578081fd5b813563ffffffff8116811461019e578182fd5b9392505050565b6001600160a01b0391909116815260200190565b63ffffffff9190911681526020019056fea2646970667358221220eea4a428b03a3538f0e65a12ae2477c07f24884be2f91aa52eceabc59b954d8264736f6c634300060c0033"

// DeployNamespaces deploys a new Ethereum contract, binding an instance of Namespaces to it.
func DeployNamespaces(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Namespaces, error) {
	parsed, err := abi.JSON(strings.NewReader(NamespacesABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(NamespacesBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Namespaces{NamespacesCaller: NamespacesCaller{contract: contract}, NamespacesTransactor: NamespacesTransactor{contract: contract}, NamespacesFilterer: NamespacesFilterer{contract: contract}}, nil
}

// Namespaces is an auto generated Go binding around an Ethereum contract.
type Namespaces struct {
	NamespacesCaller     // Read-only binding to the contract
	NamespacesTransactor // Write-only binding to the contract
	NamespacesFilterer   // Log filterer for contract events
}

// NamespacesCaller is an auto generated read-only Go binding around an Ethereum contract.
type NamespacesCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NamespacesTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NamespacesTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NamespacesFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NamespacesFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NamespacesSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NamespacesSession struct {
	Contract     *Namespaces       // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NamespacesCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NamespacesCallerSession struct {
	Contract *NamespacesCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts     // Call options to use throughout this session
}

// NamespacesTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NamespacesTransactorSession struct {
	Contract     *NamespacesTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// NamespacesRaw is an auto generated low-level Go binding around an Ethereum contract.
type NamespacesRaw struct {
	Contract *Namespaces // Generic contract binding to access the raw methods on
}

// NamespacesCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NamespacesCallerRaw struct {
	Contract *NamespacesCaller // Generic read-only contract binding to access the raw methods on
}

// NamespacesTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NamespacesTransactorRaw struct {
	Contract *NamespacesTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNamespaces creates a new instance of Namespaces, bound to a specific deployed contract.
func NewNamespaces(address common.Address, backend bind.ContractBackend) (*Namespaces, error) {
	contract, err := bindNamespaces(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Namespaces{NamespacesCaller: NamespacesCaller{contract: contract}, NamespacesTransactor: NamespacesTransactor{contract: contract}, NamespacesFilterer: NamespacesFilterer{contract: contract}}, nil
}

// NewNamespacesCaller creates a new read-only instance of Namespaces, bound to a specific deployed contract.
func NewNamespacesCaller(address common.Address, caller bind.ContractCaller) (*NamespacesCaller, error) {
	contract, err := bindNamespaces(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NamespacesCaller{contract: contract}, nil
}

// NewNamespacesTransactor creates a new write-only instance of Namespaces, bound to a specific deployed contract.
func NewNamespacesTransactor(address common.Address, transactor bind.ContractTransactor) (*NamespacesTransactor, error) {
	contract, err := bindNamespaces(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NamespacesTransactor{contract: contract}, nil
}

// NewNamespacesFilterer creates a new log filterer instance of Namespaces, bound to a specific deployed contract.
func NewNamespacesFilterer(address common.Address, filterer bind.ContractFilterer) (*NamespacesFilterer, error) {
	contract, err := bindNamespaces(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NamespacesFilterer{contract: contract}, nil
}

// bindNamespaces binds a generic wrapper to an already deployed contract.
func bindNamespaces(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(NamespacesABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Namespaces *NamespacesRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Namespaces.Contract.NamespacesCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Namespaces *NamespacesRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Namespaces.Contract.NamespacesTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Namespaces *NamespacesRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Namespaces.Contract.NamespacesTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Namespaces *NamespacesCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Namespaces.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Namespaces *NamespacesTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Namespaces.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Namespaces *NamespacesTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Namespaces.Contract.contract.Transact(opts, method, params...)
}

// NamespaceCount is a free data retrieval call binding the contract method 0xb40ec02b.
//
// Solidity: function namespaceCount() view returns(uint32)
func (_Namespaces *NamespacesCaller) NamespaceCount(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Namespaces.contract.Call(opts, &out, "namespaceCount")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// NamespaceCount is a free data retrieval call binding the contract method 0xb40ec02b.
//
// Solidity: function namespaceCount() view returns(uint32)
func (_Namespaces *NamespacesSession) NamespaceCount() (uint32, error) {
	return _Namespaces.Contract.NamespaceCount(&_Namespaces.CallOpts)
}

// NamespaceCount is a free data retrieval call binding the contract method 0xb40ec02b.
//
// Solidity: function namespaceCount() view returns(uint32)
func (_Namespaces *NamespacesCallerSession) NamespaceCount() (uint32, error) {
	return _Namespaces.Contract.NamespaceCount(&_Namespaces.CallOpts)
}

// Namespaces is a free data retrieval call binding the contract method 0xf631514c.
//
// Solidity: function namespaces(uint32 ) view returns(address)
func (_Namespaces *NamespacesCaller) Namespaces(opts *bind.CallOpts, arg0 uint32) (common.Address, error) {
	var out []interface{}
	err := _Namespaces.contract.Call(opts, &out, "namespaces", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Namespaces is a free data retrieval call binding the contract method 0xf631514c.
//
// Solidity: function namespaces(uint32 ) view returns(address)
func (_Namespaces *NamespacesSession) Namespaces(arg0 uint32) (common.Address, error) {
	return _Namespaces.Contract.Namespaces(&_Namespaces.CallOpts, arg0)
}

// Namespaces is a free data retrieval call binding the contract method 0xf631514c.
//
// Solidity: function namespaces(uint32 ) view returns(address)
func (_Namespaces *NamespacesCallerSession) Namespaces(arg0 uint32) (common.Address, error) {
	return _Namespaces.Contract.Namespaces(&_Namespaces.CallOpts, arg0)
}

// ProcessContractAt is a free data retrieval call binding the contract method 0x6d3005ae.
//
// Solidity: function processContractAt(uint32 namespaceId) view returns(address)
func (_Namespaces *NamespacesCaller) ProcessContractAt(opts *bind.CallOpts, namespaceId uint32) (common.Address, error) {
	var out []interface{}
	err := _Namespaces.contract.Call(opts, &out, "processContractAt", namespaceId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ProcessContractAt is a free data retrieval call binding the contract method 0x6d3005ae.
//
// Solidity: function processContractAt(uint32 namespaceId) view returns(address)
func (_Namespaces *NamespacesSession) ProcessContractAt(namespaceId uint32) (common.Address, error) {
	return _Namespaces.Contract.ProcessContractAt(&_Namespaces.CallOpts, namespaceId)
}

// ProcessContractAt is a free data retrieval call binding the contract method 0x6d3005ae.
//
// Solidity: function processContractAt(uint32 namespaceId) view returns(address)
func (_Namespaces *NamespacesCallerSession) ProcessContractAt(namespaceId uint32) (common.Address, error) {
	return _Namespaces.Contract.ProcessContractAt(&_Namespaces.CallOpts, namespaceId)
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() returns(uint32 result)
func (_Namespaces *NamespacesTransactor) Register(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Namespaces.contract.Transact(opts, "register")
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() returns(uint32 result)
func (_Namespaces *NamespacesSession) Register() (*types.Transaction, error) {
	return _Namespaces.Contract.Register(&_Namespaces.TransactOpts)
}

// Register is a paid mutator transaction binding the contract method 0x1aa3a008.
//
// Solidity: function register() returns(uint32 result)
func (_Namespaces *NamespacesTransactorSession) Register() (*types.Transaction, error) {
	return _Namespaces.Contract.Register(&_Namespaces.TransactOpts)
}

// NamespacesNamespaceRegisteredIterator is returned from FilterNamespaceRegistered and is used to iterate over the raw logs and unpacked data for NamespaceRegistered events raised by the Namespaces contract.
type NamespacesNamespaceRegisteredIterator struct {
	Event *NamespacesNamespaceRegistered // Event containing the contract specifics and raw log

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
func (it *NamespacesNamespaceRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespacesNamespaceRegistered)
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
		it.Event = new(NamespacesNamespaceRegistered)
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
func (it *NamespacesNamespaceRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespacesNamespaceRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespacesNamespaceRegistered represents a NamespaceRegistered event raised by the Namespaces contract.
type NamespacesNamespaceRegistered struct {
	Namespace uint32
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNamespaceRegistered is a free log retrieval operation binding the contract event 0x6342a3b1a0f483c8ec694afd510f5f330e4792137228eb79e3e14458f78c5746.
//
// Solidity: event NamespaceRegistered(uint32 namespace)
func (_Namespaces *NamespacesFilterer) FilterNamespaceRegistered(opts *bind.FilterOpts) (*NamespacesNamespaceRegisteredIterator, error) {

	logs, sub, err := _Namespaces.contract.FilterLogs(opts, "NamespaceRegistered")
	if err != nil {
		return nil, err
	}
	return &NamespacesNamespaceRegisteredIterator{contract: _Namespaces.contract, event: "NamespaceRegistered", logs: logs, sub: sub}, nil
}

// WatchNamespaceRegistered is a free log subscription operation binding the contract event 0x6342a3b1a0f483c8ec694afd510f5f330e4792137228eb79e3e14458f78c5746.
//
// Solidity: event NamespaceRegistered(uint32 namespace)
func (_Namespaces *NamespacesFilterer) WatchNamespaceRegistered(opts *bind.WatchOpts, sink chan<- *NamespacesNamespaceRegistered) (event.Subscription, error) {

	logs, sub, err := _Namespaces.contract.WatchLogs(opts, "NamespaceRegistered")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespacesNamespaceRegistered)
				if err := _Namespaces.contract.UnpackLog(event, "NamespaceRegistered", log); err != nil {
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

// ParseNamespaceRegistered is a log parse operation binding the contract event 0x6342a3b1a0f483c8ec694afd510f5f330e4792137228eb79e3e14458f78c5746.
//
// Solidity: event NamespaceRegistered(uint32 namespace)
func (_Namespaces *NamespacesFilterer) ParseNamespaceRegistered(log types.Log) (*NamespacesNamespaceRegistered, error) {
	event := new(NamespacesNamespaceRegistered)
	if err := _Namespaces.contract.UnpackLog(event, "NamespaceRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
