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

// GenesisABI is the input ABI used to generate the binding from.
const GenesisABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"}],\"name\":\"ChainRegistered\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"}],\"name\":\"GenesisUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"OracleAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"OracleRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"validatorPublicKey\",\"type\":\"bytes\"}],\"name\":\"ValidatorAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"validatorPublicKey\",\"type\":\"bytes\"}],\"name\":\"ValidatorRemoved\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"addOracle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"bytes\",\"name\":\"validatorPublicKey\",\"type\":\"bytes\"}],\"name\":\"addValidator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"}],\"name\":\"get\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"genesis\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"validators\",\"type\":\"bytes[]\"},{\"internalType\":\"address[]\",\"name\":\"oracles\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getChainCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"isOracle\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"bytes\",\"name\":\"validatorPublicKey\",\"type\":\"bytes\"}],\"name\":\"isValidator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"genesis\",\"type\":\"string\"},{\"internalType\":\"bytes[]\",\"name\":\"validatorList\",\"type\":\"bytes[]\"},{\"internalType\":\"address[]\",\"name\":\"oracleList\",\"type\":\"address[]\"}],\"name\":\"newChain\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"newChainId\",\"type\":\"uint32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"idx\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"removeOracle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"idx\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"validatorPublicKey\",\"type\":\"bytes\"}],\"name\":\"removeValidator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"chainId\",\"type\":\"uint32\"},{\"internalType\":\"string\",\"name\":\"newGenesis\",\"type\":\"string\"}],\"name\":\"setGenesis\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// GenesisBin is the compiled bytecode used for deploying new contracts.
var GenesisBin = "0x608060405234801561001057600080fd5b50600080546001600160a01b03191633179055611675806100326000396000f3fe608060405234801561001057600080fd5b506004361061009d5760003560e01c8063bb99d77a11610066578063bb99d77a14610119578063d25d23bd1461012c578063d32954a514610141578063d4f5215214610154578063d8a26e3a146101675761009d565b80623fcf71146100a25780630aa4bac2146100b757806360d6c264146100e057806374af92a2146100f35780639c8ea8cc14610106575b600080fd5b6100b56100b03660046112d9565b610189565b005b6100ca6100c536600461123b565b610365565b6040516100d791906113b7565b60405180910390f35b6100b56100ee36600461126f565b61039c565b6100ca61010136600461126f565b6104c4565b6100b5610114366004611316565b610508565b6100b561012736600461123b565b610743565b61013461084f565b6040516100d7919061155f565b6100b561014f3660046112bc565b61085b565b61013461016236600461114d565b6109d6565b61017a61017536600461121f565b610b7c565b6040516100d7939291906113c2565b6000546001600160a01b031633146101bc5760405162461bcd60e51b81526004016101b390611470565b60405180910390fd5b60025463ffffffff908116908416106101e75760405162461bcd60e51b81526004016101b39061149b565b63ffffffff8316600090815260016020526040902060028101548084106102205760405162461bcd60e51b81526004016101b3906114be565b826001600160a01b031682600201858154811061023957fe5b6000918252602090912001546001600160a01b03161461026b5760405162461bcd60e51b81526004016101b3906114e5565b81600201600182038154811061027d57fe5b6000918252602090912001546002830180546001600160a01b0390921691869081106102a557fe5b9060005260206000200160006101000a8154816001600160a01b0302191690836001600160a01b03160217905550816002018054806102e057fe5b60008281526020808220830160001990810180546001600160a01b03191690559092019092556001600160a01b0385168252600484019052604090819020805460ff19169055517f4b8171862540a056f44154d626c1390fed806ee806026247dc8c19f47b09acbe906103569087908690611570565b60405180910390a15050505050565b63ffffffff821660009081526001602090815260408083206001600160a01b038516845260040190915290205460ff165b92915050565b6000546001600160a01b031633146103c65760405162461bcd60e51b81526004016101b390611470565b60025463ffffffff908116908316106103f15760405162461bcd60e51b81526004016101b39061149b565b6103fb82826104c4565b156104185760405162461bcd60e51b81526004016101b390611536565b63ffffffff821660009081526001602081815260408320808301805493840181558452928190208451610452939190910191850190610dfe565b5060018160030183604051610467919061139b565b908152604051908190036020018120805492151560ff19909316929092179091557f3d8adf342e55e97b1f85be8e952d2b473ec50bb2004821559c2b440f0a589e4e906104b7908590859061158f565b60405180910390a1505050565b63ffffffff821660009081526001602052604080822090516003909101906104ed90849061139b565b9081526040519081900360200190205460ff16905092915050565b6000546001600160a01b031633146105325760405162461bcd60e51b81526004016101b390611470565b60025463ffffffff9081169084161061055d5760405162461bcd60e51b81526004016101b39061149b565b63ffffffff83166000908152600160208190526040909120908101548084106105985760405162461bcd60e51b81526004016101b3906114be565b6106438260010185815481106105aa57fe5b600091825260209182902001805460408051601f60026000196101006001871615020190941693909304928301859004850281018501909152818152928301828280156106385780601f1061060d57610100808354040283529160200191610638565b820191906000526020600020905b81548152906001019060200180831161061b57829003601f168201915b505050505084610d92565b61065f5760405162461bcd60e51b81526004016101b3906114e5565b81600101600182038154811061067157fe5b9060005260206000200182600101858154811061068a57fe5b9060005260206000200190805460018160011615610100020316600290046106b3929190610e7c565b50816001018054806106c157fe5b6001900381819060005260206000200160006106dd9190610ef1565b9055600082600301846040516106f3919061139b565b908152604051908190036020018120805492151560ff19909316929092179091557f7436794ad809d5819bbcbd64a94846c7da193b8280de35e7ce59d3b7b4e6bbe190610356908790869061158f565b6000546001600160a01b0316331461076d5760405162461bcd60e51b81526004016101b390611470565b60025463ffffffff908116908316106107985760405162461bcd60e51b81526004016101b39061149b565b6107a28282610365565b156107bf5760405162461bcd60e51b81526004016101b390611536565b63ffffffff8216600090815260016020818152604080842060028101805480860182559086528386200180546001600160a01b0319166001600160a01b03881690811790915585526004810190925292839020805460ff191690921790915590517f572d29453222865ac78ef8d936cb20aba9368900deb6db64997c1482fe2b30c9906104b79085908590611570565b60025463ffffffff1690565b6000546001600160a01b031633146108855760405162461bcd60e51b81526004016101b390611470565b60025463ffffffff908116908316106108b05760405162461bcd60e51b81526004016101b39061149b565b63ffffffff821660009081526001602081815260409283902080548451600294821615610100026000190190911693909304601f810183900483028401830190945283835261095793909183018282801561094c5780601f106109215761010080835404028352916020019161094c565b820191906000526020600020905b81548152906001019060200180831161092f57829003601f168201915b505050505082610deb565b156109745760405162461bcd60e51b81526004016101b390611511565b63ffffffff82166000908152600160209081526040909120825161099a92840190610dfe565b507f87f64dce9746fc7da2e672b4aacc82ad148ed5900411894ddcbe532618fa89fb826040516109ca919061155f565b60405180910390a15050565b600080546001600160a01b03163314610a015760405162461bcd60e51b81526004016101b390611470565b60025463ffffffff16600090815260016020908152604090912085519091610a2d918391880190610dfe565b508351610a439060018301906020870190610f38565b508251610a599060028301906020860190610f91565b5060005b8451811015610ab557600182600301868381518110610a7857fe5b6020026020010151604051610a8d919061139b565b908152604051908190036020019020805491151560ff19909216919091179055600101610a5d565b5060005b8351811015610b10576001826004016000868481518110610ad657fe5b6020908102919091018101516001600160a01b03168252810191909152604001600020805460ff1916911515919091179055600101610ab9565b506002546040517fced3baa88aa65d52234f5717c8b053dc44bb9df530b1f6784809640ed322b7e991610b4b9163ffffffff9091169061155f565b60405180910390a150506002805463ffffffff198116600163ffffffff928316908101909216179091559392505050565b6002546060908190819063ffffffff90811690851610610bae5760405162461bcd60e51b81526004016101b39061149b565b63ffffffff8416600090815260016020818152604092839020805484516002828616156101000260001901909216829004601f810185900485028201850190965285815291949385019390850192859190830182828015610c505780601f10610c2557610100808354040283529160200191610c50565b820191906000526020600020905b815481529060010190602001808311610c3357829003601f168201915b5050505050925081805480602002602001604051908101604052809291908181526020016000905b82821015610d235760008481526020908190208301805460408051601f6002600019610100600187161502019094169390930492830185900485028101850190915281815292830182828015610d0f5780601f10610ce457610100808354040283529160200191610d0f565b820191906000526020600020905b815481529060010190602001808311610cf257829003601f168201915b505050505081526020019060010190610c78565b50505050915080805480602002602001604051908101604052809291908181526020018280548015610d7e57602002820191906000526020600020905b81546001600160a01b03168152600190910190602001808311610d60575b505050505090509250925092509193909250565b600081604051602001610da5919061139b565b6040516020818303038152906040528051906020012083604051602001610dcc919061139b565b6040516020818303038152906040528051906020012014905092915050565b6000610df78383610d92565b9392505050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10610e3f57805160ff1916838001178555610e6c565b82800160010185558215610e6c579182015b82811115610e6c578251825591602001919060010190610e51565b50610e78929150610ff2565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10610eb55780548555610e6c565b82800160010185558215610e6c57600052602060002091601f016020900482015b82811115610e6c578254825591600101919060010190610ed6565b50805460018160011615610100020316600290046000825580601f10610f175750610f35565b601f016020900490600052602060002090810190610f359190610ff2565b50565b828054828255906000526020600020908101928215610f85579160200282015b82811115610f855782518051610f75918491602090910190610dfe565b5091602001919060010190610f58565b50610e78929150611007565b828054828255906000526020600020908101928215610fe6579160200282015b82811115610fe657825182546001600160a01b0319166001600160a01b03909116178255602090920191600190910190610fb1565b50610e78929150611024565b5b80821115610e785760008155600101610ff3565b80821115610e7857600061101b8282610ef1565b50600101611007565b5b80821115610e785780546001600160a01b0319168155600101611025565b80356001600160a01b038116811461039657600080fd5b600082601f83011261106a578081fd5b813561107d611078826115dd565b6115b6565b81815291506020808301908481018184028601820187101561109e57600080fd5b60005b848110156110c5576110b38883611043565b845292820192908201906001016110a1565b505050505092915050565b600082601f8301126110e0578081fd5b813567ffffffffffffffff8111156110f6578182fd5b611109601f8201601f19166020016115b6565b915080825283602082850101111561112057600080fd5b8060208401602084013760009082016020015292915050565b803563ffffffff8116811461039657600080fd5b600080600060608486031215611161578283fd5b833567ffffffffffffffff80821115611178578485fd5b611184878388016110d0565b945060209150818601358181111561119a578485fd5b8601601f810188136111aa578485fd5b80356111b8611078826115dd565b81815284810190838601885b848110156111ed576111db8d8984358901016110d0565b845292870192908701906001016111c4565b50909750505050604087013592505080821115611208578283fd5b506112158682870161105a565b9150509250925092565b600060208284031215611230578081fd5b8135610df78161162d565b6000806040838503121561124d578182fd5b6112578484611139565b91506112668460208501611043565b90509250929050565b60008060408385031215611281578182fd5b61128b8484611139565b9150602083013567ffffffffffffffff8111156112a6578182fd5b6112b2858286016110d0565b9150509250929050565b600080604083850312156112ce578182fd5b823561128b8161162d565b6000806000606084860312156112ed578283fd5b6112f78585611139565b92506020840135915061130d8560408601611043565b90509250925092565b60008060006060848603121561132a578283fd5b6113348585611139565b925060208401359150604084013567ffffffffffffffff811115611356578182fd5b611215868287016110d0565b6001600160a01b03169052565b600081518084526113878160208601602086016115fd565b601f01601f19169290920160200192915050565b600082516113ad8184602087016115fd565b9190910192915050565b901515815260200190565b6000606082526113d5606083018661136f565b602083820381850152818651808452828401915082838202850101838901865b8381101561142357601f1987840301855261141183835161136f565b948601949250908501906001016113f5565b5050868103604088015287518082529084019450915050818601845b8281101561146257611452858351611362565b938301939083019060010161143f565b509298975050505050505050565b60208082526011908201527037b7363ca1b7b73a3930b1ba27bbb732b960791b604082015260600190565b602080825260099082015268139bdd08199bdd5b9960ba1b604082015260600190565b6020808252600d908201526c092dcecc2d8d2c840d2dcc8caf609b1b604082015260600190565b602080825260129082015271092dcc8caf05ad6caf240dad2e6dac2e8c6d60731b604082015260600190565b6020808252600b908201526a26bab9ba103234b33332b960a91b604082015260600190565b6020808252600f908201526e105b1c9958591e481c1c995cd95b9d608a1b604082015260600190565b63ffffffff91909116815260200190565b63ffffffff9290921682526001600160a01b0316602082015260400190565b600063ffffffff84168252604060208301526115ae604083018461136f565b949350505050565b60405181810167ffffffffffffffff811182821017156115d557600080fd5b604052919050565b600067ffffffffffffffff8211156115f3578081fd5b5060209081020190565b60005b83811015611618578181015183820152602001611600565b83811115611627576000848401525b50505050565b63ffffffff81168114610f3557600080fdfea2646970667358221220b4d1943194d45a38081a1d63eadfafc3b3d1c38219fc75690e451210d0c8a82864736f6c634300060c0033"

