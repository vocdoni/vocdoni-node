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

// ResultsABI is the input ABI used to generate the binding from.
const ResultsABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"genesisAddr\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"ResultsAvailable\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"genesisAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getResults\",\"outputs\":[{\"internalType\":\"uint32[][]\",\"name\":\"tally\",\"type\":\"uint32[][]\"},{\"internalType\":\"uint32\",\"name\":\"height\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"processesAddress\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"processesAddr\",\"type\":\"address\"}],\"name\":\"setProcessesAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"processId\",\"type\":\"bytes32\"},{\"internalType\":\"uint32[][]\",\"name\":\"tally\",\"type\":\"uint32[][]\"},{\"internalType\":\"uint32\",\"name\":\"height\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"vochainId\",\"type\":\"uint32\"}],\"name\":\"setResults\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// ResultsBin is the compiled bytecode used for deploying new contracts.
var ResultsBin = "0x608060405234801561001057600080fd5b50604051610bc7380380610bc783398101604081905261002f916100c8565b60008054336001600160a01b0319909116179055610057816100a1602090811b6104a117901c565b61007c5760405162461bcd60e51b8152600401610073906100f6565b60405180910390fd5b600180546001600160a01b0319166001600160a01b039290921691909117905561012d565b6000806001600160a01b0383166100bc5760009150506100c3565b5050803b15155b919050565b6000602082840312156100d9578081fd5b81516001600160a01b03811681146100ef578182fd5b9392505050565b60208082526013908201527f496e76616c69642067656e657369734164647200000000000000000000000000604082015260600190565b610a8b8061013c6000396000f3fe608060405234801561001057600080fd5b50600436106100575760003560e01c8063133baaa51461005c578063358aad291461007157806346475c4c146100845780636065fb33146100ae5780639d455ad6146100c3575b600080fd5b61006f61006a366004610660565b6100cb565b005b61006f61007f3660046106c6565b610145565b6100976100923660046106ae565b610366565b6040516100a592919061080e565b60405180910390f35b6100b6610483565b6040516100a591906107fa565b6100b6610492565b6000546001600160a01b031633146100fe5760405162461bcd60e51b81526004016100f5906108d0565b60405180910390fd5b610107816104a1565b6101235760405162461bcd60e51b81526004016100f59061091e565b600280546001600160a01b0319166001600160a01b0392909216919091179055565b6001546040516305525d6160e11b815282916001600160a01b031690630aa4bac29061017790849033906004016109ef565b60206040518083038186803b15801561018f57600080fd5b505afa1580156101a3573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906101c7919061068e565b6101e35760405162461bcd60e51b81526004016100f5906109cb565b6002546001600160a01b031661020b5760405162461bcd60e51b81526004016100f590610994565b600085815260036020526040902060010154640100000000900460ff16156102455760405162461bcd60e51b81526004016100f59061094d565b60008363ffffffff161161026b5760405162461bcd60e51b81526004016100f590610972565b6000858152600360209081526040909120855161028a928701906104c8565b5060008581526003602052604090819020600101805464010000000063ffffffff1990911663ffffffff87161764ff00000000191617905560025490516346f32a5d60e11b81526001600160a01b03909116908190638de654ba906102f590899060049081016108b2565b600060405180830381600087803b15801561030f57600080fd5b505af1158015610323573d6000803e3d6000fd5b505050507f5aff397e0d9bfad4e73dfd9c2da1d146ce7fe8cfd1a795dbf6b95b417236fa4c8660405161035691906108a9565b60405180910390a1505050505050565b60008181526003602052604081206001015460609190640100000000900460ff166103a35760405162461bcd60e51b81526004016100f5906108fb565b600083815260036020908152604080832060018101548154835181860281018601909452808452919463ffffffff909116938592919084015b828210156104745760008481526020908190208301805460408051828502810185019091528181529283018282801561046057602002820191906000526020600020906000905b82829054906101000a900463ffffffff1663ffffffff16815260200190600401906020826003010492830192600103820291508084116104235790505b5050505050815260200190600101906103dc565b50505050915091509150915091565b6001546001600160a01b031681565b6002546001600160a01b031681565b6000806001600160a01b0383166104bc5760009150506104c3565b5050803b15155b919050565b828054828255906000526020600020908101928215610515579160200282015b828111156105155782518051610505918491602090910190610525565b50916020019190600101906104e8565b506105219291506105d0565b5090565b828054828255906000526020600020906007016008900481019282156105c45791602002820160005b8382111561059257835183826101000a81548163ffffffff021916908363ffffffff160217905550926020019260040160208160030104928301926001030261054e565b80156105c25782816101000a81549063ffffffff0219169055600401602081600301049283019260010302610592565b505b506105219291506105ed565b808211156105215760006105e48282610609565b506001016105d0565b5b8082111561052157805463ffffffff191681556001016105ee565b50805460008255600701600890049060005260206000209081019061062e9190610631565b50565b5b808211156105215760008155600101610632565b803563ffffffff8116811461065a57600080fd5b92915050565b600060208284031215610671578081fd5b81356001600160a01b0381168114610687578182fd5b9392505050565b60006020828403121561069f578081fd5b81518015158114610687578182fd5b6000602082840312156106bf578081fd5b5035919050565b600080600080608085870312156106db578283fd5b8435935067ffffffffffffffff602086013511156106f7578283fd5b6020850135850186601f82011261070c578384fd5b61071e6107198235610a35565b610a0e565b81358152602080820191908301865b84358110156107cb578a603f833587010112610747578788fd5b61075a6107196020843588010135610a35565b8235860160208181013580845281840193926040808201939290920201018e1015610783578a8bfd5b8a5b602086358a0101358110156107b35761079e8f83610646565b84526020938401939190910190600101610785565b5050855250602093840193919091019060010161072d565b50508095505050506107e08660408701610646565b91506107ef8660608701610646565b905092959194509250565b6001600160a01b0391909116815260200190565b60006040820160408352808551808352606085019150602092506060838202860101838801855b8381101561088e57878303605f19018552815180518085529087019087850190895b8181101561087957835163ffffffff1683529289019291890191600101610857565b50509587019593505090850190600101610835565b505080945050505063ffffffff841681840152509392505050565b90815260200190565b82815260408101600583106108c357fe5b8260208301529392505050565b60208082526011908201527037b7363ca1b7b73a3930b1ba27bbb732b960791b604082015260600190565b602080825260099082015268139bdd08199bdd5b9960ba1b604082015260600190565b60208082526015908201527424b73b30b634b210383937b1b2b9b9b2b9a0b2323960591b604082015260600190565b6020808252600b908201526a105b1c9958591e481cd95d60aa1b604082015260600190565b6020808252600890820152674e6f20766f74657360c01b604082015260600190565b60208082526018908201527f496e76616c69642070726f636573736573416464726573730000000000000000604082015260600190565b6020808252600a90820152694e6f74206f7261636c6560b01b604082015260600190565b63ffffffff9290921682526001600160a01b0316602082015260400190565b60405181810167ffffffffffffffff81118282101715610a2d57600080fd5b604052919050565b600067ffffffffffffffff821115610a4b578081fd5b506020908102019056fea264697066735822122015289d76761d52e418ff30f081aec30245bfc88245991d0eb14af6f79640cd6764736f6c634300060c0033"

