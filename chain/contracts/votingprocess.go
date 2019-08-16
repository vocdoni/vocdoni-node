// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package votingProcess

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
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// VotingProcessABI is the input ABI used to generate the binding from.
const VotingProcessABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getPrivateKey\",\"outputs\":[{\"name\":\"privateKey\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getGenesis\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getChainId\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getOracles\",\"outputs\":[{\"name\":\"\",\"type\":\"string[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getResults\",\"outputs\":[{\"name\":\"resultsContentHashedUri\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newValue\",\"type\":\"string\"}],\"name\":\"setGenesis\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"processes\",\"outputs\":[{\"name\":\"entityAddress\",\"type\":\"address\"},{\"name\":\"metadataContentHashedUri\",\"type\":\"string\"},{\"name\":\"voteEncryptionPrivateKey\",\"type\":\"string\"},{\"name\":\"canceled\",\"type\":\"bool\"},{\"name\":\"resultsContentHashedUri\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"entityAddress\",\"type\":\"address\"}],\"name\":\"getNextProcessId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"str1\",\"type\":\"string\"},{\"name\":\"str2\",\"type\":\"string\"}],\"name\":\"equalStrings\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"entityProcessCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\"},{\"name\":\"oraclePublicKey\",\"type\":\"string\"}],\"name\":\"removeOracle\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\"},{\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"removeValidator\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"get\",\"outputs\":[{\"name\":\"entityAddress\",\"type\":\"address\"},{\"name\":\"metadataContentHashedUri\",\"type\":\"string\"},{\"name\":\"voteEncryptionPrivateKey\",\"type\":\"string\"},{\"name\":\"canceled\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"privateKey\",\"type\":\"string\"}],\"name\":\"publishPrivateKey\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"addValidator\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"metadataContentHashedUri\",\"type\":\"string\"}],\"name\":\"create\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getValidators\",\"outputs\":[{\"name\":\"\",\"type\":\"string[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"cancel\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getProcessIndex\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"oraclePublicKey\",\"type\":\"string\"}],\"name\":\"addOracle\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"resultsContentHashedUri\",\"type\":\"string\"}],\"name\":\"publishResults\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"newValue\",\"type\":\"uint256\"}],\"name\":\"setChainId\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"entityAddress\",\"type\":\"address\"}],\"name\":\"getEntityProcessCount\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"entityAddress\",\"type\":\"address\"},{\"name\":\"processCountIndex\",\"type\":\"uint256\"}],\"name\":\"getProcessId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"chainIdValue\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"genesis\",\"type\":\"string\"}],\"name\":\"GenesisChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"chainId\",\"type\":\"uint256\"}],\"name\":\"ChainIdChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"entityAddress\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"ProcessCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"entityAddress\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"ProcessCanceled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"ValidatorAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"ValidatorRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"oraclePublicKey\",\"type\":\"string\"}],\"name\":\"OracleAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"oraclePublicKey\",\"type\":\"string\"}],\"name\":\"OracleRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"processId\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"privateKey\",\"type\":\"string\"}],\"name\":\"PrivateKeyPublished\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"processId\",\"type\":\"bytes32\"},{\"indexed\":false,\"name\":\"results\",\"type\":\"string\"}],\"name\":\"ResultsPublished\",\"type\":\"event\"}]"

// VotingProcess is an auto generated Go binding around an Ethereum contract.
type VotingProcess struct {
	VotingProcessCaller     // Read-only binding to the contract
	VotingProcessTransactor // Write-only binding to the contract
	VotingProcessFilterer   // Log filterer for contract events
}

// VotingProcessCaller is an auto generated read-only Go binding around an Ethereum contract.
type VotingProcessCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VotingProcessTransactor is an auto generated write-only Go binding around an Ethereum contract.
type VotingProcessTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VotingProcessFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type VotingProcessFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// VotingProcessSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type VotingProcessSession struct {
	Contract     *VotingProcess    // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// VotingProcessCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type VotingProcessCallerSession struct {
	Contract *VotingProcessCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts        // Call options to use throughout this session
}

// VotingProcessTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type VotingProcessTransactorSession struct {
	Contract     *VotingProcessTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts        // Transaction auth options to use throughout this session
}

// VotingProcessRaw is an auto generated low-level Go binding around an Ethereum contract.
type VotingProcessRaw struct {
	Contract *VotingProcess // Generic contract binding to access the raw methods on
}

// VotingProcessCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type VotingProcessCallerRaw struct {
	Contract *VotingProcessCaller // Generic read-only contract binding to access the raw methods on
}

// VotingProcessTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type VotingProcessTransactorRaw struct {
	Contract *VotingProcessTransactor // Generic write-only contract binding to access the raw methods on
}