// DeployGenesis deploys a new Ethereum contract, binding an instance of Genesis to it.
func DeployGenesis(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Genesis, error) {
	parsed, err := abi.JSON(strings.NewReader(GenesisABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(GenesisBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Genesis{GenesisCaller: GenesisCaller{contract: contract}, GenesisTransactor: GenesisTransactor{contract: contract}, GenesisFilterer: GenesisFilterer{contract: contract}}, nil
}

// Genesis is an auto generated Go binding around an Ethereum contract.
type Genesis struct {
	GenesisCaller     // Read-only binding to the contract
	GenesisTransactor // Write-only binding to the contract
	GenesisFilterer   // Log filterer for contract events
}

// GenesisCaller is an auto generated read-only Go binding around an Ethereum contract.
type GenesisCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GenesisTransactor is an auto generated write-only Go binding around an Ethereum contract.
type GenesisTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GenesisFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type GenesisFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GenesisSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type GenesisSession struct {
	Contract     *Genesis          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// GenesisCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type GenesisCallerSession struct {
	Contract *GenesisCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// GenesisTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type GenesisTransactorSession struct {
	Contract     *GenesisTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// GenesisRaw is an auto generated low-level Go binding around an Ethereum contract.
type GenesisRaw struct {
	Contract *Genesis // Generic contract binding to access the raw methods on
}

// GenesisCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type GenesisCallerRaw struct {
	Contract *GenesisCaller // Generic read-only contract binding to access the raw methods on
}

// GenesisTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type GenesisTransactorRaw struct {
	Contract *GenesisTransactor // Generic write-only contract binding to access the raw methods on
}

// NewGenesis creates a new instance of Genesis, bound to a specific deployed contract.
func NewGenesis(address common.Address, backend bind.ContractBackend) (*Genesis, error) {
	contract, err := bindGenesis(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Genesis{GenesisCaller: GenesisCaller{contract: contract}, GenesisTransactor: GenesisTransactor{contract: contract}, GenesisFilterer: GenesisFilterer{contract: contract}}, nil
}

// NewGenesisCaller creates a new read-only instance of Genesis, bound to a specific deployed contract.
func NewGenesisCaller(address common.Address, caller bind.ContractCaller) (*GenesisCaller, error) {
	contract, err := bindGenesis(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &GenesisCaller{contract: contract}, nil
}

// NewGenesisTransactor creates a new write-only instance of Genesis, bound to a specific deployed contract.
func NewGenesisTransactor(address common.Address, transactor bind.ContractTransactor) (*GenesisTransactor, error) {
	contract, err := bindGenesis(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &GenesisTransactor{contract: contract}, nil
}

// NewGenesisFilterer creates a new log filterer instance of Genesis, bound to a specific deployed contract.
func NewGenesisFilterer(address common.Address, filterer bind.ContractFilterer) (*GenesisFilterer, error) {
	contract, err := bindGenesis(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &GenesisFilterer{contract: contract}, nil
}

// bindGenesis binds a generic wrapper to an already deployed contract.
func bindGenesis(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(GenesisABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Genesis *GenesisRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Genesis.Contract.GenesisCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Genesis *GenesisRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Genesis.Contract.GenesisTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Genesis *GenesisRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Genesis.Contract.GenesisTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Genesis *GenesisCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Genesis.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Genesis *GenesisTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Genesis.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Genesis *GenesisTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Genesis.Contract.contract.Transact(opts, method, params...)
}

// Get is a free data retrieval call binding the contract method 0xd8a26e3a.
//
// Solidity: function get(uint32 chainId) view returns(string genesis, bytes[] validators, address[] oracles)
func (_Genesis *GenesisCaller) Get(opts *bind.CallOpts, chainId uint32) (struct {
	Genesis    string
	Validators [][]byte
	Oracles    []common.Address
}, error) {
	var out []interface{}
	err := _Genesis.contract.Call(opts, &out, "get", chainId)

	outstruct := new(struct {
		Genesis    string
		Validators [][]byte
		Oracles    []common.Address
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Genesis = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.Validators = *abi.ConvertType(out[1], new([][]byte)).(*[][]byte)
	outstruct.Oracles = *abi.ConvertType(out[2], new([]common.Address)).(*[]common.Address)

	return *outstruct, err

}

// Get is a free data retrieval call binding the contract method 0xd8a26e3a.
//
// Solidity: function get(uint32 chainId) view returns(string genesis, bytes[] validators, address[] oracles)
func (_Genesis *GenesisSession) Get(chainId uint32) (struct {
	Genesis    string
	Validators [][]byte
	Oracles    []common.Address
}, error) {
	return _Genesis.Contract.Get(&_Genesis.CallOpts, chainId)
}

// Get is a free data retrieval call binding the contract method 0xd8a26e3a.
//
// Solidity: function get(uint32 chainId) view returns(string genesis, bytes[] validators, address[] oracles)
func (_Genesis *GenesisCallerSession) Get(chainId uint32) (struct {
	Genesis    string
	Validators [][]byte
	Oracles    []common.Address
}, error) {
	return _Genesis.Contract.Get(&_Genesis.CallOpts, chainId)
}

// GetChainCount is a free data retrieval call binding the contract method 0xd25d23bd.
//
// Solidity: function getChainCount() view returns(uint32)
func (_Genesis *GenesisCaller) GetChainCount(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _Genesis.contract.Call(opts, &out, "getChainCount")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// GetChainCount is a free data retrieval call binding the contract method 0xd25d23bd.
//
// Solidity: function getChainCount() view returns(uint32)
func (_Genesis *GenesisSession) GetChainCount() (uint32, error) {
	return _Genesis.Contract.GetChainCount(&_Genesis.CallOpts)
}

// GetChainCount is a free data retrieval call binding the contract method 0xd25d23bd.
//
// Solidity: function getChainCount() view returns(uint32)
func (_Genesis *GenesisCallerSession) GetChainCount() (uint32, error) {
	return _Genesis.Contract.GetChainCount(&_Genesis.CallOpts)
}

// IsOracle is a free data retrieval call binding the contract method 0x0aa4bac2.
//
// Solidity: function isOracle(uint32 chainId, address oracleAddress) view returns(bool)
func (_Genesis *GenesisCaller) IsOracle(opts *bind.CallOpts, chainId uint32, oracleAddress common.Address) (bool, error) {
	var out []interface{}
	err := _Genesis.contract.Call(opts, &out, "isOracle", chainId, oracleAddress)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOracle is a free data retrieval call binding the contract method 0x0aa4bac2.
//
// Solidity: function isOracle(uint32 chainId, address oracleAddress) view returns(bool)
func (_Genesis *GenesisSession) IsOracle(chainId uint32, oracleAddress common.Address) (bool, error) {
	return _Genesis.Contract.IsOracle(&_Genesis.CallOpts, chainId, oracleAddress)
}

// IsOracle is a free data retrieval call binding the contract method 0x0aa4bac2.
//
// Solidity: function isOracle(uint32 chainId, address oracleAddress) view returns(bool)
func (_Genesis *GenesisCallerSession) IsOracle(chainId uint32, oracleAddress common.Address) (bool, error) {
	return _Genesis.Contract.IsOracle(&_Genesis.CallOpts, chainId, oracleAddress)
}

// IsValidator is a free data retrieval call binding the contract method 0x74af92a2.
//
// Solidity: function isValidator(uint32 chainId, bytes validatorPublicKey) view returns(bool)
func (_Genesis *GenesisCaller) IsValidator(opts *bind.CallOpts, chainId uint32, validatorPublicKey []byte) (bool, error) {
	var out []interface{}
	err := _Genesis.contract.Call(opts, &out, "isValidator", chainId, validatorPublicKey)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidator is a free data retrieval call binding the contract method 0x74af92a2.
//
// Solidity: function isValidator(uint32 chainId, bytes validatorPublicKey) view returns(bool)
func (_Genesis *GenesisSession) IsValidator(chainId uint32, validatorPublicKey []byte) (bool, error) {
	return _Genesis.Contract.IsValidator(&_Genesis.CallOpts, chainId, validatorPublicKey)
}

// IsValidator is a free data retrieval call binding the contract method 0x74af92a2.
//
// Solidity: function isValidator(uint32 chainId, bytes validatorPublicKey) view returns(bool)
func (_Genesis *GenesisCallerSession) IsValidator(chainId uint32, validatorPublicKey []byte) (bool, error) {
	return _Genesis.Contract.IsValidator(&_Genesis.CallOpts, chainId, validatorPublicKey)
}

// AddOracle is a paid mutator transaction binding the contract method 0xbb99d77a.
//
// Solidity: function addOracle(uint32 chainId, address oracleAddress) returns()
func (_Genesis *GenesisTransactor) AddOracle(opts *bind.TransactOpts, chainId uint32, oracleAddress common.Address) (*types.Transaction, error) {
	return _Genesis.contract.Transact(opts, "addOracle", chainId, oracleAddress)
}

// AddOracle is a paid mutator transaction binding the contract method 0xbb99d77a.
//
// Solidity: function addOracle(uint32 chainId, address oracleAddress) returns()
func (_Genesis *GenesisSession) AddOracle(chainId uint32, oracleAddress common.Address) (*types.Transaction, error) {
	return _Genesis.Contract.AddOracle(&_Genesis.TransactOpts, chainId, oracleAddress)
}

// AddOracle is a paid mutator transaction binding the contract method 0xbb99d77a.
//
// Solidity: function addOracle(uint32 chainId, address oracleAddress) returns()
func (_Genesis *GenesisTransactorSession) AddOracle(chainId uint32, oracleAddress common.Address) (*types.Transaction, error) {
	return _Genesis.Contract.AddOracle(&_Genesis.TransactOpts, chainId, oracleAddress)
}

// AddValidator is a paid mutator transaction binding the contract method 0x60d6c264.
//
// Solidity: function addValidator(uint32 chainId, bytes validatorPublicKey) returns()
func (_Genesis *GenesisTransactor) AddValidator(opts *bind.TransactOpts, chainId uint32, validatorPublicKey []byte) (*types.Transaction, error) {
	return _Genesis.contract.Transact(opts, "addValidator", chainId, validatorPublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0x60d6c264.
//
// Solidity: function addValidator(uint32 chainId, bytes validatorPublicKey) returns()
func (_Genesis *GenesisSession) AddValidator(chainId uint32, validatorPublicKey []byte) (*types.Transaction, error) {
	return _Genesis.Contract.AddValidator(&_Genesis.TransactOpts, chainId, validatorPublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0x60d6c264.
//
// Solidity: function addValidator(uint32 chainId, bytes validatorPublicKey) returns()
func (_Genesis *GenesisTransactorSession) AddValidator(chainId uint32, validatorPublicKey []byte) (*types.Transaction, error) {
	return _Genesis.Contract.AddValidator(&_Genesis.TransactOpts, chainId, validatorPublicKey)
}

// NewChain is a paid mutator transaction binding the contract method 0xd4f52152.
//
// Solidity: function newChain(string genesis, bytes[] validatorList, address[] oracleList) returns(uint32 newChainId)
func (_Genesis *GenesisTransactor) NewChain(opts *bind.TransactOpts, genesis string, validatorList [][]byte, oracleList []common.Address) (*types.Transaction, error) {
	return _Genesis.contract.Transact(opts, "newChain", genesis, validatorList, oracleList)
}

// NewChain is a paid mutator transaction binding the contract method 0xd4f52152.
//
// Solidity: function newChain(string genesis, bytes[] validatorList, address[] oracleList) returns(uint32 newChainId)
func (_Genesis *GenesisSession) NewChain(genesis string, validatorList [][]byte, oracleList []common.Address) (*types.Transaction, error) {
	return _Genesis.Contract.NewChain(&_Genesis.TransactOpts, genesis, validatorList, oracleList)
}

// NewChain is a paid mutator transaction binding the contract method 0xd4f52152.
//
// Solidity: function newChain(string genesis, bytes[] validatorList, address[] oracleList) returns(uint32 newChainId)
func (_Genesis *GenesisTransactorSession) NewChain(genesis string, validatorList [][]byte, oracleList []common.Address) (*types.Transaction, error) {
	return _Genesis.Contract.NewChain(&_Genesis.TransactOpts, genesis, validatorList, oracleList)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x003fcf71.
//
// Solidity: function removeOracle(uint32 chainId, uint256 idx, address oracleAddress) returns()
func (_Genesis *GenesisTransactor) RemoveOracle(opts *bind.TransactOpts, chainId uint32, idx *big.Int, oracleAddress common.Address) (*types.Transaction, error) {
	return _Genesis.contract.Transact(opts, "removeOracle", chainId, idx, oracleAddress)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x003fcf71.
//
// Solidity: function removeOracle(uint32 chainId, uint256 idx, address oracleAddress) returns()
func (_Genesis *GenesisSession) RemoveOracle(chainId uint32, idx *big.Int, oracleAddress common.Address) (*types.Transaction, error) {
	return _Genesis.Contract.RemoveOracle(&_Genesis.TransactOpts, chainId, idx, oracleAddress)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x003fcf71.
//
// Solidity: function removeOracle(uint32 chainId, uint256 idx, address oracleAddress) returns()
func (_Genesis *GenesisTransactorSession) RemoveOracle(chainId uint32, idx *big.Int, oracleAddress common.Address) (*types.Transaction, error) {
	return _Genesis.Contract.RemoveOracle(&_Genesis.TransactOpts, chainId, idx, oracleAddress)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x9c8ea8cc.
//
// Solidity: function removeValidator(uint32 chainId, uint256 idx, bytes validatorPublicKey) returns()
func (_Genesis *GenesisTransactor) RemoveValidator(opts *bind.TransactOpts, chainId uint32, idx *big.Int, validatorPublicKey []byte) (*types.Transaction, error) {
	return _Genesis.contract.Transact(opts, "removeValidator", chainId, idx, validatorPublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x9c8ea8cc.
//
// Solidity: function removeValidator(uint32 chainId, uint256 idx, bytes validatorPublicKey) returns()
func (_Genesis *GenesisSession) RemoveValidator(chainId uint32, idx *big.Int, validatorPublicKey []byte) (*types.Transaction, error) {
	return _Genesis.Contract.RemoveValidator(&_Genesis.TransactOpts, chainId, idx, validatorPublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x9c8ea8cc.
//
// Solidity: function removeValidator(uint32 chainId, uint256 idx, bytes validatorPublicKey) returns()
func (_Genesis *GenesisTransactorSession) RemoveValidator(chainId uint32, idx *big.Int, validatorPublicKey []byte) (*types.Transaction, error) {
	return _Genesis.Contract.RemoveValidator(&_Genesis.TransactOpts, chainId, idx, validatorPublicKey)
}

// SetGenesis is a paid mutator transaction binding the contract method 0xd32954a5.
//
// Solidity: function setGenesis(uint32 chainId, string newGenesis) returns()
func (_Genesis *GenesisTransactor) SetGenesis(opts *bind.TransactOpts, chainId uint32, newGenesis string) (*types.Transaction, error) {
	return _Genesis.contract.Transact(opts, "setGenesis", chainId, newGenesis)
}

// SetGenesis is a paid mutator transaction binding the contract method 0xd32954a5.
//
// Solidity: function setGenesis(uint32 chainId, string newGenesis) returns()
func (_Genesis *GenesisSession) SetGenesis(chainId uint32, newGenesis string) (*types.Transaction, error) {
	return _Genesis.Contract.SetGenesis(&_Genesis.TransactOpts, chainId, newGenesis)
}

// SetGenesis is a paid mutator transaction binding the contract method 0xd32954a5.
//
// Solidity: function setGenesis(uint32 chainId, string newGenesis) returns()
func (_Genesis *GenesisTransactorSession) SetGenesis(chainId uint32, newGenesis string) (*types.Transaction, error) {
	return _Genesis.Contract.SetGenesis(&_Genesis.TransactOpts, chainId, newGenesis)
}

// GenesisChainRegisteredIterator is returned from FilterChainRegistered and is used to iterate over the raw logs and unpacked data for ChainRegistered events raised by the Genesis contract.
type GenesisChainRegisteredIterator struct {
	Event *GenesisChainRegistered // Event containing the contract specifics and raw log

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
func (it *GenesisChainRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GenesisChainRegistered)
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
		it.Event = new(GenesisChainRegistered)
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
func (it *GenesisChainRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GenesisChainRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GenesisChainRegistered represents a ChainRegistered event raised by the Genesis contract.
type GenesisChainRegistered struct {
	ChainId uint32
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterChainRegistered is a free log retrieval operation binding the contract event 0xced3baa88aa65d52234f5717c8b053dc44bb9df530b1f6784809640ed322b7e9.
//
// Solidity: event ChainRegistered(uint32 chainId)
func (_Genesis *GenesisFilterer) FilterChainRegistered(opts *bind.FilterOpts) (*GenesisChainRegisteredIterator, error) {

	logs, sub, err := _Genesis.contract.FilterLogs(opts, "ChainRegistered")
	if err != nil {
		return nil, err
	}
	return &GenesisChainRegisteredIterator{contract: _Genesis.contract, event: "ChainRegistered", logs: logs, sub: sub}, nil
}

// WatchChainRegistered is a free log subscription operation binding the contract event 0xced3baa88aa65d52234f5717c8b053dc44bb9df530b1f6784809640ed322b7e9.
//
// Solidity: event ChainRegistered(uint32 chainId)
func (_Genesis *GenesisFilterer) WatchChainRegistered(opts *bind.WatchOpts, sink chan<- *GenesisChainRegistered) (event.Subscription, error) {

	logs, sub, err := _Genesis.contract.WatchLogs(opts, "ChainRegistered")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GenesisChainRegistered)
				if err := _Genesis.contract.UnpackLog(event, "ChainRegistered", log); err != nil {
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

// ParseChainRegistered is a log parse operation binding the contract event 0xced3baa88aa65d52234f5717c8b053dc44bb9df530b1f6784809640ed322b7e9.
//
// Solidity: event ChainRegistered(uint32 chainId)
func (_Genesis *GenesisFilterer) ParseChainRegistered(log types.Log) (*GenesisChainRegistered, error) {
	event := new(GenesisChainRegistered)
	if err := _Genesis.contract.UnpackLog(event, "ChainRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// GenesisGenesisUpdatedIterator is returned from FilterGenesisUpdated and is used to iterate over the raw logs and unpacked data for GenesisUpdated events raised by the Genesis contract.
type GenesisGenesisUpdatedIterator struct {
	Event *GenesisGenesisUpdated // Event containing the contract specifics and raw log

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
func (it *GenesisGenesisUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GenesisGenesisUpdated)
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
		it.Event = new(GenesisGenesisUpdated)
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
func (it *GenesisGenesisUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GenesisGenesisUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GenesisGenesisUpdated represents a GenesisUpdated event raised by the Genesis contract.
type GenesisGenesisUpdated struct {
	ChainId uint32
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterGenesisUpdated is a free log retrieval operation binding the contract event 0x87f64dce9746fc7da2e672b4aacc82ad148ed5900411894ddcbe532618fa89fb.
//
// Solidity: event GenesisUpdated(uint32 chainId)
func (_Genesis *GenesisFilterer) FilterGenesisUpdated(opts *bind.FilterOpts) (*GenesisGenesisUpdatedIterator, error) {

	logs, sub, err := _Genesis.contract.FilterLogs(opts, "GenesisUpdated")
	if err != nil {
		return nil, err
	}
	return &GenesisGenesisUpdatedIterator{contract: _Genesis.contract, event: "GenesisUpdated", logs: logs, sub: sub}, nil
}

// WatchGenesisUpdated is a free log subscription operation binding the contract event 0x87f64dce9746fc7da2e672b4aacc82ad148ed5900411894ddcbe532618fa89fb.
//
// Solidity: event GenesisUpdated(uint32 chainId)
func (_Genesis *GenesisFilterer) WatchGenesisUpdated(opts *bind.WatchOpts, sink chan<- *GenesisGenesisUpdated) (event.Subscription, error) {

	logs, sub, err := _Genesis.contract.WatchLogs(opts, "GenesisUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GenesisGenesisUpdated)
				if err := _Genesis.contract.UnpackLog(event, "GenesisUpdated", log); err != nil {
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

// ParseGenesisUpdated is a log parse operation binding the contract event 0x87f64dce9746fc7da2e672b4aacc82ad148ed5900411894ddcbe532618fa89fb.
//
// Solidity: event GenesisUpdated(uint32 chainId)
func (_Genesis *GenesisFilterer) ParseGenesisUpdated(log types.Log) (*GenesisGenesisUpdated, error) {
	event := new(GenesisGenesisUpdated)
	if err := _Genesis.contract.UnpackLog(event, "GenesisUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// GenesisOracleAddedIterator is returned from FilterOracleAdded and is used to iterate over the raw logs and unpacked data for OracleAdded events raised by the Genesis contract.
type GenesisOracleAddedIterator struct {
	Event *GenesisOracleAdded // Event containing the contract specifics and raw log

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
func (it *GenesisOracleAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GenesisOracleAdded)
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
		it.Event = new(GenesisOracleAdded)
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
func (it *GenesisOracleAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GenesisOracleAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GenesisOracleAdded represents a OracleAdded event raised by the Genesis contract.
type GenesisOracleAdded struct {
	ChainId       uint32
	OracleAddress common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOracleAdded is a free log retrieval operation binding the contract event 0x572d29453222865ac78ef8d936cb20aba9368900deb6db64997c1482fe2b30c9.
//
// Solidity: event OracleAdded(uint32 chainId, address oracleAddress)
func (_Genesis *GenesisFilterer) FilterOracleAdded(opts *bind.FilterOpts) (*GenesisOracleAddedIterator, error) {

	logs, sub, err := _Genesis.contract.FilterLogs(opts, "OracleAdded")
	if err != nil {
		return nil, err
	}
	return &GenesisOracleAddedIterator{contract: _Genesis.contract, event: "OracleAdded", logs: logs, sub: sub}, nil
}

// WatchOracleAdded is a free log subscription operation binding the contract event 0x572d29453222865ac78ef8d936cb20aba9368900deb6db64997c1482fe2b30c9.
//
// Solidity: event OracleAdded(uint32 chainId, address oracleAddress)
func (_Genesis *GenesisFilterer) WatchOracleAdded(opts *bind.WatchOpts, sink chan<- *GenesisOracleAdded) (event.Subscription, error) {

	logs, sub, err := _Genesis.contract.WatchLogs(opts, "OracleAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GenesisOracleAdded)
				if err := _Genesis.contract.UnpackLog(event, "OracleAdded", log); err != nil {
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

// ParseOracleAdded is a log parse operation binding the contract event 0x572d29453222865ac78ef8d936cb20aba9368900deb6db64997c1482fe2b30c9.
//
// Solidity: event OracleAdded(uint32 chainId, address oracleAddress)
func (_Genesis *GenesisFilterer) ParseOracleAdded(log types.Log) (*GenesisOracleAdded, error) {
	event := new(GenesisOracleAdded)
	if err := _Genesis.contract.UnpackLog(event, "OracleAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// GenesisOracleRemovedIterator is returned from FilterOracleRemoved and is used to iterate over the raw logs and unpacked data for OracleRemoved events raised by the Genesis contract.
type GenesisOracleRemovedIterator struct {
	Event *GenesisOracleRemoved // Event containing the contract specifics and raw log

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
func (it *GenesisOracleRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GenesisOracleRemoved)
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
		it.Event = new(GenesisOracleRemoved)
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
func (it *GenesisOracleRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GenesisOracleRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GenesisOracleRemoved represents a OracleRemoved event raised by the Genesis contract.
type GenesisOracleRemoved struct {
	ChainId       uint32
	OracleAddress common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOracleRemoved is a free log retrieval operation binding the contract event 0x4b8171862540a056f44154d626c1390fed806ee806026247dc8c19f47b09acbe.
//
// Solidity: event OracleRemoved(uint32 chainId, address oracleAddress)
func (_Genesis *GenesisFilterer) FilterOracleRemoved(opts *bind.FilterOpts) (*GenesisOracleRemovedIterator, error) {

	logs, sub, err := _Genesis.contract.FilterLogs(opts, "OracleRemoved")
	if err != nil {
		return nil, err
	}
	return &GenesisOracleRemovedIterator{contract: _Genesis.contract, event: "OracleRemoved", logs: logs, sub: sub}, nil
}

// WatchOracleRemoved is a free log subscription operation binding the contract event 0x4b8171862540a056f44154d626c1390fed806ee806026247dc8c19f47b09acbe.
//
// Solidity: event OracleRemoved(uint32 chainId, address oracleAddress)
func (_Genesis *GenesisFilterer) WatchOracleRemoved(opts *bind.WatchOpts, sink chan<- *GenesisOracleRemoved) (event.Subscription, error) {

	logs, sub, err := _Genesis.contract.WatchLogs(opts, "OracleRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GenesisOracleRemoved)
				if err := _Genesis.contract.UnpackLog(event, "OracleRemoved", log); err != nil {
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

// ParseOracleRemoved is a log parse operation binding the contract event 0x4b8171862540a056f44154d626c1390fed806ee806026247dc8c19f47b09acbe.
//
// Solidity: event OracleRemoved(uint32 chainId, address oracleAddress)
func (_Genesis *GenesisFilterer) ParseOracleRemoved(log types.Log) (*GenesisOracleRemoved, error) {
	event := new(GenesisOracleRemoved)
	if err := _Genesis.contract.UnpackLog(event, "OracleRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// GenesisValidatorAddedIterator is returned from FilterValidatorAdded and is used to iterate over the raw logs and unpacked data for ValidatorAdded events raised by the Genesis contract.
type GenesisValidatorAddedIterator struct {
	Event *GenesisValidatorAdded // Event containing the contract specifics and raw log

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
func (it *GenesisValidatorAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GenesisValidatorAdded)
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
		it.Event = new(GenesisValidatorAdded)
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
func (it *GenesisValidatorAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GenesisValidatorAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GenesisValidatorAdded represents a ValidatorAdded event raised by the Genesis contract.
type GenesisValidatorAdded struct {
	ChainId            uint32
	ValidatorPublicKey []byte
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterValidatorAdded is a free log retrieval operation binding the contract event 0x3d8adf342e55e97b1f85be8e952d2b473ec50bb2004821559c2b440f0a589e4e.
//
// Solidity: event ValidatorAdded(uint32 chainId, bytes validatorPublicKey)
func (_Genesis *GenesisFilterer) FilterValidatorAdded(opts *bind.FilterOpts) (*GenesisValidatorAddedIterator, error) {

	logs, sub, err := _Genesis.contract.FilterLogs(opts, "ValidatorAdded")
	if err != nil {
		return nil, err
	}
	return &GenesisValidatorAddedIterator{contract: _Genesis.contract, event: "ValidatorAdded", logs: logs, sub: sub}, nil
}

// WatchValidatorAdded is a free log subscription operation binding the contract event 0x3d8adf342e55e97b1f85be8e952d2b473ec50bb2004821559c2b440f0a589e4e.
//
// Solidity: event ValidatorAdded(uint32 chainId, bytes validatorPublicKey)
func (_Genesis *GenesisFilterer) WatchValidatorAdded(opts *bind.WatchOpts, sink chan<- *GenesisValidatorAdded) (event.Subscription, error) {

	logs, sub, err := _Genesis.contract.WatchLogs(opts, "ValidatorAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GenesisValidatorAdded)
				if err := _Genesis.contract.UnpackLog(event, "ValidatorAdded", log); err != nil {
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

// ParseValidatorAdded is a log parse operation binding the contract event 0x3d8adf342e55e97b1f85be8e952d2b473ec50bb2004821559c2b440f0a589e4e.
//
// Solidity: event ValidatorAdded(uint32 chainId, bytes validatorPublicKey)
func (_Genesis *GenesisFilterer) ParseValidatorAdded(log types.Log) (*GenesisValidatorAdded, error) {
	event := new(GenesisValidatorAdded)
	if err := _Genesis.contract.UnpackLog(event, "ValidatorAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// GenesisValidatorRemovedIterator is returned from FilterValidatorRemoved and is used to iterate over the raw logs and unpacked data for ValidatorRemoved events raised by the Genesis contract.
type GenesisValidatorRemovedIterator struct {
	Event *GenesisValidatorRemoved // Event containing the contract specifics and raw log

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
func (it *GenesisValidatorRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GenesisValidatorRemoved)
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
		it.Event = new(GenesisValidatorRemoved)
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
func (it *GenesisValidatorRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GenesisValidatorRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GenesisValidatorRemoved represents a ValidatorRemoved event raised by the Genesis contract.
type GenesisValidatorRemoved struct {
	ChainId            uint32
	ValidatorPublicKey []byte
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterValidatorRemoved is a free log retrieval operation binding the contract event 0x7436794ad809d5819bbcbd64a94846c7da193b8280de35e7ce59d3b7b4e6bbe1.
//
// Solidity: event ValidatorRemoved(uint32 chainId, bytes validatorPublicKey)
func (_Genesis *GenesisFilterer) FilterValidatorRemoved(opts *bind.FilterOpts) (*GenesisValidatorRemovedIterator, error) {

	logs, sub, err := _Genesis.contract.FilterLogs(opts, "ValidatorRemoved")
	if err != nil {
		return nil, err
	}
	return &GenesisValidatorRemovedIterator{contract: _Genesis.contract, event: "ValidatorRemoved", logs: logs, sub: sub}, nil
}

// WatchValidatorRemoved is a free log subscription operation binding the contract event 0x7436794ad809d5819bbcbd64a94846c7da193b8280de35e7ce59d3b7b4e6bbe1.
//
// Solidity: event ValidatorRemoved(uint32 chainId, bytes validatorPublicKey)
func (_Genesis *GenesisFilterer) WatchValidatorRemoved(opts *bind.WatchOpts, sink chan<- *GenesisValidatorRemoved) (event.Subscription, error) {

	logs, sub, err := _Genesis.contract.WatchLogs(opts, "ValidatorRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GenesisValidatorRemoved)
				if err := _Genesis.contract.UnpackLog(event, "ValidatorRemoved", log); err != nil {
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

// ParseValidatorRemoved is a log parse operation binding the contract event 0x7436794ad809d5819bbcbd64a94846c7da193b8280de35e7ce59d3b7b4e6bbe1.
//
// Solidity: event ValidatorRemoved(uint32 chainId, bytes validatorPublicKey)
func (_Genesis *GenesisFilterer) ParseValidatorRemoved(log types.Log) (*GenesisValidatorRemoved, error) {
	event := new(GenesisValidatorRemoved)
	if err := _Genesis.contract.UnpackLog(event, "ValidatorRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