// DeployResults deploys a new Ethereum contract, binding an instance of Results to it.
func DeployResults(auth *bind.TransactOpts, backend bind.ContractBackend, genesisAddr common.Address) (common.Address, *types.Transaction, *Results, error) {
	parsed, err := abi.JSON(strings.NewReader(ResultsABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(ResultsBin), backend, genesisAddr)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Results{ResultsCaller: ResultsCaller{contract: contract}, ResultsTransactor: ResultsTransactor{contract: contract}, ResultsFilterer: ResultsFilterer{contract: contract}}, nil
}

// Results is an auto generated Go binding around an Ethereum contract.
type Results struct {
	ResultsCaller     // Read-only binding to the contract
	ResultsTransactor // Write-only binding to the contract
	ResultsFilterer   // Log filterer for contract events
}

// ResultsCaller is an auto generated read-only Go binding around an Ethereum contract.
type ResultsCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ResultsTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ResultsTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ResultsFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ResultsFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ResultsSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ResultsSession struct {
	Contract     *Results          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ResultsCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ResultsCallerSession struct {
	Contract *ResultsCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// ResultsTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ResultsTransactorSession struct {
	Contract     *ResultsTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// ResultsRaw is an auto generated low-level Go binding around an Ethereum contract.
type ResultsRaw struct {
	Contract *Results // Generic contract binding to access the raw methods on
}

// ResultsCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ResultsCallerRaw struct {
	Contract *ResultsCaller // Generic read-only contract binding to access the raw methods on
}

// ResultsTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ResultsTransactorRaw struct {
	Contract *ResultsTransactor // Generic write-only contract binding to access the raw methods on
}

// NewResults creates a new instance of Results, bound to a specific deployed contract.
func NewResults(address common.Address, backend bind.ContractBackend) (*Results, error) {
	contract, err := bindResults(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Results{ResultsCaller: ResultsCaller{contract: contract}, ResultsTransactor: ResultsTransactor{contract: contract}, ResultsFilterer: ResultsFilterer{contract: contract}}, nil
}

// NewResultsCaller creates a new read-only instance of Results, bound to a specific deployed contract.
func NewResultsCaller(address common.Address, caller bind.ContractCaller) (*ResultsCaller, error) {
	contract, err := bindResults(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ResultsCaller{contract: contract}, nil
}

// NewResultsTransactor creates a new write-only instance of Results, bound to a specific deployed contract.
func NewResultsTransactor(address common.Address, transactor bind.ContractTransactor) (*ResultsTransactor, error) {
	contract, err := bindResults(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ResultsTransactor{contract: contract}, nil
}

// NewResultsFilterer creates a new log filterer instance of Results, bound to a specific deployed contract.
func NewResultsFilterer(address common.Address, filterer bind.ContractFilterer) (*ResultsFilterer, error) {
	contract, err := bindResults(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ResultsFilterer{contract: contract}, nil
}

// bindResults binds a generic wrapper to an already deployed contract.
func bindResults(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ResultsABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Results *ResultsRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Results.Contract.ResultsCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Results *ResultsRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Results.Contract.ResultsTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Results *ResultsRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Results.Contract.ResultsTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Results *ResultsCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Results.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Results *ResultsTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Results.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Results *ResultsTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Results.Contract.contract.Transact(opts, method, params...)
}

// GenesisAddress is a free data retrieval call binding the contract method 0x6065fb33.
//
// Solidity: function genesisAddress() view returns(address)
func (_Results *ResultsCaller) GenesisAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Results.contract.Call(opts, &out, "genesisAddress")
	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err
}

// GenesisAddress is a free data retrieval call binding the contract method 0x6065fb33.
//
// Solidity: function genesisAddress() view returns(address)
func (_Results *ResultsSession) GenesisAddress() (common.Address, error) {
	return _Results.Contract.GenesisAddress(&_Results.CallOpts)
}

// GenesisAddress is a free data retrieval call binding the contract method 0x6065fb33.
//
// Solidity: function genesisAddress() view returns(address)
func (_Results *ResultsCallerSession) GenesisAddress() (common.Address, error) {
	return _Results.Contract.GenesisAddress(&_Results.CallOpts)
}

// GetResults is a free data retrieval call binding the contract method 0x46475c4c.
//
// Solidity: function getResults(bytes32 processId) view returns(uint32[][] tally, uint32 height)
func (_Results *ResultsCaller) GetResults(opts *bind.CallOpts, processId [32]byte) (struct {
	Tally  [][]uint32
	Height uint32
}, error) {
	var out []interface{}
	err := _Results.contract.Call(opts, &out, "getResults", processId)

	outstruct := new(struct {
		Tally  [][]uint32
		Height uint32
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Tally = *abi.ConvertType(out[0], new([][]uint32)).(*[][]uint32)
	outstruct.Height = *abi.ConvertType(out[1], new(uint32)).(*uint32)

	return *outstruct, err
}

// GetResults is a free data retrieval call binding the contract method 0x46475c4c.
//
// Solidity: function getResults(bytes32 processId) view returns(uint32[][] tally, uint32 height)
func (_Results *ResultsSession) GetResults(processId [32]byte) (struct {
	Tally  [][]uint32
	Height uint32
}, error) {
	return _Results.Contract.GetResults(&_Results.CallOpts, processId)
}

// GetResults is a free data retrieval call binding the contract method 0x46475c4c.
//
// Solidity: function getResults(bytes32 processId) view returns(uint32[][] tally, uint32 height)
func (_Results *ResultsCallerSession) GetResults(processId [32]byte) (struct {
	Tally  [][]uint32
	Height uint32
}, error) {
	return _Results.Contract.GetResults(&_Results.CallOpts, processId)
}

// ProcessesAddress is a free data retrieval call binding the contract method 0x9d455ad6.
//
// Solidity: function processesAddress() view returns(address)
func (_Results *ResultsCaller) ProcessesAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Results.contract.Call(opts, &out, "processesAddress")
	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err
}

// ProcessesAddress is a free data retrieval call binding the contract method 0x9d455ad6.
//
// Solidity: function processesAddress() view returns(address)
func (_Results *ResultsSession) ProcessesAddress() (common.Address, error) {
	return _Results.Contract.ProcessesAddress(&_Results.CallOpts)
}

// ProcessesAddress is a free data retrieval call binding the contract method 0x9d455ad6.
//
// Solidity: function processesAddress() view returns(address)
func (_Results *ResultsCallerSession) ProcessesAddress() (common.Address, error) {
	return _Results.Contract.ProcessesAddress(&_Results.CallOpts)
}

// SetProcessesAddress is a paid mutator transaction binding the contract method 0x133baaa5.
//
// Solidity: function setProcessesAddress(address processesAddr) returns()
func (_Results *ResultsTransactor) SetProcessesAddress(opts *bind.TransactOpts, processesAddr common.Address) (*types.Transaction, error) {
	return _Results.contract.Transact(opts, "setProcessesAddress", processesAddr)
}

// SetProcessesAddress is a paid mutator transaction binding the contract method 0x133baaa5.
//
// Solidity: function setProcessesAddress(address processesAddr) returns()
func (_Results *ResultsSession) SetProcessesAddress(processesAddr common.Address) (*types.Transaction, error) {
	return _Results.Contract.SetProcessesAddress(&_Results.TransactOpts, processesAddr)
}

// SetProcessesAddress is a paid mutator transaction binding the contract method 0x133baaa5.
//
// Solidity: function setProcessesAddress(address processesAddr) returns()
func (_Results *ResultsTransactorSession) SetProcessesAddress(processesAddr common.Address) (*types.Transaction, error) {
	return _Results.Contract.SetProcessesAddress(&_Results.TransactOpts, processesAddr)
}

// SetResults is a paid mutator transaction binding the contract method 0x358aad29.
//
// Solidity: function setResults(bytes32 processId, uint32[][] tally, uint32 height, uint32 vochainId) returns()
func (_Results *ResultsTransactor) SetResults(opts *bind.TransactOpts, processId [32]byte, tally [][]uint32, height uint32, vochainId uint32) (*types.Transaction, error) {
	return _Results.contract.Transact(opts, "setResults", processId, tally, height, vochainId)
}

// SetResults is a paid mutator transaction binding the contract method 0x358aad29.
//
// Solidity: function setResults(bytes32 processId, uint32[][] tally, uint32 height, uint32 vochainId) returns()
func (_Results *ResultsSession) SetResults(processId [32]byte, tally [][]uint32, height uint32, vochainId uint32) (*types.Transaction, error) {
	return _Results.Contract.SetResults(&_Results.TransactOpts, processId, tally, height, vochainId)
}

// SetResults is a paid mutator transaction binding the contract method 0x358aad29.
//
// Solidity: function setResults(bytes32 processId, uint32[][] tally, uint32 height, uint32 vochainId) returns()
func (_Results *ResultsTransactorSession) SetResults(processId [32]byte, tally [][]uint32, height uint32, vochainId uint32) (*types.Transaction, error) {
	return _Results.Contract.SetResults(&_Results.TransactOpts, processId, tally, height, vochainId)
}

// ResultsResultsAvailableIterator is returned from FilterResultsAvailable and is used to iterate over the raw logs and unpacked data for ResultsAvailable events raised by the Results contract.
type ResultsResultsAvailableIterator struct {
	Event *ResultsResultsAvailable // Event containing the contract specifics and raw log

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
func (it *ResultsResultsAvailableIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ResultsResultsAvailable)
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
		it.Event = new(ResultsResultsAvailable)
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
func (it *ResultsResultsAvailableIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ResultsResultsAvailableIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ResultsResultsAvailable represents a ResultsAvailable event raised by the Results contract.
type ResultsResultsAvailable struct {
	ProcessId [32]byte
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterResultsAvailable is a free log retrieval operation binding the contract event 0x5aff397e0d9bfad4e73dfd9c2da1d146ce7fe8cfd1a795dbf6b95b417236fa4c.
//
// Solidity: event ResultsAvailable(bytes32 processId)
func (_Results *ResultsFilterer) FilterResultsAvailable(opts *bind.FilterOpts) (*ResultsResultsAvailableIterator, error) {
	logs, sub, err := _Results.contract.FilterLogs(opts, "ResultsAvailable")
	if err != nil {
		return nil, err
	}
	return &ResultsResultsAvailableIterator{contract: _Results.contract, event: "ResultsAvailable", logs: logs, sub: sub}, nil
}

// WatchResultsAvailable is a free log subscription operation binding the contract event 0x5aff397e0d9bfad4e73dfd9c2da1d146ce7fe8cfd1a795dbf6b95b417236fa4c.
//
// Solidity: event ResultsAvailable(bytes32 processId)
func (_Results *ResultsFilterer) WatchResultsAvailable(opts *bind.WatchOpts, sink chan<- *ResultsResultsAvailable) (event.Subscription, error) {
	logs, sub, err := _Results.contract.WatchLogs(opts, "ResultsAvailable")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ResultsResultsAvailable)
				if err := _Results.contract.UnpackLog(event, "ResultsAvailable", log); err != nil {
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

// ParseResultsAvailable is a log parse operation binding the contract event 0x5aff397e0d9bfad4e73dfd9c2da1d146ce7fe8cfd1a795dbf6b95b417236fa4c.
//
// Solidity: event ResultsAvailable(bytes32 processId)
func (_Results *ResultsFilterer) ParseResultsAvailable(log types.Log) (*ResultsResultsAvailable, error) {
	event := new(ResultsResultsAvailable)
	if err := _Results.contract.UnpackLog(event, "ResultsAvailable", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