// NewVotingProcess creates a new instance of VotingProcess, bound to a specific deployed contract.
func NewVotingProcess(address common.Address, backend bind.ContractBackend) (*VotingProcess, error) {
	contract, err := bindVotingProcess(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &VotingProcess{VotingProcessCaller: VotingProcessCaller{contract: contract}, VotingProcessTransactor: VotingProcessTransactor{contract: contract}, VotingProcessFilterer: VotingProcessFilterer{contract: contract}}, nil
}

// NewVotingProcessCaller creates a new read-only instance of VotingProcess, bound to a specific deployed contract.
func NewVotingProcessCaller(address common.Address, caller bind.ContractCaller) (*VotingProcessCaller, error) {
	contract, err := bindVotingProcess(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &VotingProcessCaller{contract: contract}, nil
}

// NewVotingProcessTransactor creates a new write-only instance of VotingProcess, bound to a specific deployed contract.
func NewVotingProcessTransactor(address common.Address, transactor bind.ContractTransactor) (*VotingProcessTransactor, error) {
	contract, err := bindVotingProcess(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &VotingProcessTransactor{contract: contract}, nil
}

// NewVotingProcessFilterer creates a new log filterer instance of VotingProcess, bound to a specific deployed contract.
func NewVotingProcessFilterer(address common.Address, filterer bind.ContractFilterer) (*VotingProcessFilterer, error) {
	contract, err := bindVotingProcess(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &VotingProcessFilterer{contract: contract}, nil
}

// bindVotingProcess binds a generic wrapper to an already deployed contract.
func bindVotingProcess(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(VotingProcessABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_VotingProcess *VotingProcessRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _VotingProcess.Contract.VotingProcessCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_VotingProcess *VotingProcessRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _VotingProcess.Contract.VotingProcessTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_VotingProcess *VotingProcessRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _VotingProcess.Contract.VotingProcessTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_VotingProcess *VotingProcessCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _VotingProcess.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_VotingProcess *VotingProcessTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _VotingProcess.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_VotingProcess *VotingProcessTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _VotingProcess.Contract.contract.Transact(opts, method, params...)
}

// EntityProcessCount is a free data retrieval call binding the contract method 0x7c09faeb.
//
// Solidity: function entityProcessCount(address ) constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) EntityProcessCount(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "entityProcessCount", arg0)
	return *ret0, err
}

// EntityProcessCount is a free data retrieval call binding the contract method 0x7c09faeb.
//
// Solidity: function entityProcessCount(address ) constant returns(uint256)
func (_VotingProcess *VotingProcessSession) EntityProcessCount(arg0 common.Address) (*big.Int, error) {
	return _VotingProcess.Contract.EntityProcessCount(&_VotingProcess.CallOpts, arg0)
}

// EntityProcessCount is a free data retrieval call binding the contract method 0x7c09faeb.
//
// Solidity: function entityProcessCount(address ) constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) EntityProcessCount(arg0 common.Address) (*big.Int, error) {
	return _VotingProcess.Contract.EntityProcessCount(&_VotingProcess.CallOpts, arg0)
}

// EqualStrings is a free data retrieval call binding the contract method 0x791f0333.
//
// Solidity: function equalStrings(string str1, string str2) constant returns(bool)
func (_VotingProcess *VotingProcessCaller) EqualStrings(opts *bind.CallOpts, str1 string, str2 string) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "equalStrings", str1, str2)
	return *ret0, err
}

// EqualStrings is a free data retrieval call binding the contract method 0x791f0333.
//
// Solidity: function equalStrings(string str1, string str2) constant returns(bool)
func (_VotingProcess *VotingProcessSession) EqualStrings(str1 string, str2 string) (bool, error) {
	return _VotingProcess.Contract.EqualStrings(&_VotingProcess.CallOpts, str1, str2)
}

// EqualStrings is a free data retrieval call binding the contract method 0x791f0333.
//
// Solidity: function equalStrings(string str1, string str2) constant returns(bool)
func (_VotingProcess *VotingProcessCallerSession) EqualStrings(str1 string, str2 string) (bool, error) {
	return _VotingProcess.Contract.EqualStrings(&_VotingProcess.CallOpts, str1, str2)
}

// Get is a free data retrieval call binding the contract method 0x8eaa6ac0.
//
// Solidity: function get(bytes32 processId) constant returns(address entityAddress, string metadataContentHashedUri, string voteEncryptionPrivateKey, bool canceled)
func (_VotingProcess *VotingProcessCaller) Get(opts *bind.CallOpts, processId [32]byte) (struct {
	EntityAddress            common.Address
	MetadataContentHashedUri string
	VoteEncryptionPrivateKey string
	Canceled                 bool
}, error) {
	ret := new(struct {
		EntityAddress            common.Address
		MetadataContentHashedUri string
		VoteEncryptionPrivateKey string
		Canceled                 bool
	})
	out := ret
	err := _VotingProcess.contract.Call(opts, out, "get", processId)
	return *ret, err
}

// Get is a free data retrieval call binding the contract method 0x8eaa6ac0.
//
// Solidity: function get(bytes32 processId) constant returns(address entityAddress, string metadataContentHashedUri, string voteEncryptionPrivateKey, bool canceled)
func (_VotingProcess *VotingProcessSession) Get(processId [32]byte) (struct {
	EntityAddress            common.Address
	MetadataContentHashedUri string
	VoteEncryptionPrivateKey string
	Canceled                 bool
}, error) {
	return _VotingProcess.Contract.Get(&_VotingProcess.CallOpts, processId)
}

// Get is a free data retrieval call binding the contract method 0x8eaa6ac0.
//
// Solidity: function get(bytes32 processId) constant returns(address entityAddress, string metadataContentHashedUri, string voteEncryptionPrivateKey, bool canceled)
func (_VotingProcess *VotingProcessCallerSession) Get(processId [32]byte) (struct {
	EntityAddress            common.Address
	MetadataContentHashedUri string
	VoteEncryptionPrivateKey string
	Canceled                 bool
}, error) {
	return _VotingProcess.Contract.Get(&_VotingProcess.CallOpts, processId)
}

// GetChainId is a free data retrieval call binding the contract method 0x3408e470.
//
// Solidity: function getChainId() constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) GetChainId(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getChainId")
	return *ret0, err
}

// GetChainId is a free data retrieval call binding the contract method 0x3408e470.
//
// Solidity: function getChainId() constant returns(uint256)
func (_VotingProcess *VotingProcessSession) GetChainId() (*big.Int, error) {
	return _VotingProcess.Contract.GetChainId(&_VotingProcess.CallOpts)
}

// GetChainId is a free data retrieval call binding the contract method 0x3408e470.
//
// Solidity: function getChainId() constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) GetChainId() (*big.Int, error) {
	return _VotingProcess.Contract.GetChainId(&_VotingProcess.CallOpts)
}

// GetEntityProcessCount is a free data retrieval call binding the contract method 0xf2bcb15e.
//
// Solidity: function getEntityProcessCount(address entityAddress) constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) GetEntityProcessCount(opts *bind.CallOpts, entityAddress common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getEntityProcessCount", entityAddress)
	return *ret0, err
}

// GetEntityProcessCount is a free data retrieval call binding the contract method 0xf2bcb15e.
//
// Solidity: function getEntityProcessCount(address entityAddress) constant returns(uint256)
func (_VotingProcess *VotingProcessSession) GetEntityProcessCount(entityAddress common.Address) (*big.Int, error) {
	return _VotingProcess.Contract.GetEntityProcessCount(&_VotingProcess.CallOpts, entityAddress)
}

// GetEntityProcessCount is a free data retrieval call binding the contract method 0xf2bcb15e.
//
// Solidity: function getEntityProcessCount(address entityAddress) constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) GetEntityProcessCount(entityAddress common.Address) (*big.Int, error) {
	return _VotingProcess.Contract.GetEntityProcessCount(&_VotingProcess.CallOpts, entityAddress)
}

// GetGenesis is a free data retrieval call binding the contract method 0x1a43bcb5.
//
// Solidity: function getGenesis() constant returns(string)
func (_VotingProcess *VotingProcessCaller) GetGenesis(opts *bind.CallOpts) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getGenesis")
	return *ret0, err
}

// GetGenesis is a free data retrieval call binding the contract method 0x1a43bcb5.
//
// Solidity: function getGenesis() constant returns(string)
func (_VotingProcess *VotingProcessSession) GetGenesis() (string, error) {
	return _VotingProcess.Contract.GetGenesis(&_VotingProcess.CallOpts)
}

// GetGenesis is a free data retrieval call binding the contract method 0x1a43bcb5.
//
// Solidity: function getGenesis() constant returns(string)
func (_VotingProcess *VotingProcessCallerSession) GetGenesis() (string, error) {
	return _VotingProcess.Contract.GetGenesis(&_VotingProcess.CallOpts)
}

// GetNextProcessId is a free data retrieval call binding the contract method 0x68141f2c.
//
// Solidity: function getNextProcessId(address entityAddress) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) GetNextProcessId(opts *bind.CallOpts, entityAddress common.Address) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getNextProcessId", entityAddress)
	return *ret0, err
}

// GetNextProcessId is a free data retrieval call binding the contract method 0x68141f2c.
//
// Solidity: function getNextProcessId(address entityAddress) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) GetNextProcessId(entityAddress common.Address) ([32]byte, error) {
	return _VotingProcess.Contract.GetNextProcessId(&_VotingProcess.CallOpts, entityAddress)
}

// GetNextProcessId is a free data retrieval call binding the contract method 0x68141f2c.
//
// Solidity: function getNextProcessId(address entityAddress) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) GetNextProcessId(entityAddress common.Address) ([32]byte, error) {
	return _VotingProcess.Contract.GetNextProcessId(&_VotingProcess.CallOpts, entityAddress)
}

