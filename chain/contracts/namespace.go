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

// NamespaceABI is the input ABI used to generate the binding from.
const NamespaceABI = "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"chainId\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"ChainIdUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"genesis\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"GenesisUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"NamespaceUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"OracleAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"OracleRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"validatorPublicKey\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"ValidatorAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"validatorPublicKey\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"ValidatorRemoved\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"addOracle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"string\",\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"addValidator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"}],\"name\":\"getNamespace\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"chainId\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"genesis\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"validators\",\"type\":\"string[]\"},{\"internalType\":\"address[]\",\"name\":\"oracles\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"isOracle\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"string\",\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"isValidator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"uint256\",\"name\":\"idx\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"oracleAddress\",\"type\":\"address\"}],\"name\":\"removeOracle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"uint256\",\"name\":\"idx\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"validatorPublicKey\",\"type\":\"string\"}],\"name\":\"removeValidator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"string\",\"name\":\"newChainId\",\"type\":\"string\"}],\"name\":\"setChainId\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"string\",\"name\":\"newGenesis\",\"type\":\"string\"}],\"name\":\"setGenesis\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint16\",\"name\":\"namespace\",\"type\":\"uint16\"},{\"internalType\":\"string\",\"name\":\"chainId\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"genesis\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"validators\",\"type\":\"string[]\"},{\"internalType\":\"address[]\",\"name\":\"oracles\",\"type\":\"address[]\"}],\"name\":\"setNamespace\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// NamespaceBin is the compiled bytecode used for deploying new Namespace.
var NamespaceBin = "0x608060405234801561001057600080fd5b50600080546001600160a01b0319163317905561172c806100326000396000f3fe608060405234801561001057600080fd5b506004361061009e5760003560e01c8063adb7dbe811610066578063adb7dbe814610104578063bb60b3151461012d578063db246f2114610140578063e0aa333614610153578063e3498a7f146101765761009e565b80631366c808146100a35780634bcc8188146100b857806359cff9a6146100cb57806360f0f670146100de5780639aa16ee9146100f1575b600080fd5b6100b66100b1366004611235565b610189565b005b6100b66100c63660046111fd565b6102e8565b6100b66100d9366004611235565b6103a7565b6100b66100ec366004611235565b610463565b6100b66100ff36600461138e565b61056f565b610117610112366004611235565b61073a565b604051610124919061148a565b60405180910390f35b6100b661013b366004611283565b61083f565b61011761014e3660046111fd565b610913565b6101666101613660046111da565b610990565b6040516101249493929190611495565b6100b66101843660046113cf565b610c10565b6000546001600160a01b031633146101bc5760405162461bcd60e51b81526004016101b39061157f565b60405180910390fd5b61ffff8216600090815260016020818152604092839020820180548451600294821615610100026000190190911693909304601f81018390048302840183019094528383526102639390918301828280156102585780601f1061022d57610100808354040283529160200191610258565b820191906000526020600020905b81548152906001019060200180831161023b57829003601f168201915b505050505082610e3d565b156102805760405162461bcd60e51b81526004016101b3906115fd565b61ffff8216600090815260016020818152604090922083516102aa93919092019190840190610e96565b507f09b915de2907fa8b732e1b8549d1d8748d1f6365789bacd8bfc1c2b13321f1e981836040516102dc929190611559565b60405180910390a15050565b6000546001600160a01b031633146103125760405162461bcd60e51b81526004016101b39061157f565b61031c8282610913565b156103395760405162461bcd60e51b81526004016101b390611622565b61ffff8216600090815260016020818152604080842060030180549384018155845292200180546001600160a01b0319166001600160a01b038416179055517f46046a89d1b1ddc11139d795a177db8e9b123e25c07e8d7b3b537aefc994b6ad906102dc908390859061146d565b6000546001600160a01b031633146103d15760405162461bcd60e51b81526004016101b39061157f565b6103db828261073a565b156103f85760405162461bcd60e51b81526004016101b390611622565b61ffff82166000908152600160208181526040832060020180549283018155835291829020835161043193919092019190840190610e96565b507faa457f0c02f923a1498e47a5c9d4b832e998fcf5b391974fc0c6a946794a813481836040516102dc929190611559565b6000546001600160a01b0316331461048d5760405162461bcd60e51b81526004016101b39061157f565b61ffff821660009081526001602081815260409283902080548451600294821615610100026000190190911693909304601f81018390048302840183019094528383526104fc9390918301828280156102585780601f1061022d57610100808354040283529160200191610258565b156105195760405162461bcd60e51b81526004016101b3906115fd565b61ffff82166000908152600160209081526040909120825161053d92840190610e96565b507fe3d9869f91cf391b3bf911c3a1467e4195d49417ea46a46edc8ffb59edb2faa181836040516102dc929190611559565b6000546001600160a01b031633146105995760405162461bcd60e51b81526004016101b39061157f565b61ffff83166000908152600160205260409020600301548083106105cf5760405162461bcd60e51b81526004016101b3906115aa565b61ffff8416600090815260016020526040902060030180546001600160a01b0384169190859081106105fd57fe5b6000918252602090912001546001600160a01b03161461062f5760405162461bcd60e51b81526004016101b3906115d1565b61ffff841660009081526001602052604090206003018054600019830190811061065557fe5b600091825260208083209091015461ffff871683526001909152604090912060030180546001600160a01b03909216918590811061068f57fe5b600091825260208083209190910180546001600160a01b0319166001600160a01b03949094169390931790925561ffff861681526001909152604090206003018054806106d857fe5b600082815260209020810160001990810180546001600160a01b03191690550190556040517feb7308698004c0bfb1007fb03df3d23b5ec8704e43aaeca3bfce122db656e09f9061072c908490879061146d565b60405180910390a150505050565b6000805b61ffff84166000908152600160205260409020600201548110156108335761ffff84166000908152600160205260409020600201805461081c91908390811061078357fe5b600091825260209182902001805460408051601f60026000196101006001871615020190941693909304928301859004850281018501909152818152928301828280156108115780601f106107e657610100808354040283529160200191610811565b820191906000526020600020905b8154815290600101906020018083116107f457829003601f168201915b505050505084610e3d565b1561082b576001915050610839565b60010161073e565b50600090505b92915050565b6000546001600160a01b031633146108695760405162461bcd60e51b81526004016101b39061157f565b61ffff8516600090815260016020908152604090912085519091610891918391880190610e96565b5083516108a79060018301906020870190610e96565b5082516108bd9060028301906020860190610f14565b5081516108d39060038301906020850190610f6d565b507f06500a9a8bac2497581b3067d4076b05a0485705bdc05a53983cdbb9185fc8f186604051610903919061164b565b60405180910390a1505050505050565b6000805b61ffff84166000908152600160205260409020600301548110156108335761ffff8416600090815260016020526040902060030180546001600160a01b03851691908390811061096357fe5b6000918252602090912001546001600160a01b03161415610988576001915050610839565b600101610917565b61ffff8116600090815260016020818152604092839020805484516002828616156101000260001901909216829004601f8101859004850282018501909652858152606095869586958695949185019391850192600386019291869190830182828015610a3e5780601f10610a1357610100808354040283529160200191610a3e565b820191906000526020600020905b815481529060010190602001808311610a2157829003601f168201915b5050865460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815295995088945092508401905082828015610acc5780601f10610aa157610100808354040283529160200191610acc565b820191906000526020600020905b815481529060010190602001808311610aaf57829003601f168201915b5050505050925081805480602002602001604051908101604052809291908181526020016000905b82821015610b9f5760008481526020908190208301805460408051601f6002600019610100600187161502019094169390930492830185900485028101850190915281815292830182828015610b8b5780601f10610b6057610100808354040283529160200191610b8b565b820191906000526020600020905b815481529060010190602001808311610b6e57829003601f168201915b505050505081526020019060010190610af4565b50505050915080805480602002602001604051908101604052809291908181526020018280548015610bfa57602002820191906000526020600020905b81546001600160a01b03168152600190910190602001808311610bdc575b5050505050905093509350935093509193509193565b6000546001600160a01b03163314610c3a5760405162461bcd60e51b81526004016101b39061157f565b61ffff8316600090815260016020526040902060020154808310610c705760405162461bcd60e51b81526004016101b3906115aa565b61ffff841660009081526001602052604090206002018054610d30919085908110610c9757fe5b600091825260209182902001805460408051601f6002600019610100600187161502019094169390930492830185900485028101850190915281815292830182828015610d255780601f10610cfa57610100808354040283529160200191610d25565b820191906000526020600020905b815481529060010190602001808311610d0857829003601f168201915b505050505083610e3d565b610d4c5760405162461bcd60e51b81526004016101b3906115d1565b61ffff8416600090815260016020526040902060020180546000198301908110610d7257fe5b90600052602060002001600160008661ffff1661ffff1681526020019081526020016000206002018481548110610da557fe5b906000526020600020019080546001816001161561010002031660029004610dce929190610fce565b5061ffff84166000908152600160205260409020600201805480610dee57fe5b600190038181906000526020600020016000610e0a9190611043565b90557f443f0e063aa676cbc61e749911d0c2652869c9ec48c4bb503eed9f19a44c250f828560405161072c929190611559565b600081604051602001610e509190611451565b6040516020818303038152906040528051906020012083604051602001610e779190611451565b6040516020818303038152906040528051906020012014905092915050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10610ed757805160ff1916838001178555610f04565b82800160010185558215610f04579182015b82811115610f04578251825591602001919060010190610ee9565b50610f1092915061108a565b5090565b828054828255906000526020600020908101928215610f61579160200282015b82811115610f615782518051610f51918491602090910190610e96565b5091602001919060010190610f34565b50610f1092915061109f565b828054828255906000526020600020908101928215610fc2579160200282015b82811115610fc257825182546001600160a01b0319166001600160a01b03909116178255602090920191600190910190610f8d565b50610f109291506110bc565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106110075780548555610f04565b82800160010185558215610f0457600052602060002091601f016020900482015b82811115610f04578254825591600101919060010190611028565b50805460018160011615610100020316600290046000825580601f106110695750611087565b601f016020900490600052602060002090810190611087919061108a565b50565b5b80821115610f10576000815560010161108b565b80821115610f105760006110b38282611043565b5060010161109f565b5b80821115610f105780546001600160a01b03191681556001016110bd565b600082601f8301126110eb578081fd5b81356110fe6110f982611681565b61165a565b81815291506020808301908481018184028601820187101561111f57600080fd5b6000805b858110156111535782356001600160a01b0381168114611141578283fd5b85529383019391830191600101611123565b50505050505092915050565b600082601f83011261116f578081fd5b813567ffffffffffffffff811115611185578182fd5b611198601f8201601f191660200161165a565b91508082528360208285010111156111af57600080fd5b8060208401602084013760009082016020015292915050565b803561ffff8116811461083957600080fd5b6000602082840312156111eb578081fd5b81356111f6816116e6565b9392505050565b6000806040838503121561120f578081fd5b823561121a816116e6565b9150602083013561122a816116d1565b809150509250929050565b60008060408385031215611247578182fd5b8235611252816116e6565b9150602083013567ffffffffffffffff81111561126d578182fd5b6112798582860161115f565b9150509250929050565b600080600080600060a0868803121561129a578081fd5b6112a487876111c8565b945060208087013567ffffffffffffffff808211156112c1578384fd5b6112cd8a838b0161115f565b965060408901359150808211156112e2578384fd5b6112ee8a838b0161115f565b95506060890135915080821115611303578384fd5b818901915089601f830112611316578384fd5b81356113246110f982611681565b81815284810190848601875b84811015611359576113478f8984358a010161115f565b84529287019290870190600101611330565b509097505050506080890135925080831115611373578384fd5b5050611381888289016110db565b9150509295509295909350565b6000806000606084860312156113a2578283fd5b83356113ad816116e6565b92506020840135915060408401356113c4816116d1565b809150509250925092565b6000806000606084860312156113e3578283fd5b6113ed85856111c8565b925060208401359150604084013567ffffffffffffffff81111561140f578182fd5b61141b8682870161115f565b9150509250925092565b6000815180845261143d8160208601602086016116a1565b601f01601f19169290920160200192915050565b600082516114638184602087016116a1565b9190910192915050565b6001600160a01b0392909216825261ffff16602082015260400190565b901515815260200190565b6000608082526114a86080830187611425565b6020838203818501526114bb8288611425565b848103604086015286518082529092508183019082810284018301838901865b8381101561150957601f198784030185526114f7838351611425565b948601949250908501906001016114db565b5050868103606088015287518082529084019450915050818601845b8281101561154a5781516001600160a01b031685529383019390830190600101611525565b50929998505050505050505050565b60006040825261156c6040830185611425565b905061ffff831660208301529392505050565b60208082526011908201527037b7363ca1b7b73a3930b1ba27bbb732b960791b604082015260600190565b6020808252600d908201526c092dcecc2d8d2c840d2dcc8caf609b1b604082015260600190565b602080825260129082015271092dcc8caf05ad6caf240dad2e6dac2e8c6d60731b604082015260600190565b6020808252600b908201526a26bab9ba103234b33332b960a91b604082015260600190565b6020808252600f908201526e105b1c9958591e481c1c995cd95b9d608a1b604082015260600190565b61ffff91909116815260200190565b60405181810167ffffffffffffffff8111828210171561167957600080fd5b604052919050565b600067ffffffffffffffff821115611697578081fd5b5060209081020190565b60005b838110156116bc5781810151838201526020016116a4565b838111156116cb576000848401525b50505050565b6001600160a01b038116811461108757600080fd5b61ffff8116811461108757600080fdfea2646970667358221220ee90c3f97aded97e2eab99b77031175e3e5f852cd33e4b57ce19b95a8f48179864736f6c634300060c0033"