// GetOracles is a free data retrieval call binding the contract method 0x40884c52.
//
// Solidity: function getOracles() constant returns(string[])
func (_VotingProcess *VotingProcessCaller) GetOracles(opts *bind.CallOpts) ([]string, error) {
	var (
		ret0 = new([]string)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getOracles")
	return *ret0, err
}

// GetOracles is a free data retrieval call binding the contract method 0x40884c52.
//
// Solidity: function getOracles() constant returns(string[])
func (_VotingProcess *VotingProcessSession) GetOracles() ([]string, error) {
	return _VotingProcess.Contract.GetOracles(&_VotingProcess.CallOpts)
}

// GetOracles is a free data retrieval call binding the contract method 0x40884c52.
//
// Solidity: function getOracles() constant returns(string[])
func (_VotingProcess *VotingProcessCallerSession) GetOracles() ([]string, error) {
	return _VotingProcess.Contract.GetOracles(&_VotingProcess.CallOpts)
}

// GetPrivateKey is a free data retrieval call binding the contract method 0x18208792.
//
// Solidity: function getPrivateKey(bytes32 processId) constant returns(string privateKey)
func (_VotingProcess *VotingProcessCaller) GetPrivateKey(opts *bind.CallOpts, processId [32]byte) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getPrivateKey", processId)
	return *ret0, err
}

// GetPrivateKey is a free data retrieval call binding the contract method 0x18208792.
//
// Solidity: function getPrivateKey(bytes32 processId) constant returns(string privateKey)
func (_VotingProcess *VotingProcessSession) GetPrivateKey(processId [32]byte) (string, error) {
	return _VotingProcess.Contract.GetPrivateKey(&_VotingProcess.CallOpts, processId)
}

// GetPrivateKey is a free data retrieval call binding the contract method 0x18208792.
//
// Solidity: function getPrivateKey(bytes32 processId) constant returns(string privateKey)
func (_VotingProcess *VotingProcessCallerSession) GetPrivateKey(processId [32]byte) (string, error) {
	return _VotingProcess.Contract.GetPrivateKey(&_VotingProcess.CallOpts, processId)
}

// GetProcessId is a free data retrieval call binding the contract method 0xf3b86c99.
//
// Solidity: function getProcessId(address entityAddress, uint256 processCountIndex) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) GetProcessId(opts *bind.CallOpts, entityAddress common.Address, processCountIndex *big.Int) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getProcessId", entityAddress, processCountIndex)
	return *ret0, err
}

// GetProcessId is a free data retrieval call binding the contract method 0xf3b86c99.
//
// Solidity: function getProcessId(address entityAddress, uint256 processCountIndex) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) GetProcessId(entityAddress common.Address, processCountIndex *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.GetProcessId(&_VotingProcess.CallOpts, entityAddress, processCountIndex)
}

// GetProcessId is a free data retrieval call binding the contract method 0xf3b86c99.
//
// Solidity: function getProcessId(address entityAddress, uint256 processCountIndex) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) GetProcessId(entityAddress common.Address, processCountIndex *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.GetProcessId(&_VotingProcess.CallOpts, entityAddress, processCountIndex)
}

// GetProcessIndex is a free data retrieval call binding the contract method 0xc585ad91.
//
// Solidity: function getProcessIndex(bytes32 processId) constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) GetProcessIndex(opts *bind.CallOpts, processId [32]byte) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getProcessIndex", processId)
	return *ret0, err
}

// GetProcessIndex is a free data retrieval call binding the contract method 0xc585ad91.
//
// Solidity: function getProcessIndex(bytes32 processId) constant returns(uint256)
func (_VotingProcess *VotingProcessSession) GetProcessIndex(processId [32]byte) (*big.Int, error) {
	return _VotingProcess.Contract.GetProcessIndex(&_VotingProcess.CallOpts, processId)
}

// GetProcessIndex is a free data retrieval call binding the contract method 0xc585ad91.
//
// Solidity: function getProcessIndex(bytes32 processId) constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) GetProcessIndex(processId [32]byte) (*big.Int, error) {
	return _VotingProcess.Contract.GetProcessIndex(&_VotingProcess.CallOpts, processId)
}

// GetResults is a free data retrieval call binding the contract method 0x46475c4c.
//
// Solidity: function getResults(bytes32 processId) constant returns(string resultsContentHashedUri)
func (_VotingProcess *VotingProcessCaller) GetResults(opts *bind.CallOpts, processId [32]byte) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getResults", processId)
	return *ret0, err
}

// GetResults is a free data retrieval call binding the contract method 0x46475c4c.
//
// Solidity: function getResults(bytes32 processId) constant returns(string resultsContentHashedUri)
func (_VotingProcess *VotingProcessSession) GetResults(processId [32]byte) (string, error) {
	return _VotingProcess.Contract.GetResults(&_VotingProcess.CallOpts, processId)
}

// GetResults is a free data retrieval call binding the contract method 0x46475c4c.
//
// Solidity: function getResults(bytes32 processId) constant returns(string resultsContentHashedUri)
func (_VotingProcess *VotingProcessCallerSession) GetResults(processId [32]byte) (string, error) {
	return _VotingProcess.Contract.GetResults(&_VotingProcess.CallOpts, processId)
}

// GetValidators is a free data retrieval call binding the contract method 0xb7ab4db5.
//
// Solidity: function getValidators() constant returns(string[])
func (_VotingProcess *VotingProcessCaller) GetValidators(opts *bind.CallOpts) ([]string, error) {
	var (
		ret0 = new([]string)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getValidators")
	return *ret0, err
}

// GetValidators is a free data retrieval call binding the contract method 0xb7ab4db5.
//
// Solidity: function getValidators() constant returns(string[])
func (_VotingProcess *VotingProcessSession) GetValidators() ([]string, error) {
	return _VotingProcess.Contract.GetValidators(&_VotingProcess.CallOpts)
}

// GetValidators is a free data retrieval call binding the contract method 0xb7ab4db5.
//
// Solidity: function getValidators() constant returns(string[])
func (_VotingProcess *VotingProcessCallerSession) GetValidators() ([]string, error) {
	return _VotingProcess.Contract.GetValidators(&_VotingProcess.CallOpts)
}

// Processes is a free data retrieval call binding the contract method 0x579e51c7.
//
// Solidity: function processes(uint256 ) constant returns(address entityAddress, string metadataContentHashedUri, string voteEncryptionPrivateKey, bool canceled, string resultsContentHashedUri)
func (_VotingProcess *VotingProcessCaller) Processes(opts *bind.CallOpts, arg0 *big.Int) (struct {
	EntityAddress            common.Address
	MetadataContentHashedUri string
	VoteEncryptionPrivateKey string
	Canceled                 bool
	ResultsContentHashedUri  string
}, error) {
	ret := new(struct {
		EntityAddress            common.Address
		MetadataContentHashedUri string
		VoteEncryptionPrivateKey string
		Canceled                 bool
		ResultsContentHashedUri  string
	})
	out := ret
	err := _VotingProcess.contract.Call(opts, out, "processes", arg0)
	return *ret, err
}

// Processes is a free data retrieval call binding the contract method 0x579e51c7.
//
// Solidity: function processes(uint256 ) constant returns(address entityAddress, string metadataContentHashedUri, string voteEncryptionPrivateKey, bool canceled, string resultsContentHashedUri)
func (_VotingProcess *VotingProcessSession) Processes(arg0 *big.Int) (struct {
	EntityAddress            common.Address
	MetadataContentHashedUri string
	VoteEncryptionPrivateKey string
	Canceled                 bool
	ResultsContentHashedUri  string
}, error) {
	return _VotingProcess.Contract.Processes(&_VotingProcess.CallOpts, arg0)
}

// Processes is a free data retrieval call binding the contract method 0x579e51c7.
//
// Solidity: function processes(uint256 ) constant returns(address entityAddress, string metadataContentHashedUri, string voteEncryptionPrivateKey, bool canceled, string resultsContentHashedUri)
func (_VotingProcess *VotingProcessCallerSession) Processes(arg0 *big.Int) (struct {
	EntityAddress            common.Address
	MetadataContentHashedUri string
	VoteEncryptionPrivateKey string
	Canceled                 bool
	ResultsContentHashedUri  string
}, error) {
	return _VotingProcess.Contract.Processes(&_VotingProcess.CallOpts, arg0)
}

// AddOracle is a paid mutator transaction binding the contract method 0xc994bc86.
//
// Solidity: function addOracle(string oraclePublicKey) returns()
func (_VotingProcess *VotingProcessTransactor) AddOracle(opts *bind.TransactOpts, oraclePublicKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "addOracle", oraclePublicKey)
}

// AddOracle is a paid mutator transaction binding the contract method 0xc994bc86.
//
// Solidity: function addOracle(string oraclePublicKey) returns()
func (_VotingProcess *VotingProcessSession) AddOracle(oraclePublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.AddOracle(&_VotingProcess.TransactOpts, oraclePublicKey)
}

// AddOracle is a paid mutator transaction binding the contract method 0xc994bc86.
//
// Solidity: function addOracle(string oraclePublicKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) AddOracle(oraclePublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.AddOracle(&_VotingProcess.TransactOpts, oraclePublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0xb5da04f5.
//
// Solidity: function addValidator(string validatorPublicKey) returns()
func (_VotingProcess *VotingProcessTransactor) AddValidator(opts *bind.TransactOpts, validatorPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "addValidator", validatorPublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0xb5da04f5.
//
// Solidity: function addValidator(string validatorPublicKey) returns()
func (_VotingProcess *VotingProcessSession) AddValidator(validatorPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.AddValidator(&_VotingProcess.TransactOpts, validatorPublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0xb5da04f5.
//
// Solidity: function addValidator(string validatorPublicKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) AddValidator(validatorPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.AddValidator(&_VotingProcess.TransactOpts, validatorPublicKey)
}

// Cancel is a paid mutator transaction binding the contract method 0xc4d252f5.
//
// Solidity: function cancel(bytes32 processId) returns()
func (_VotingProcess *VotingProcessTransactor) Cancel(opts *bind.TransactOpts, processId [32]byte) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "cancel", processId)
}

// Cancel is a paid mutator transaction binding the contract method 0xc4d252f5.
//
// Solidity: function cancel(bytes32 processId) returns()
func (_VotingProcess *VotingProcessSession) Cancel(processId [32]byte) (*types.Transaction, error) {
	return _VotingProcess.Contract.Cancel(&_VotingProcess.TransactOpts, processId)
}

// Cancel is a paid mutator transaction binding the contract method 0xc4d252f5.
//
// Solidity: function cancel(bytes32 processId) returns()
func (_VotingProcess *VotingProcessTransactorSession) Cancel(processId [32]byte) (*types.Transaction, error) {
	return _VotingProcess.Contract.Cancel(&_VotingProcess.TransactOpts, processId)
}

// Create is a paid mutator transaction binding the contract method 0xb6a46b3b.
//
// Solidity: function create(string metadataContentHashedUri) returns()
func (_VotingProcess *VotingProcessTransactor) Create(opts *bind.TransactOpts, metadataContentHashedUri string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "create", metadataContentHashedUri)
}

// Create is a paid mutator transaction binding the contract method 0xb6a46b3b.
//
// Solidity: function create(string metadataContentHashedUri) returns()
func (_VotingProcess *VotingProcessSession) Create(metadataContentHashedUri string) (*types.Transaction, error) {
	return _VotingProcess.Contract.Create(&_VotingProcess.TransactOpts, metadataContentHashedUri)
}

// Create is a paid mutator transaction binding the contract method 0xb6a46b3b.
//
// Solidity: function create(string metadataContentHashedUri) returns()
func (_VotingProcess *VotingProcessTransactorSession) Create(metadataContentHashedUri string) (*types.Transaction, error) {
	return _VotingProcess.Contract.Create(&_VotingProcess.TransactOpts, metadataContentHashedUri)
}

// PublishPrivateKey is a paid mutator transaction binding the contract method 0xa803dc4c.
//
// Solidity: function publishPrivateKey(bytes32 processId, string privateKey) returns()
func (_VotingProcess *VotingProcessTransactor) PublishPrivateKey(opts *bind.TransactOpts, processId [32]byte, privateKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "publishPrivateKey", processId, privateKey)
}

// PublishPrivateKey is a paid mutator transaction binding the contract method 0xa803dc4c.
//
// Solidity: function publishPrivateKey(bytes32 processId, string privateKey) returns()
func (_VotingProcess *VotingProcessSession) PublishPrivateKey(processId [32]byte, privateKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.PublishPrivateKey(&_VotingProcess.TransactOpts, processId, privateKey)
}

// PublishPrivateKey is a paid mutator transaction binding the contract method 0xa803dc4c.
//
// Solidity: function publishPrivateKey(bytes32 processId, string privateKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) PublishPrivateKey(processId [32]byte, privateKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.PublishPrivateKey(&_VotingProcess.TransactOpts, processId, privateKey)
}

// PublishResults is a paid mutator transaction binding the contract method 0xec8f670e.
//
// Solidity: function publishResults(bytes32 processId, string resultsContentHashedUri) returns()
func (_VotingProcess *VotingProcessTransactor) PublishResults(opts *bind.TransactOpts, processId [32]byte, resultsContentHashedUri string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "publishResults", processId, resultsContentHashedUri)
}

// PublishResults is a paid mutator transaction binding the contract method 0xec8f670e.
//
// Solidity: function publishResults(bytes32 processId, string resultsContentHashedUri) returns()
func (_VotingProcess *VotingProcessSession) PublishResults(processId [32]byte, resultsContentHashedUri string) (*types.Transaction, error) {
	return _VotingProcess.Contract.PublishResults(&_VotingProcess.TransactOpts, processId, resultsContentHashedUri)
}