// DeployNamespace deploys a new Ethereum contract, binding an instance of Namespace to it.
func DeployNamespace(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Namespace, error) {
	parsed, err := abi.JSON(strings.NewReader(NamespaceABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(NamespaceBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Namespace{NamespaceCaller: NamespaceCaller{contract: contract}, NamespaceTransactor: NamespaceTransactor{contract: contract}, NamespaceFilterer: NamespaceFilterer{contract: contract}}, nil
}

// Namespace is an auto generated Go binding around an Ethereum contract.
type Namespace struct {
	NamespaceCaller     // Read-only binding to the contract
	NamespaceTransactor // Write-only binding to the contract
	NamespaceFilterer   // Log filterer for contract events
}

// NamespaceCaller is an auto generated read-only Go binding around an Ethereum contract.
type NamespaceCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NamespaceTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NamespaceTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NamespaceFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NamespaceFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NamespaceSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NamespaceSession struct {
	Contract     *Namespace        // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NamespaceCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NamespaceCallerSession struct {
	Contract *NamespaceCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts    // Call options to use throughout this session
}

// NamespaceTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NamespaceTransactorSession struct {
	Contract     *NamespaceTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts    // Transaction auth options to use throughout this session
}

// NamespaceRaw is an auto generated low-level Go binding around an Ethereum contract.
type NamespaceRaw struct {
	Contract *Namespace // Generic contract binding to access the raw methods on
}

// NamespaceCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NamespaceCallerRaw struct {
	Contract *NamespaceCaller // Generic read-only contract binding to access the raw methods on
}

// NamespaceTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NamespaceTransactorRaw struct {
	Contract *NamespaceTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNamespace creates a new instance of Namespace, bound to a specific deployed contract.
func NewNamespace(address common.Address, backend bind.ContractBackend) (*Namespace, error) {
	contract, err := bindNamespace(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Namespace{NamespaceCaller: NamespaceCaller{contract: contract}, NamespaceTransactor: NamespaceTransactor{contract: contract}, NamespaceFilterer: NamespaceFilterer{contract: contract}}, nil
}

// NewNamespaceCaller creates a new read-only instance of Namespace, bound to a specific deployed contract.
func NewNamespaceCaller(address common.Address, caller bind.ContractCaller) (*NamespaceCaller, error) {
	contract, err := bindNamespace(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NamespaceCaller{contract: contract}, nil
}

// NewNamespaceTransactor creates a new write-only instance of Namespace, bound to a specific deployed contract.
func NewNamespaceTransactor(address common.Address, transactor bind.ContractTransactor) (*NamespaceTransactor, error) {
	contract, err := bindNamespace(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NamespaceTransactor{contract: contract}, nil
}

// NewNamespaceFilterer creates a new log filterer instance of Namespace, bound to a specific deployed contract.
func NewNamespaceFilterer(address common.Address, filterer bind.ContractFilterer) (*NamespaceFilterer, error) {
	contract, err := bindNamespace(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NamespaceFilterer{contract: contract}, nil
}

// bindNamespace binds a generic wrapper to an already deployed contract.
func bindNamespace(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(NamespaceABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Namespace *NamespaceRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Namespace.Contract.NamespaceCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Namespace *NamespaceRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Namespace.Contract.NamespaceTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Namespace *NamespaceRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Namespace.Contract.NamespaceTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Namespace *NamespaceCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Namespace.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Namespace *NamespaceTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Namespace.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Namespace *NamespaceTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Namespace.Contract.contract.Transact(opts, method, params...)
}

// GetNamespace is a free data retrieval call binding the contract method 0xe0aa3336.
//
// Solidity: function getNamespace(uint16 namespace) view returns(string chainId, string genesis, string[] validators, address[] oracles)
func (_Namespace *NamespaceCaller) GetNamespace(opts *bind.CallOpts, namespace uint16) (struct {
	ChainId    string
	Genesis    string
	Validators []string
	Oracles    []common.Address
}, error) {
	var out []interface{}
	err := _Namespace.contract.Call(opts, &out, "getNamespace", namespace)

	outstruct := new(struct {
		ChainId    string
		Genesis    string
		Validators []string
		Oracles    []common.Address
	})

	outstruct.ChainId = out[0].(string)
	outstruct.Genesis = out[1].(string)
	outstruct.Validators = out[2].([]string)
	outstruct.Oracles = out[3].([]common.Address)

	return *outstruct, err

}

// GetNamespace is a free data retrieval call binding the contract method 0xe0aa3336.
//
// Solidity: function getNamespace(uint16 namespace) view returns(string chainId, string genesis, string[] validators, address[] oracles)
func (_Namespace *NamespaceSession) GetNamespace(namespace uint16) (struct {
	ChainId    string
	Genesis    string
	Validators []string
	Oracles    []common.Address
}, error) {
	return _Namespace.Contract.GetNamespace(&_Namespace.CallOpts, namespace)
}

// GetNamespace is a free data retrieval call binding the contract method 0xe0aa3336.
//
// Solidity: function getNamespace(uint16 namespace) view returns(string chainId, string genesis, string[] validators, address[] oracles)
func (_Namespace *NamespaceCallerSession) GetNamespace(namespace uint16) (struct {
	ChainId    string
	Genesis    string
	Validators []string
	Oracles    []common.Address
}, error) {
	return _Namespace.Contract.GetNamespace(&_Namespace.CallOpts, namespace)
}

// IsOracle is a free data retrieval call binding the contract method 0xdb246f21.
//
// Solidity: function isOracle(uint16 namespace, address oracleAddress) view returns(bool)
func (_Namespace *NamespaceCaller) IsOracle(opts *bind.CallOpts, namespace uint16, oracleAddress common.Address) (bool, error) {
	var out []interface{}
	err := _Namespace.contract.Call(opts, &out, "isOracle", namespace, oracleAddress)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOracle is a free data retrieval call binding the contract method 0xdb246f21.
//
// Solidity: function isOracle(uint16 namespace, address oracleAddress) view returns(bool)
func (_Namespace *NamespaceSession) IsOracle(namespace uint16, oracleAddress common.Address) (bool, error) {
	return _Namespace.Contract.IsOracle(&_Namespace.CallOpts, namespace, oracleAddress)
}

// IsOracle is a free data retrieval call binding the contract method 0xdb246f21.
//
// Solidity: function isOracle(uint16 namespace, address oracleAddress) view returns(bool)
func (_Namespace *NamespaceCallerSession) IsOracle(namespace uint16, oracleAddress common.Address) (bool, error) {
	return _Namespace.Contract.IsOracle(&_Namespace.CallOpts, namespace, oracleAddress)
}

// IsValidator is a free data retrieval call binding the contract method 0xadb7dbe8.
//
// Solidity: function isValidator(uint16 namespace, string validatorPublicKey) view returns(bool)
func (_Namespace *NamespaceCaller) IsValidator(opts *bind.CallOpts, namespace uint16, validatorPublicKey string) (bool, error) {
	var out []interface{}
	err := _Namespace.contract.Call(opts, &out, "isValidator", namespace, validatorPublicKey)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidator is a free data retrieval call binding the contract method 0xadb7dbe8.
//
// Solidity: function isValidator(uint16 namespace, string validatorPublicKey) view returns(bool)
func (_Namespace *NamespaceSession) IsValidator(namespace uint16, validatorPublicKey string) (bool, error) {
	return _Namespace.Contract.IsValidator(&_Namespace.CallOpts, namespace, validatorPublicKey)
}

// IsValidator is a free data retrieval call binding the contract method 0xadb7dbe8.
//
// Solidity: function isValidator(uint16 namespace, string validatorPublicKey) view returns(bool)
func (_Namespace *NamespaceCallerSession) IsValidator(namespace uint16, validatorPublicKey string) (bool, error) {
	return _Namespace.Contract.IsValidator(&_Namespace.CallOpts, namespace, validatorPublicKey)
}

// AddOracle is a paid mutator transaction binding the contract method 0x4bcc8188.
//
// Solidity: function addOracle(uint16 namespace, address oracleAddress) returns()
func (_Namespace *NamespaceTransactor) AddOracle(opts *bind.TransactOpts, namespace uint16, oracleAddress common.Address) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "addOracle", namespace, oracleAddress)
}

// AddOracle is a paid mutator transaction binding the contract method 0x4bcc8188.
//
// Solidity: function addOracle(uint16 namespace, address oracleAddress) returns()
func (_Namespace *NamespaceSession) AddOracle(namespace uint16, oracleAddress common.Address) (*types.Transaction, error) {
	return _Namespace.Contract.AddOracle(&_Namespace.TransactOpts, namespace, oracleAddress)
}

// AddOracle is a paid mutator transaction binding the contract method 0x4bcc8188.
//
// Solidity: function addOracle(uint16 namespace, address oracleAddress) returns()
func (_Namespace *NamespaceTransactorSession) AddOracle(namespace uint16, oracleAddress common.Address) (*types.Transaction, error) {
	return _Namespace.Contract.AddOracle(&_Namespace.TransactOpts, namespace, oracleAddress)
}

// AddValidator is a paid mutator transaction binding the contract method 0x59cff9a6.
//
// Solidity: function addValidator(uint16 namespace, string validatorPublicKey) returns()
func (_Namespace *NamespaceTransactor) AddValidator(opts *bind.TransactOpts, namespace uint16, validatorPublicKey string) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "addValidator", namespace, validatorPublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0x59cff9a6.
//
// Solidity: function addValidator(uint16 namespace, string validatorPublicKey) returns()
func (_Namespace *NamespaceSession) AddValidator(namespace uint16, validatorPublicKey string) (*types.Transaction, error) {
	return _Namespace.Contract.AddValidator(&_Namespace.TransactOpts, namespace, validatorPublicKey)
}

// AddValidator is a paid mutator transaction binding the contract method 0x59cff9a6.
//
// Solidity: function addValidator(uint16 namespace, string validatorPublicKey) returns()
func (_Namespace *NamespaceTransactorSession) AddValidator(namespace uint16, validatorPublicKey string) (*types.Transaction, error) {
	return _Namespace.Contract.AddValidator(&_Namespace.TransactOpts, namespace, validatorPublicKey)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x9aa16ee9.
//
// Solidity: function removeOracle(uint16 namespace, uint256 idx, address oracleAddress) returns()
func (_Namespace *NamespaceTransactor) RemoveOracle(opts *bind.TransactOpts, namespace uint16, idx *big.Int, oracleAddress common.Address) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "removeOracle", namespace, idx, oracleAddress)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x9aa16ee9.
//
// Solidity: function removeOracle(uint16 namespace, uint256 idx, address oracleAddress) returns()
func (_Namespace *NamespaceSession) RemoveOracle(namespace uint16, idx *big.Int, oracleAddress common.Address) (*types.Transaction, error) {
	return _Namespace.Contract.RemoveOracle(&_Namespace.TransactOpts, namespace, idx, oracleAddress)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0x9aa16ee9.
//
// Solidity: function removeOracle(uint16 namespace, uint256 idx, address oracleAddress) returns()
func (_Namespace *NamespaceTransactorSession) RemoveOracle(namespace uint16, idx *big.Int, oracleAddress common.Address) (*types.Transaction, error) {
	return _Namespace.Contract.RemoveOracle(&_Namespace.TransactOpts, namespace, idx, oracleAddress)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0xe3498a7f.
//
// Solidity: function removeValidator(uint16 namespace, uint256 idx, string validatorPublicKey) returns()
func (_Namespace *NamespaceTransactor) RemoveValidator(opts *bind.TransactOpts, namespace uint16, idx *big.Int, validatorPublicKey string) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "removeValidator", namespace, idx, validatorPublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0xe3498a7f.
//
// Solidity: function removeValidator(uint16 namespace, uint256 idx, string validatorPublicKey) returns()
func (_Namespace *NamespaceSession) RemoveValidator(namespace uint16, idx *big.Int, validatorPublicKey string) (*types.Transaction, error) {
	return _Namespace.Contract.RemoveValidator(&_Namespace.TransactOpts, namespace, idx, validatorPublicKey)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0xe3498a7f.
//
// Solidity: function removeValidator(uint16 namespace, uint256 idx, string validatorPublicKey) returns()
func (_Namespace *NamespaceTransactorSession) RemoveValidator(namespace uint16, idx *big.Int, validatorPublicKey string) (*types.Transaction, error) {
	return _Namespace.Contract.RemoveValidator(&_Namespace.TransactOpts, namespace, idx, validatorPublicKey)
}

// SetChainId is a paid mutator transaction binding the contract method 0x60f0f670.
//
// Solidity: function setChainId(uint16 namespace, string newChainId) returns()
func (_Namespace *NamespaceTransactor) SetChainId(opts *bind.TransactOpts, namespace uint16, newChainId string) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "setChainId", namespace, newChainId)
}

// SetChainId is a paid mutator transaction binding the contract method 0x60f0f670.
//
// Solidity: function setChainId(uint16 namespace, string newChainId) returns()
func (_Namespace *NamespaceSession) SetChainId(namespace uint16, newChainId string) (*types.Transaction, error) {
	return _Namespace.Contract.SetChainId(&_Namespace.TransactOpts, namespace, newChainId)
}

// SetChainId is a paid mutator transaction binding the contract method 0x60f0f670.
//
// Solidity: function setChainId(uint16 namespace, string newChainId) returns()
func (_Namespace *NamespaceTransactorSession) SetChainId(namespace uint16, newChainId string) (*types.Transaction, error) {
	return _Namespace.Contract.SetChainId(&_Namespace.TransactOpts, namespace, newChainId)
}

// SetGenesis is a paid mutator transaction binding the contract method 0x1366c808.
//
// Solidity: function setGenesis(uint16 namespace, string newGenesis) returns()
func (_Namespace *NamespaceTransactor) SetGenesis(opts *bind.TransactOpts, namespace uint16, newGenesis string) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "setGenesis", namespace, newGenesis)
}

// SetGenesis is a paid mutator transaction binding the contract method 0x1366c808.
//
// Solidity: function setGenesis(uint16 namespace, string newGenesis) returns()
func (_Namespace *NamespaceSession) SetGenesis(namespace uint16, newGenesis string) (*types.Transaction, error) {
	return _Namespace.Contract.SetGenesis(&_Namespace.TransactOpts, namespace, newGenesis)
}

// SetGenesis is a paid mutator transaction binding the contract method 0x1366c808.
//
// Solidity: function setGenesis(uint16 namespace, string newGenesis) returns()
func (_Namespace *NamespaceTransactorSession) SetGenesis(namespace uint16, newGenesis string) (*types.Transaction, error) {
	return _Namespace.Contract.SetGenesis(&_Namespace.TransactOpts, namespace, newGenesis)
}

// SetNamespace is a paid mutator transaction binding the contract method 0xbb60b315.
//
// Solidity: function setNamespace(uint16 namespace, string chainId, string genesis, string[] validators, address[] oracles) returns()
func (_Namespace *NamespaceTransactor) SetNamespace(opts *bind.TransactOpts, namespace uint16, chainId string, genesis string, validators []string, oracles []common.Address) (*types.Transaction, error) {
	return _Namespace.contract.Transact(opts, "setNamespace", namespace, chainId, genesis, validators, oracles)
}

// SetNamespace is a paid mutator transaction binding the contract method 0xbb60b315.
//
// Solidity: function setNamespace(uint16 namespace, string chainId, string genesis, string[] validators, address[] oracles) returns()
func (_Namespace *NamespaceSession) SetNamespace(namespace uint16, chainId string, genesis string, validators []string, oracles []common.Address) (*types.Transaction, error) {
	return _Namespace.Contract.SetNamespace(&_Namespace.TransactOpts, namespace, chainId, genesis, validators, oracles)
}

// SetNamespace is a paid mutator transaction binding the contract method 0xbb60b315.
//
// Solidity: function setNamespace(uint16 namespace, string chainId, string genesis, string[] validators, address[] oracles) returns()
func (_Namespace *NamespaceTransactorSession) SetNamespace(namespace uint16, chainId string, genesis string, validators []string, oracles []common.Address) (*types.Transaction, error) {
	return _Namespace.Contract.SetNamespace(&_Namespace.TransactOpts, namespace, chainId, genesis, validators, oracles)
}

// NamespaceChainIdUpdatedIterator is returned from FilterChainIdUpdated and is used to iterate over the raw logs and unpacked data for ChainIdUpdated events raised by the Namespace contract.
type NamespaceChainIdUpdatedIterator struct {
	Event *NamespaceChainIdUpdated // Event containing the contract specifics and raw log

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
func (it *NamespaceChainIdUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceChainIdUpdated)
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
		it.Event = new(NamespaceChainIdUpdated)
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
func (it *NamespaceChainIdUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceChainIdUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceChainIdUpdated represents a ChainIdUpdated event raised by the Namespace contract.
type NamespaceChainIdUpdated struct {
	ChainId   string
	Namespace uint16
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterChainIdUpdated is a free log retrieval operation binding the contract event 0xe3d9869f91cf391b3bf911c3a1467e4195d49417ea46a46edc8ffb59edb2faa1.
//
// Solidity: event ChainIdUpdated(string chainId, uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterChainIdUpdated(opts *bind.FilterOpts) (*NamespaceChainIdUpdatedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "ChainIdUpdated")
	if err != nil {
		return nil, err
	}
	return &NamespaceChainIdUpdatedIterator{contract: _Namespace.contract, event: "ChainIdUpdated", logs: logs, sub: sub}, nil
}

// WatchChainIdUpdated is a free log subscription operation binding the contract event 0xe3d9869f91cf391b3bf911c3a1467e4195d49417ea46a46edc8ffb59edb2faa1.
//
// Solidity: event ChainIdUpdated(string chainId, uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchChainIdUpdated(opts *bind.WatchOpts, sink chan<- *NamespaceChainIdUpdated) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "ChainIdUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceChainIdUpdated)
				if err := _Namespace.contract.UnpackLog(event, "ChainIdUpdated", log); err != nil {
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

// ParseChainIdUpdated is a log parse operation binding the contract event 0xe3d9869f91cf391b3bf911c3a1467e4195d49417ea46a46edc8ffb59edb2faa1.
//
// Solidity: event ChainIdUpdated(string chainId, uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseChainIdUpdated(log types.Log) (*NamespaceChainIdUpdated, error) {
	event := new(NamespaceChainIdUpdated)
	if err := _Namespace.contract.UnpackLog(event, "ChainIdUpdated", log); err != nil {
		return nil, err
	}
	return event, nil
}

// NamespaceGenesisUpdatedIterator is returned from FilterGenesisUpdated and is used to iterate over the raw logs and unpacked data for GenesisUpdated events raised by the Namespace contract.
type NamespaceGenesisUpdatedIterator struct {
	Event *NamespaceGenesisUpdated // Event containing the contract specifics and raw log

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
func (it *NamespaceGenesisUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceGenesisUpdated)
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
		it.Event = new(NamespaceGenesisUpdated)
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
func (it *NamespaceGenesisUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceGenesisUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceGenesisUpdated represents a GenesisUpdated event raised by the Namespace contract.
type NamespaceGenesisUpdated struct {
	Genesis   string
	Namespace uint16
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterGenesisUpdated is a free log retrieval operation binding the contract event 0x09b915de2907fa8b732e1b8549d1d8748d1f6365789bacd8bfc1c2b13321f1e9.
//
// Solidity: event GenesisUpdated(string genesis, uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterGenesisUpdated(opts *bind.FilterOpts) (*NamespaceGenesisUpdatedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "GenesisUpdated")
	if err != nil {
		return nil, err
	}
	return &NamespaceGenesisUpdatedIterator{contract: _Namespace.contract, event: "GenesisUpdated", logs: logs, sub: sub}, nil
}

// WatchGenesisUpdated is a free log subscription operation binding the contract event 0x09b915de2907fa8b732e1b8549d1d8748d1f6365789bacd8bfc1c2b13321f1e9.
//
// Solidity: event GenesisUpdated(string genesis, uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchGenesisUpdated(opts *bind.WatchOpts, sink chan<- *NamespaceGenesisUpdated) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "GenesisUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceGenesisUpdated)
				if err := _Namespace.contract.UnpackLog(event, "GenesisUpdated", log); err != nil {
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

// ParseGenesisUpdated is a log parse operation binding the contract event 0x09b915de2907fa8b732e1b8549d1d8748d1f6365789bacd8bfc1c2b13321f1e9.
//
// Solidity: event GenesisUpdated(string genesis, uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseGenesisUpdated(log types.Log) (*NamespaceGenesisUpdated, error) {
	event := new(NamespaceGenesisUpdated)
	if err := _Namespace.contract.UnpackLog(event, "GenesisUpdated", log); err != nil {
		return nil, err
	}
	return event, nil
}

// NamespaceNamespaceUpdatedIterator is returned from FilterNamespaceUpdated and is used to iterate over the raw logs and unpacked data for NamespaceUpdated events raised by the Namespace contract.
type NamespaceNamespaceUpdatedIterator struct {
	Event *NamespaceNamespaceUpdated // Event containing the contract specifics and raw log

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
func (it *NamespaceNamespaceUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceNamespaceUpdated)
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
		it.Event = new(NamespaceNamespaceUpdated)
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
func (it *NamespaceNamespaceUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceNamespaceUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceNamespaceUpdated represents a NamespaceUpdated event raised by the Namespace contract.
type NamespaceNamespaceUpdated struct {
	Namespace uint16
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNamespaceUpdated is a free log retrieval operation binding the contract event 0x06500a9a8bac2497581b3067d4076b05a0485705bdc05a53983cdbb9185fc8f1.
//
// Solidity: event NamespaceUpdated(uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterNamespaceUpdated(opts *bind.FilterOpts) (*NamespaceNamespaceUpdatedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "NamespaceUpdated")
	if err != nil {
		return nil, err
	}
	return &NamespaceNamespaceUpdatedIterator{contract: _Namespace.contract, event: "NamespaceUpdated", logs: logs, sub: sub}, nil
}

// WatchNamespaceUpdated is a free log subscription operation binding the contract event 0x06500a9a8bac2497581b3067d4076b05a0485705bdc05a53983cdbb9185fc8f1.
//
// Solidity: event NamespaceUpdated(uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchNamespaceUpdated(opts *bind.WatchOpts, sink chan<- *NamespaceNamespaceUpdated) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "NamespaceUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceNamespaceUpdated)
				if err := _Namespace.contract.UnpackLog(event, "NamespaceUpdated", log); err != nil {
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

// ParseNamespaceUpdated is a log parse operation binding the contract event 0x06500a9a8bac2497581b3067d4076b05a0485705bdc05a53983cdbb9185fc8f1.
//
// Solidity: event NamespaceUpdated(uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseNamespaceUpdated(log types.Log) (*NamespaceNamespaceUpdated, error) {
	event := new(NamespaceNamespaceUpdated)
	if err := _Namespace.contract.UnpackLog(event, "NamespaceUpdated", log); err != nil {
		return nil, err
	}
	return event, nil
}

// NamespaceOracleAddedIterator is returned from FilterOracleAdded and is used to iterate over the raw logs and unpacked data for OracleAdded events raised by the Namespace contract.
type NamespaceOracleAddedIterator struct {
	Event *NamespaceOracleAdded // Event containing the contract specifics and raw log

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
func (it *NamespaceOracleAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceOracleAdded)
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
		it.Event = new(NamespaceOracleAdded)
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
func (it *NamespaceOracleAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceOracleAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceOracleAdded represents a OracleAdded event raised by the Namespace contract.
type NamespaceOracleAdded struct {
	OracleAddress common.Address
	Namespace     uint16
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOracleAdded is a free log retrieval operation binding the contract event 0x46046a89d1b1ddc11139d795a177db8e9b123e25c07e8d7b3b537aefc994b6ad.
//
// Solidity: event OracleAdded(address oracleAddress, uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterOracleAdded(opts *bind.FilterOpts) (*NamespaceOracleAddedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "OracleAdded")
	if err != nil {
		return nil, err
	}
	return &NamespaceOracleAddedIterator{contract: _Namespace.contract, event: "OracleAdded", logs: logs, sub: sub}, nil
}

// WatchOracleAdded is a free log subscription operation binding the contract event 0x46046a89d1b1ddc11139d795a177db8e9b123e25c07e8d7b3b537aefc994b6ad.
//
// Solidity: event OracleAdded(address oracleAddress, uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchOracleAdded(opts *bind.WatchOpts, sink chan<- *NamespaceOracleAdded) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "OracleAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceOracleAdded)
				if err := _Namespace.contract.UnpackLog(event, "OracleAdded", log); err != nil {
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

// ParseOracleAdded is a log parse operation binding the contract event 0x46046a89d1b1ddc11139d795a177db8e9b123e25c07e8d7b3b537aefc994b6ad.
//
// Solidity: event OracleAdded(address oracleAddress, uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseOracleAdded(log types.Log) (*NamespaceOracleAdded, error) {
	event := new(NamespaceOracleAdded)
	if err := _Namespace.contract.UnpackLog(event, "OracleAdded", log); err != nil {
		return nil, err
	}
	return event, nil
}

// NamespaceOracleRemovedIterator is returned from FilterOracleRemoved and is used to iterate over the raw logs and unpacked data for OracleRemoved events raised by the Namespace contract.
type NamespaceOracleRemovedIterator struct {
	Event *NamespaceOracleRemoved // Event containing the contract specifics and raw log

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
func (it *NamespaceOracleRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceOracleRemoved)
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
		it.Event = new(NamespaceOracleRemoved)
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
func (it *NamespaceOracleRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceOracleRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceOracleRemoved represents a OracleRemoved event raised by the Namespace contract.
type NamespaceOracleRemoved struct {
	OracleAddress common.Address
	Namespace     uint16
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOracleRemoved is a free log retrieval operation binding the contract event 0xeb7308698004c0bfb1007fb03df3d23b5ec8704e43aaeca3bfce122db656e09f.
//
// Solidity: event OracleRemoved(address oracleAddress, uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterOracleRemoved(opts *bind.FilterOpts) (*NamespaceOracleRemovedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "OracleRemoved")
	if err != nil {
		return nil, err
	}
	return &NamespaceOracleRemovedIterator{contract: _Namespace.contract, event: "OracleRemoved", logs: logs, sub: sub}, nil
}

// WatchOracleRemoved is a free log subscription operation binding the contract event 0xeb7308698004c0bfb1007fb03df3d23b5ec8704e43aaeca3bfce122db656e09f.
//
// Solidity: event OracleRemoved(address oracleAddress, uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchOracleRemoved(opts *bind.WatchOpts, sink chan<- *NamespaceOracleRemoved) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "OracleRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceOracleRemoved)
				if err := _Namespace.contract.UnpackLog(event, "OracleRemoved", log); err != nil {
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

// ParseOracleRemoved is a log parse operation binding the contract event 0xeb7308698004c0bfb1007fb03df3d23b5ec8704e43aaeca3bfce122db656e09f.
//
// Solidity: event OracleRemoved(address oracleAddress, uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseOracleRemoved(log types.Log) (*NamespaceOracleRemoved, error) {
	event := new(NamespaceOracleRemoved)
	if err := _Namespace.contract.UnpackLog(event, "OracleRemoved", log); err != nil {
		return nil, err
	}
	return event, nil
}

// NamespaceValidatorAddedIterator is returned from FilterValidatorAdded and is used to iterate over the raw logs and unpacked data for ValidatorAdded events raised by the Namespace contract.
type NamespaceValidatorAddedIterator struct {
	Event *NamespaceValidatorAdded // Event containing the contract specifics and raw log

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
func (it *NamespaceValidatorAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceValidatorAdded)
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
		it.Event = new(NamespaceValidatorAdded)
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
func (it *NamespaceValidatorAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceValidatorAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceValidatorAdded represents a ValidatorAdded event raised by the Namespace contract.
type NamespaceValidatorAdded struct {
	ValidatorPublicKey string
	Namespace          uint16
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterValidatorAdded is a free log retrieval operation binding the contract event 0xaa457f0c02f923a1498e47a5c9d4b832e998fcf5b391974fc0c6a946794a8134.
//
// Solidity: event ValidatorAdded(string validatorPublicKey, uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterValidatorAdded(opts *bind.FilterOpts) (*NamespaceValidatorAddedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "ValidatorAdded")
	if err != nil {
		return nil, err
	}
	return &NamespaceValidatorAddedIterator{contract: _Namespace.contract, event: "ValidatorAdded", logs: logs, sub: sub}, nil
}

// WatchValidatorAdded is a free log subscription operation binding the contract event 0xaa457f0c02f923a1498e47a5c9d4b832e998fcf5b391974fc0c6a946794a8134.
//
// Solidity: event ValidatorAdded(string validatorPublicKey, uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchValidatorAdded(opts *bind.WatchOpts, sink chan<- *NamespaceValidatorAdded) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "ValidatorAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceValidatorAdded)
				if err := _Namespace.contract.UnpackLog(event, "ValidatorAdded", log); err != nil {
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

// ParseValidatorAdded is a log parse operation binding the contract event 0xaa457f0c02f923a1498e47a5c9d4b832e998fcf5b391974fc0c6a946794a8134.
//
// Solidity: event ValidatorAdded(string validatorPublicKey, uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseValidatorAdded(log types.Log) (*NamespaceValidatorAdded, error) {
	event := new(NamespaceValidatorAdded)
	if err := _Namespace.contract.UnpackLog(event, "ValidatorAdded", log); err != nil {
		return nil, err
	}
	return event, nil
}

// NamespaceValidatorRemovedIterator is returned from FilterValidatorRemoved and is used to iterate over the raw logs and unpacked data for ValidatorRemoved events raised by the Namespace contract.
type NamespaceValidatorRemovedIterator struct {
	Event *NamespaceValidatorRemoved // Event containing the contract specifics and raw log

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
func (it *NamespaceValidatorRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NamespaceValidatorRemoved)
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
		it.Event = new(NamespaceValidatorRemoved)
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
func (it *NamespaceValidatorRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NamespaceValidatorRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NamespaceValidatorRemoved represents a ValidatorRemoved event raised by the Namespace contract.
type NamespaceValidatorRemoved struct {
	ValidatorPublicKey string
	Namespace          uint16
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterValidatorRemoved is a free log retrieval operation binding the contract event 0x443f0e063aa676cbc61e749911d0c2652869c9ec48c4bb503eed9f19a44c250f.
//
// Solidity: event ValidatorRemoved(string validatorPublicKey, uint16 namespace)
func (_Namespace *NamespaceFilterer) FilterValidatorRemoved(opts *bind.FilterOpts) (*NamespaceValidatorRemovedIterator, error) {

	logs, sub, err := _Namespace.contract.FilterLogs(opts, "ValidatorRemoved")
	if err != nil {
		return nil, err
	}
	return &NamespaceValidatorRemovedIterator{contract: _Namespace.contract, event: "ValidatorRemoved", logs: logs, sub: sub}, nil
}

// WatchValidatorRemoved is a free log subscription operation binding the contract event 0x443f0e063aa676cbc61e749911d0c2652869c9ec48c4bb503eed9f19a44c250f.
//
// Solidity: event ValidatorRemoved(string validatorPublicKey, uint16 namespace)
func (_Namespace *NamespaceFilterer) WatchValidatorRemoved(opts *bind.WatchOpts, sink chan<- *NamespaceValidatorRemoved) (event.Subscription, error) {

	logs, sub, err := _Namespace.contract.WatchLogs(opts, "ValidatorRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NamespaceValidatorRemoved)
				if err := _Namespace.contract.UnpackLog(event, "ValidatorRemoved", log); err != nil {
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

// ParseValidatorRemoved is a log parse operation binding the contract event 0x443f0e063aa676cbc61e749911d0c2652869c9ec48c4bb503eed9f19a44c250f.
//
// Solidity: event ValidatorRemoved(string validatorPublicKey, uint16 namespace)
func (_Namespace *NamespaceFilterer) ParseValidatorRemoved(log types.Log) (*NamespaceValidatorRemoved, error) {
	event := new(NamespaceValidatorRemoved)
	if err := _Namespace.contract.UnpackLog(event, "ValidatorRemoved", log); err != nil {
		return nil, err
	}
	return event, nil
}