// PublishResults is a paid mutator transaction binding the contract method 0xec8f670e.
//
// Solidity: function publishResults(bytes32 processId, string resultsContentHashedUri) returns()
func (_VotingProcess *VotingProcessTransactorSession) PublishResults(processId [32]byte, resultsContentHashedUri string) (*types.Transaction, error) {
	return _VotingProcess.Contract.PublishResults(&_VotingProcess.TransactOpts, processId, resultsContentHashedUri)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x7f07492d.
//
// Solidity: function removeOracle(uint256 idx, string oraclePublicKey) returns()
func (_VotingProcess *VotingProcessTransactor) RemoveOracle(opts *bind.TransactOpts, idx *big.Int, oraclePublicKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "removeOracle", idx, oraclePublicKey)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x7f07492d.
//
// Solidity: function removeOracle(uint256 idx, string oraclePublicKey) returns()
func (_VotingProcess *VotingProcessSession) RemoveOracle(idx *big.Int, oraclePublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.RemoveOracle(&_VotingProcess.TransactOpts, idx, oraclePublicKey)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x7f07492d.
//
// Solidity: function removeOracle(uint256 idx, string oraclePublicKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) RemoveOracle(idx *big.Int, oraclePublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.RemoveOracle(&_VotingProcess.TransactOpts, idx, oraclePublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x8ceb30c2.
//
// Solidity: function removeValidator(uint256 idx, string validatorPublicKey) returns()
func (_VotingProcess *VotingProcessTransactor) RemoveValidator(opts *bind.TransactOpts, idx *big.Int, validatorPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "removeValidator", idx, validatorPublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x8ceb30c2.
//
// Solidity: function removeValidator(uint256 idx, string validatorPublicKey) returns()
func (_VotingProcess *VotingProcessSession) RemoveValidator(idx *big.Int, validatorPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.RemoveValidator(&_VotingProcess.TransactOpts, idx, validatorPublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x8ceb30c2.
//
// Solidity: function removeValidator(uint256 idx, string validatorPublicKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) RemoveValidator(idx *big.Int, validatorPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.RemoveValidator(&_VotingProcess.TransactOpts, idx, validatorPublicKey)
}

// SetChainId is a paid mutator transaction binding the contract method 0xef0e2ff4.
//
// Solidity: function setChainId(uint256 newValue) returns()
func (_VotingProcess *VotingProcessTransactor) SetChainId(opts *bind.TransactOpts, newValue *big.Int) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "setChainId", newValue)
}

// SetChainId is a paid mutator transaction binding the contract method 0xef0e2ff4.
//
// Solidity: function setChainId(uint256 newValue) returns()
func (_VotingProcess *VotingProcessSession) SetChainId(newValue *big.Int) (*types.Transaction, error) {
	return _VotingProcess.Contract.SetChainId(&_VotingProcess.TransactOpts, newValue)
}

// SetChainId is a paid mutator transaction binding the contract method 0xef0e2ff4.
//
// Solidity: function setChainId(uint256 newValue) returns()
func (_VotingProcess *VotingProcessTransactorSession) SetChainId(newValue *big.Int) (*types.Transaction, error) {
	return _VotingProcess.Contract.SetChainId(&_VotingProcess.TransactOpts, newValue)
}

// SetGenesis is a paid mutator transaction binding the contract method 0x47bb3da7.
//
// Solidity: function setGenesis(string newValue) returns()
func (_VotingProcess *VotingProcessTransactor) SetGenesis(opts *bind.TransactOpts, newValue string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "setGenesis", newValue)
}

// SetGenesis is a paid mutator transaction binding the contract method 0x47bb3da7.
//
// Solidity: function setGenesis(string newValue) returns()
func (_VotingProcess *VotingProcessSession) SetGenesis(newValue string) (*types.Transaction, error) {
	return _VotingProcess.Contract.SetGenesis(&_VotingProcess.TransactOpts, newValue)
}

// SetGenesis is a paid mutator transaction binding the contract method 0x47bb3da7.
//
// Solidity: function setGenesis(string newValue) returns()
func (_VotingProcess *VotingProcessTransactorSession) SetGenesis(newValue string) (*types.Transaction, error) {
	return _VotingProcess.Contract.SetGenesis(&_VotingProcess.TransactOpts, newValue)
}

// VotingProcessChainIdChangedIterator is returned from FilterChainIdChanged and is used to iterate over the raw logs and unpacked data for ChainIdChanged events raised by the VotingProcess contract.
type VotingProcessChainIdChangedIterator struct {
	Event *VotingProcessChainIdChanged // Event containing the contract specifics and raw log

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
func (it *VotingProcessChainIdChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessChainIdChanged)
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
		it.Event = new(VotingProcessChainIdChanged)
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
func (it *VotingProcessChainIdChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessChainIdChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessChainIdChanged represents a ChainIdChanged event raised by the VotingProcess contract.
type VotingProcessChainIdChanged struct {
	ChainId *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterChainIdChanged is a free log retrieval operation binding the contract event 0x5a4bfdb771a9b72401d824fd5b321058c7c69fbe4a7c209c37af285e6d061a8c.
//
// Solidity: event ChainIdChanged(uint256 chainId)
func (_VotingProcess *VotingProcessFilterer) FilterChainIdChanged(opts *bind.FilterOpts) (*VotingProcessChainIdChangedIterator, error) {

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "ChainIdChanged")
	if err != nil {
		return nil, err
	}
	return &VotingProcessChainIdChangedIterator{contract: _VotingProcess.contract, event: "ChainIdChanged", logs: logs, sub: sub}, nil
}

// WatchChainIdChanged is a free log subscription operation binding the contract event 0x5a4bfdb771a9b72401d824fd5b321058c7c69fbe4a7c209c37af285e6d061a8c.
//
// Solidity: event ChainIdChanged(uint256 chainId)
func (_VotingProcess *VotingProcessFilterer) WatchChainIdChanged(opts *bind.WatchOpts, sink chan<- *VotingProcessChainIdChanged) (event.Subscription, error) {

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "ChainIdChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessChainIdChanged)
				if err := _VotingProcess.contract.UnpackLog(event, "ChainIdChanged", log); err != nil {
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

// ParseChainIdChanged is a log parse operation binding the contract event 0x5a4bfdb771a9b72401d824fd5b321058c7c69fbe4a7c209c37af285e6d061a8c.
//
// Solidity: event ChainIdChanged(uint256 chainId)
func (_VotingProcess *VotingProcessFilterer) ParseChainIdChanged(log types.Log) (*VotingProcessChainIdChanged, error) {
	event := new(VotingProcessChainIdChanged)
	if err := _VotingProcess.contract.UnpackLog(event, "ChainIdChanged", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessGenesisChangedIterator is returned from FilterGenesisChanged and is used to iterate over the raw logs and unpacked data for GenesisChanged events raised by the VotingProcess contract.
type VotingProcessGenesisChangedIterator struct {
	Event *VotingProcessGenesisChanged // Event containing the contract specifics and raw log

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
func (it *VotingProcessGenesisChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessGenesisChanged)
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
		it.Event = new(VotingProcessGenesisChanged)
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
func (it *VotingProcessGenesisChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessGenesisChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessGenesisChanged represents a GenesisChanged event raised by the VotingProcess contract.
type VotingProcessGenesisChanged struct {
	Genesis string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterGenesisChanged is a free log retrieval operation binding the contract event 0xb07272e5a32a7a57581e0409555209cb59b02e13b62da30135eb3b3431078e36.
//
// Solidity: event GenesisChanged(string genesis)
func (_VotingProcess *VotingProcessFilterer) FilterGenesisChanged(opts *bind.FilterOpts) (*VotingProcessGenesisChangedIterator, error) {

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "GenesisChanged")
	if err != nil {
		return nil, err
	}
	return &VotingProcessGenesisChangedIterator{contract: _VotingProcess.contract, event: "GenesisChanged", logs: logs, sub: sub}, nil
}

// WatchGenesisChanged is a free log subscription operation binding the contract event 0xb07272e5a32a7a57581e0409555209cb59b02e13b62da30135eb3b3431078e36.
//
// Solidity: event GenesisChanged(string genesis)
func (_VotingProcess *VotingProcessFilterer) WatchGenesisChanged(opts *bind.WatchOpts, sink chan<- *VotingProcessGenesisChanged) (event.Subscription, error) {

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "GenesisChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessGenesisChanged)
				if err := _VotingProcess.contract.UnpackLog(event, "GenesisChanged", log); err != nil {
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

// ParseGenesisChanged is a log parse operation binding the contract event 0xb07272e5a32a7a57581e0409555209cb59b02e13b62da30135eb3b3431078e36.
//
// Solidity: event GenesisChanged(string genesis)
func (_VotingProcess *VotingProcessFilterer) ParseGenesisChanged(log types.Log) (*VotingProcessGenesisChanged, error) {
	event := new(VotingProcessGenesisChanged)
	if err := _VotingProcess.contract.UnpackLog(event, "GenesisChanged", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessOracleAddedIterator is returned from FilterOracleAdded and is used to iterate over the raw logs and unpacked data for OracleAdded events raised by the VotingProcess contract.
type VotingProcessOracleAddedIterator struct {
	Event *VotingProcessOracleAdded // Event containing the contract specifics and raw log

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
func (it *VotingProcessOracleAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessOracleAdded)
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
		it.Event = new(VotingProcessOracleAdded)
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
func (it *VotingProcessOracleAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessOracleAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessOracleAdded represents a OracleAdded event raised by the VotingProcess contract.
type VotingProcessOracleAdded struct {
	OraclePublicKey string
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOracleAdded is a free log retrieval operation binding the contract event 0xf2cf567db3ebcbfcad45b1da586f6b7f795e01cad81a993c3cea865f259eec79.
//
// Solidity: event OracleAdded(string oraclePublicKey)
func (_VotingProcess *VotingProcessFilterer) FilterOracleAdded(opts *bind.FilterOpts) (*VotingProcessOracleAddedIterator, error) {

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "OracleAdded")
	if err != nil {
		return nil, err
	}
	return &VotingProcessOracleAddedIterator{contract: _VotingProcess.contract, event: "OracleAdded", logs: logs, sub: sub}, nil
}

// WatchOracleAdded is a free log subscription operation binding the contract event 0xf2cf567db3ebcbfcad45b1da586f6b7f795e01cad81a993c3cea865f259eec79.
//
// Solidity: event OracleAdded(string oraclePublicKey)
func (_VotingProcess *VotingProcessFilterer) WatchOracleAdded(opts *bind.WatchOpts, sink chan<- *VotingProcessOracleAdded) (event.Subscription, error) {

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "OracleAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessOracleAdded)
				if err := _VotingProcess.contract.UnpackLog(event, "OracleAdded", log); err != nil {
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

// ParseOracleAdded is a log parse operation binding the contract event 0xf2cf567db3ebcbfcad45b1da586f6b7f795e01cad81a993c3cea865f259eec79.
//
// Solidity: event OracleAdded(string oraclePublicKey)
func (_VotingProcess *VotingProcessFilterer) ParseOracleAdded(log types.Log) (*VotingProcessOracleAdded, error) {
	event := new(VotingProcessOracleAdded)
	if err := _VotingProcess.contract.UnpackLog(event, "OracleAdded", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessOracleRemovedIterator is returned from FilterOracleRemoved and is used to iterate over the raw logs and unpacked data for OracleRemoved events raised by the VotingProcess contract.
type VotingProcessOracleRemovedIterator struct {
	Event *VotingProcessOracleRemoved // Event containing the contract specifics and raw log

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
func (it *VotingProcessOracleRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessOracleRemoved)
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
		it.Event = new(VotingProcessOracleRemoved)
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
func (it *VotingProcessOracleRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessOracleRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessOracleRemoved represents a OracleRemoved event raised by the VotingProcess contract.
type VotingProcessOracleRemoved struct {
	OraclePublicKey string
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterOracleRemoved is a free log retrieval operation binding the contract event 0xa28dc34059cd4716d51da651406dc8ff96399e3cf46725143a7f2855c68cf394.
//
// Solidity: event OracleRemoved(string oraclePublicKey)
func (_VotingProcess *VotingProcessFilterer) FilterOracleRemoved(opts *bind.FilterOpts) (*VotingProcessOracleRemovedIterator, error) {

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "OracleRemoved")
	if err != nil {
		return nil, err
	}
	return &VotingProcessOracleRemovedIterator{contract: _VotingProcess.contract, event: "OracleRemoved", logs: logs, sub: sub}, nil
}

// WatchOracleRemoved is a free log subscription operation binding the contract event 0xa28dc34059cd4716d51da651406dc8ff96399e3cf46725143a7f2855c68cf394.
//
// Solidity: event OracleRemoved(string oraclePublicKey)
func (_VotingProcess *VotingProcessFilterer) WatchOracleRemoved(opts *bind.WatchOpts, sink chan<- *VotingProcessOracleRemoved) (event.Subscription, error) {

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "OracleRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessOracleRemoved)
				if err := _VotingProcess.contract.UnpackLog(event, "OracleRemoved", log); err != nil {
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

// ParseOracleRemoved is a log parse operation binding the contract event 0xa28dc34059cd4716d51da651406dc8ff96399e3cf46725143a7f2855c68cf394.
//
// Solidity: event OracleRemoved(string oraclePublicKey)
func (_VotingProcess *VotingProcessFilterer) ParseOracleRemoved(log types.Log) (*VotingProcessOracleRemoved, error) {
	event := new(VotingProcessOracleRemoved)
	if err := _VotingProcess.contract.UnpackLog(event, "OracleRemoved", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessPrivateKeyPublishedIterator is returned from FilterPrivateKeyPublished and is used to iterate over the raw logs and unpacked data for PrivateKeyPublished events raised by the VotingProcess contract.
type VotingProcessPrivateKeyPublishedIterator struct {
	Event *VotingProcessPrivateKeyPublished // Event containing the contract specifics and raw log

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
func (it *VotingProcessPrivateKeyPublishedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessPrivateKeyPublished)
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
		it.Event = new(VotingProcessPrivateKeyPublished)
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
func (it *VotingProcessPrivateKeyPublishedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessPrivateKeyPublishedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessPrivateKeyPublished represents a PrivateKeyPublished event raised by the VotingProcess contract.
type VotingProcessPrivateKeyPublished struct {
	ProcessId  [32]byte
	PrivateKey string
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterPrivateKeyPublished is a free log retrieval operation binding the contract event 0x16e81256ba7e41ceea97db602f7c59414ba5110c3a6641245f6621279f626301.
//
// Solidity: event PrivateKeyPublished(bytes32 indexed processId, string privateKey)
func (_VotingProcess *VotingProcessFilterer) FilterPrivateKeyPublished(opts *bind.FilterOpts, processId [][32]byte) (*VotingProcessPrivateKeyPublishedIterator, error) {

	var processIdRule []interface{}
	for _, processIdItem := range processId {
		processIdRule = append(processIdRule, processIdItem)
	}

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "PrivateKeyPublished", processIdRule)
	if err != nil {
		return nil, err
	}
	return &VotingProcessPrivateKeyPublishedIterator{contract: _VotingProcess.contract, event: "PrivateKeyPublished", logs: logs, sub: sub}, nil
}

// WatchPrivateKeyPublished is a free log subscription operation binding the contract event 0x16e81256ba7e41ceea97db602f7c59414ba5110c3a6641245f6621279f626301.
//
// Solidity: event PrivateKeyPublished(bytes32 indexed processId, string privateKey)
func (_VotingProcess *VotingProcessFilterer) WatchPrivateKeyPublished(opts *bind.WatchOpts, sink chan<- *VotingProcessPrivateKeyPublished, processId [][32]byte) (event.Subscription, error) {

	var processIdRule []interface{}
	for _, processIdItem := range processId {
		processIdRule = append(processIdRule, processIdItem)
	}

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "PrivateKeyPublished", processIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessPrivateKeyPublished)
				if err := _VotingProcess.contract.UnpackLog(event, "PrivateKeyPublished", log); err != nil {
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

// ParsePrivateKeyPublished is a log parse operation binding the contract event 0x16e81256ba7e41ceea97db602f7c59414ba5110c3a6641245f6621279f626301.
//
// Solidity: event PrivateKeyPublished(bytes32 indexed processId, string privateKey)
func (_VotingProcess *VotingProcessFilterer) ParsePrivateKeyPublished(log types.Log) (*VotingProcessPrivateKeyPublished, error) {
	event := new(VotingProcessPrivateKeyPublished)
	if err := _VotingProcess.contract.UnpackLog(event, "PrivateKeyPublished", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessProcessCanceledIterator is returned from FilterProcessCanceled and is used to iterate over the raw logs and unpacked data for ProcessCanceled events raised by the VotingProcess contract.
type VotingProcessProcessCanceledIterator struct {
	Event *VotingProcessProcessCanceled // Event containing the contract specifics and raw log

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
func (it *VotingProcessProcessCanceledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessProcessCanceled)
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
		it.Event = new(VotingProcessProcessCanceled)
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
func (it *VotingProcessProcessCanceledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessProcessCanceledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessProcessCanceled represents a ProcessCanceled event raised by the VotingProcess contract.
type VotingProcessProcessCanceled struct {
	EntityAddress common.Address
	ProcessId     [32]byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterProcessCanceled is a free log retrieval operation binding the contract event 0xa8a2eafb1f78e64e1c3921d10b28aad02d1fa21cba6bbc76b7e8601b19a9c08d.
//
// Solidity: event ProcessCanceled(address indexed entityAddress, bytes32 processId)
func (_VotingProcess *VotingProcessFilterer) FilterProcessCanceled(opts *bind.FilterOpts, entityAddress []common.Address) (*VotingProcessProcessCanceledIterator, error) {

	var entityAddressRule []interface{}
	for _, entityAddressItem := range entityAddress {
		entityAddressRule = append(entityAddressRule, entityAddressItem)
	}

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "ProcessCanceled", entityAddressRule)
	if err != nil {
		return nil, err
	}
	return &VotingProcessProcessCanceledIterator{contract: _VotingProcess.contract, event: "ProcessCanceled", logs: logs, sub: sub}, nil
}

// WatchProcessCanceled is a free log subscription operation binding the contract event 0xa8a2eafb1f78e64e1c3921d10b28aad02d1fa21cba6bbc76b7e8601b19a9c08d.
//
// Solidity: event ProcessCanceled(address indexed entityAddress, bytes32 processId)
func (_VotingProcess *VotingProcessFilterer) WatchProcessCanceled(opts *bind.WatchOpts, sink chan<- *VotingProcessProcessCanceled, entityAddress []common.Address) (event.Subscription, error) {

	var entityAddressRule []interface{}
	for _, entityAddressItem := range entityAddress {
		entityAddressRule = append(entityAddressRule, entityAddressItem)
	}

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "ProcessCanceled", entityAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessProcessCanceled)
				if err := _VotingProcess.contract.UnpackLog(event, "ProcessCanceled", log); err != nil {
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

// ParseProcessCanceled is a log parse operation binding the contract event 0xa8a2eafb1f78e64e1c3921d10b28aad02d1fa21cba6bbc76b7e8601b19a9c08d.
//
// Solidity: event ProcessCanceled(address indexed entityAddress, bytes32 processId)
func (_VotingProcess *VotingProcessFilterer) ParseProcessCanceled(log types.Log) (*VotingProcessProcessCanceled, error) {
	event := new(VotingProcessProcessCanceled)
	if err := _VotingProcess.contract.UnpackLog(event, "ProcessCanceled", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessProcessCreatedIterator is returned from FilterProcessCreated and is used to iterate over the raw logs and unpacked data for ProcessCreated events raised by the VotingProcess contract.
type VotingProcessProcessCreatedIterator struct {
	Event *VotingProcessProcessCreated // Event containing the contract specifics and raw log

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
func (it *VotingProcessProcessCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessProcessCreated)
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
		it.Event = new(VotingProcessProcessCreated)
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
func (it *VotingProcessProcessCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessProcessCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessProcessCreated represents a ProcessCreated event raised by the VotingProcess contract.
type VotingProcessProcessCreated struct {
	EntityAddress common.Address
	ProcessId     [32]byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterProcessCreated is a free log retrieval operation binding the contract event 0x9965a4077be8cb89c7c91129beed322708724b0449a1a2ba88ddae1d0b2d7172.
//
// Solidity: event ProcessCreated(address indexed entityAddress, bytes32 processId)
func (_VotingProcess *VotingProcessFilterer) FilterProcessCreated(opts *bind.FilterOpts, entityAddress []common.Address) (*VotingProcessProcessCreatedIterator, error) {

	var entityAddressRule []interface{}
	for _, entityAddressItem := range entityAddress {
		entityAddressRule = append(entityAddressRule, entityAddressItem)
	}

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "ProcessCreated", entityAddressRule)
	if err != nil {
		return nil, err
	}
	return &VotingProcessProcessCreatedIterator{contract: _VotingProcess.contract, event: "ProcessCreated", logs: logs, sub: sub}, nil
}

// WatchProcessCreated is a free log subscription operation binding the contract event 0x9965a4077be8cb89c7c91129beed322708724b0449a1a2ba88ddae1d0b2d7172.
//
// Solidity: event ProcessCreated(address indexed entityAddress, bytes32 processId)
func (_VotingProcess *VotingProcessFilterer) WatchProcessCreated(opts *bind.WatchOpts, sink chan<- *VotingProcessProcessCreated, entityAddress []common.Address) (event.Subscription, error) {

	var entityAddressRule []interface{}
	for _, entityAddressItem := range entityAddress {
		entityAddressRule = append(entityAddressRule, entityAddressItem)
	}

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "ProcessCreated", entityAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessProcessCreated)
				if err := _VotingProcess.contract.UnpackLog(event, "ProcessCreated", log); err != nil {
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

// ParseProcessCreated is a log parse operation binding the contract event 0x9965a4077be8cb89c7c91129beed322708724b0449a1a2ba88ddae1d0b2d7172.
//
// Solidity: event ProcessCreated(address indexed entityAddress, bytes32 processId)
func (_VotingProcess *VotingProcessFilterer) ParseProcessCreated(log types.Log) (*VotingProcessProcessCreated, error) {
	event := new(VotingProcessProcessCreated)
	if err := _VotingProcess.contract.UnpackLog(event, "ProcessCreated", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessResultsPublishedIterator is returned from FilterResultsPublished and is used to iterate over the raw logs and unpacked data for ResultsPublished events raised by the VotingProcess contract.
type VotingProcessResultsPublishedIterator struct {
	Event *VotingProcessResultsPublished // Event containing the contract specifics and raw log

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
func (it *VotingProcessResultsPublishedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessResultsPublished)
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
		it.Event = new(VotingProcessResultsPublished)
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
func (it *VotingProcessResultsPublishedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessResultsPublishedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessResultsPublished represents a ResultsPublished event raised by the VotingProcess contract.
type VotingProcessResultsPublished struct {
	ProcessId [32]byte
	Results   string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterResultsPublished is a free log retrieval operation binding the contract event 0x8b7614f77555532e6b033ab6da00ac40495940598b8916b1c77a44351428ae0a.
//
// Solidity: event ResultsPublished(bytes32 indexed processId, string results)
func (_VotingProcess *VotingProcessFilterer) FilterResultsPublished(opts *bind.FilterOpts, processId [][32]byte) (*VotingProcessResultsPublishedIterator, error) {

	var processIdRule []interface{}
	for _, processIdItem := range processId {
		processIdRule = append(processIdRule, processIdItem)
	}

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "ResultsPublished", processIdRule)
	if err != nil {
		return nil, err
	}
	return &VotingProcessResultsPublishedIterator{contract: _VotingProcess.contract, event: "ResultsPublished", logs: logs, sub: sub}, nil
}

// WatchResultsPublished is a free log subscription operation binding the contract event 0x8b7614f77555532e6b033ab6da00ac40495940598b8916b1c77a44351428ae0a.
//
// Solidity: event ResultsPublished(bytes32 indexed processId, string results)
func (_VotingProcess *VotingProcessFilterer) WatchResultsPublished(opts *bind.WatchOpts, sink chan<- *VotingProcessResultsPublished, processId [][32]byte) (event.Subscription, error) {

	var processIdRule []interface{}
	for _, processIdItem := range processId {
		processIdRule = append(processIdRule, processIdItem)
	}

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "ResultsPublished", processIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessResultsPublished)
				if err := _VotingProcess.contract.UnpackLog(event, "ResultsPublished", log); err != nil {
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

// ParseResultsPublished is a log parse operation binding the contract event 0x8b7614f77555532e6b033ab6da00ac40495940598b8916b1c77a44351428ae0a.
//
// Solidity: event ResultsPublished(bytes32 indexed processId, string results)
func (_VotingProcess *VotingProcessFilterer) ParseResultsPublished(log types.Log) (*VotingProcessResultsPublished, error) {
	event := new(VotingProcessResultsPublished)
	if err := _VotingProcess.contract.UnpackLog(event, "ResultsPublished", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessValidatorAddedIterator is returned from FilterValidatorAdded and is used to iterate over the raw logs and unpacked data for ValidatorAdded events raised by the VotingProcess contract.
type VotingProcessValidatorAddedIterator struct {
	Event *VotingProcessValidatorAdded // Event containing the contract specifics and raw log

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
func (it *VotingProcessValidatorAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessValidatorAdded)
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
		it.Event = new(VotingProcessValidatorAdded)
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
func (it *VotingProcessValidatorAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessValidatorAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessValidatorAdded represents a ValidatorAdded event raised by the VotingProcess contract.
type VotingProcessValidatorAdded struct {
	ValidatorPublicKey string
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterValidatorAdded is a free log retrieval operation binding the contract event 0x0eb17eb6d7f643e1f1a79af44e460fffabb6b2a8beff44bff08160d8d3403d3f.
//
// Solidity: event ValidatorAdded(string validatorPublicKey)
func (_VotingProcess *VotingProcessFilterer) FilterValidatorAdded(opts *bind.FilterOpts) (*VotingProcessValidatorAddedIterator, error) {

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "ValidatorAdded")
	if err != nil {
		return nil, err
	}
	return &VotingProcessValidatorAddedIterator{contract: _VotingProcess.contract, event: "ValidatorAdded", logs: logs, sub: sub}, nil
}

// WatchValidatorAdded is a free log subscription operation binding the contract event 0x0eb17eb6d7f643e1f1a79af44e460fffabb6b2a8beff44bff08160d8d3403d3f.
//
// Solidity: event ValidatorAdded(string validatorPublicKey)
func (_VotingProcess *VotingProcessFilterer) WatchValidatorAdded(opts *bind.WatchOpts, sink chan<- *VotingProcessValidatorAdded) (event.Subscription, error) {

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "ValidatorAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessValidatorAdded)
				if err := _VotingProcess.contract.UnpackLog(event, "ValidatorAdded", log); err != nil {
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

// ParseValidatorAdded is a log parse operation binding the contract event 0x0eb17eb6d7f643e1f1a79af44e460fffabb6b2a8beff44bff08160d8d3403d3f.
//
// Solidity: event ValidatorAdded(string validatorPublicKey)
func (_VotingProcess *VotingProcessFilterer) ParseValidatorAdded(log types.Log) (*VotingProcessValidatorAdded, error) {
	event := new(VotingProcessValidatorAdded)
	if err := _VotingProcess.contract.UnpackLog(event, "ValidatorAdded", log); err != nil {
		return nil, err
	}
	return event, nil
}

// VotingProcessValidatorRemovedIterator is returned from FilterValidatorRemoved and is used to iterate over the raw logs and unpacked data for ValidatorRemoved events raised by the VotingProcess contract.
type VotingProcessValidatorRemovedIterator struct {
	Event *VotingProcessValidatorRemoved // Event containing the contract specifics and raw log

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
func (it *VotingProcessValidatorRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(VotingProcessValidatorRemoved)
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
		it.Event = new(VotingProcessValidatorRemoved)
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
func (it *VotingProcessValidatorRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *VotingProcessValidatorRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// VotingProcessValidatorRemoved represents a ValidatorRemoved event raised by the VotingProcess contract.
type VotingProcessValidatorRemoved struct {
	ValidatorPublicKey string
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterValidatorRemoved is a free log retrieval operation binding the contract event 0x53344ca00b011ca20d3dc9f1bb71ed60e097b598b9f35482879138cc15f28ef9.
//
// Solidity: event ValidatorRemoved(string validatorPublicKey)
func (_VotingProcess *VotingProcessFilterer) FilterValidatorRemoved(opts *bind.FilterOpts) (*VotingProcessValidatorRemovedIterator, error) {

	logs, sub, err := _VotingProcess.contract.FilterLogs(opts, "ValidatorRemoved")
	if err != nil {
		return nil, err
	}
	return &VotingProcessValidatorRemovedIterator{contract: _VotingProcess.contract, event: "ValidatorRemoved", logs: logs, sub: sub}, nil
}

// WatchValidatorRemoved is a free log subscription operation binding the contract event 0x53344ca00b011ca20d3dc9f1bb71ed60e097b598b9f35482879138cc15f28ef9.
//
// Solidity: event ValidatorRemoved(string validatorPublicKey)
func (_VotingProcess *VotingProcessFilterer) WatchValidatorRemoved(opts *bind.WatchOpts, sink chan<- *VotingProcessValidatorRemoved) (event.Subscription, error) {

	logs, sub, err := _VotingProcess.contract.WatchLogs(opts, "ValidatorRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(VotingProcessValidatorRemoved)
				if err := _VotingProcess.contract.UnpackLog(event, "ValidatorRemoved", log); err != nil {
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

// ParseValidatorRemoved is a log parse operation binding the contract event 0x53344ca00b011ca20d3dc9f1bb71ed60e097b598b9f35482879138cc15f28ef9.
//
// Solidity: event ValidatorRemoved(string validatorPublicKey)
func (_VotingProcess *VotingProcessFilterer) ParseValidatorRemoved(log types.Log) (*VotingProcessValidatorRemoved, error) {
	event := new(VotingProcessValidatorRemoved)
	if err := _VotingProcess.contract.UnpackLog(event, "ValidatorRemoved", log); err != nil {
		return nil, err
	}
	return event, nil
}
