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

// TokenStorageProofABI is the input ABI used to generate the binding from.
const TokenStorageProofABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"registrar\",\"type\":\"address\"}],\"name\":\"TokenRegistered\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"getBalanceMappingPosition\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"holder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"getHolderBalanceSlot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"ercTokenAddress\",\"type\":\"address\"}],\"name\":\"isRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"registerToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"tokenAddresses\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"tokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"registered\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// TokenStorageProofBin is the compiled bytecode used for deploying new contracts.
var TokenStorageProofBin = "0x60806040526002805463ffffffff1916905534801561001d57600080fd5b50611aa58061002d6000396000f3fe608060405234801561001057600080fd5b506004361061007d5760003560e01c80639f181b5e1161005b5780639f181b5e146102b8578063c3c5a547146102d9578063e486033914610313578063e5df8b84146103525761007d565b80632815a86a146100825780632fcd10a2146100ba5780634ccdd506146100e6575b600080fd5b6100a86004803603602081101561009857600080fd5b50356001600160a01b031661038b565b60408051918252519081900360200190f35b6100a8600480360360408110156100d057600080fd5b506001600160a01b038135169060200135610467565b6102b6600480360360c08110156100fc57600080fd5b6001600160a01b03823516916020810135916040820135919081019060808101606082013564010000000081111561013357600080fd5b82018360208201111561014557600080fd5b8035906020019184600183028401116401000000008311171561016757600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092959493602081019350359150506401000000008111156101ba57600080fd5b8201836020820111156101cc57600080fd5b803590602001918460018302840111640100000000831117156101ee57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561024157600080fd5b82018360208201111561025357600080fd5b8035906020019184600183028401116401000000008311171561027557600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955061049f945050505050565b005b6102c06107e4565b6040805163ffffffff9092168252519081900360200190f35b6102ff600480360360208110156102ef57600080fd5b50356001600160a01b03166107f0565b604080519115158252519081900360200190f35b6103396004803603602081101561032957600080fd5b50356001600160a01b0316610891565b6040805192835290151560208301528051918290030190f35b61036f6004803603602081101561036857600080fd5b50356108ad565b604080516001600160a01b039092168252519081900360200190f35b60408051808201909152600f81526e494e56414c49445f4144445245535360881b60208201526000906001600160a01b0383166104465760405162461bcd60e51b81526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561040b5781810151838201526020016103f3565b50505050905090810190601f1680156104385780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b50506001600160a01b0381166000908152602081905260409020545b919050565b604080516001600160a01b03939093166020808501919091528382019290925280518084038201815260609093019052815191012090565b6104a8866108d4565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b8152509061051b5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b50610525866107f0565b15604051806040016040528060128152602001711053149150511657d49151d254d51154915160721b8152509061059d5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b50604080516370a0823160e01b8152336004820152905187916000916001600160a01b038416916370a08231916024808301926020929190829003018186803b1580156105e957600080fd5b505afa1580156105fd573d6000803e3d6000fd5b505050506040513d602081101561061357600080fd5b505160408051808201909152601081526f4e4f545f454e4f5547485f46554e445360801b60208201529091508161068b5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b50600061069a898888886108f7565b905060006106aa3386848c610a0e565b9050600081116040518060400160405280601081526020016f4e4f545f454e4f5547485f46554e445360801b815250906107255760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b506001600160a01b038a166000818152602081905260408082206001808201805460ff1916821790558d8255805480820182558185527fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b031916861790556002805463ffffffff19811663ffffffff9182169093011691909117905590519092339290917f487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf09190a35050505050505050505050565b60025463ffffffff1681565b60408051808201909152600f81526e494e56414c49445f4144445245535360881b60208201526000906001600160a01b03831661086e5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b50506001600160a01b031660009081526020819052604090206001015460ff1690565b6000602081905290815260409020805460019091015460ff1682565b600181815481106108ba57fe5b6000918252602090912001546001600160a01b0316905081565b6000806001600160a01b0383166108ef576000915050610462565b50503b151590565b60408051808201909152601781527f424c4f434b484153485f4e4f545f415641494c41424c4500000000000000000060208201526000908440908161097d5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b50604080516bffffffffffffffffffffffff19606089901b16602080830191909152825180830360140181526034909201909252805191012060006109c28684610aed565b905060606109d1868385610bfb565b9050610a016109e76109e2836111ec565b611231565b805160029081106109f457fe5b6020026020010151611347565b9998505050505050505050565b60408051808201909152601881527f554e50524f4345535345445f53544f524147455f524f4f540000000000000000602082015260009083610a915760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b506000610a9e8684610467565b60408051602080820184905282518083038201815291830190925280519101209091506060610ace878784610bfb565b9050610ae1610adc826111ec565b611347565b98975050505050505050565b6000607b8351116040518060400160405280601481526020017324a72b20a624a22fa12627a1a5afa422a0a222a960611b81525090610b6d5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b50818380519060200120146040518060400160405280601481526020017324a72b20a624a22fa12627a1a5afa422a0a222a960611b81525090610bf15760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040b5781810151838201526020016103f3565b505050607b015190565b604080516020808252818301909252606091829190602082018180368337019050509050826020820152610c30816000611375565b90506060610c406109e2876111ec565b9050606060006060610c506119b1565b6000855160001415610ce0577f56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218a14610cc2576040805162461bcd60e51b815260206004820152600f60248201526e2130b21032b6b83a3c90383937b7b360891b604482015290519081900360640190fd5b505060408051600081526020810190915295506111e5945050505050565b60005b86518110156111dc57610d08878281518110610cfb57fe5b60200260200101516114b4565b955080158015610d1e5750855160208701208b14155b15610d67576040805162461bcd60e51b815260206004820152601460248201527318985908199a5c9cdd081c1c9bdbd9881c185c9d60621b604482015290519081900360640190fd5b8015801590610d7e5750610d7a86611522565b8514155b15610dbb576040805162461bcd60e51b81526020600482015260086024820152670c4c2c840d0c2e6d60c31b604482015290519081900360640190fd5b610dd7878281518110610dca57fe5b6020026020010151611231565b9350835160021415610ff95760006060610e0c610e0787600081518110610dfa57fe5b60200260200101516115d6565b611658565b90925090506000610e1e858c84611766565b905080850194508151811015610eb35760018a5103841015610e715760405162461bcd60e51b81526004018080602001828103825260268152602001806119cc6026913960400191505060405180910390fd5b6000805b506040519080825280601f01601f191660200182016040528015610ea0576020820181803683370190505b509b5050505050505050505050506111e5565b8215610f555760018a5103841015610f12576040805162461bcd60e51b815260206004820152601c60248201527f6c656166206d75737420636f6d65206c61737420696e2070726f6f6600000000604482015290519081900360640190fd5b8a51851015610f2357600080610e75565b86600181518110610f3057fe5b60200260200101519550610f43866115d6565b9b5050505050505050505050506111e5565b60018a5103841415610f985760405162461bcd60e51b8152600401808060200182810382526026815260200180611a4a6026913960400191505060405180910390fd5b610fb587600181518110610fa857fe5b60200260200101516117db565b610fd757610fc987600181518110610dfa57fe5b805190602001209750610ff1565b610fe787600181518110610cfb57fe5b8051906020012097505b5050506111d4565b8351601114156111d4578751821461115d57600088838151811061101957fe5b01602001516001939093019260f81c9050601081106110695760405162461bcd60e51b815260040180806020018281038252602e8152602001806119f2602e913960400191505060405180910390fd5b611088858260ff168151811061107b57fe5b6020026020010151611807565b1561110557600188510382146110e5576040805162461bcd60e51b815260206004820152601d60248201527f6c656166206e6f646573206f6e6c79206174206c617374206c6576656c000000604482015290519081900360640190fd5b505060408051600081526020810190915297506111e59650505050505050565b611117858260ff1681518110610fa857fe5b61113b5761112d858260ff1681518110610dfa57fe5b805190602001209550611157565b61114d858260ff1681518110610cfb57fe5b8051906020012095505b506111d4565b600187510381146111b5576040805162461bcd60e51b815260206004820152601760248201527f73686f756c64206265206174206c617374206c6576656c000000000000000000604482015290519081900360640190fd5b6111c584601081518110610dfa57fe5b985050505050505050506111e5565b600101610ce3565b50505050505050505b9392505050565b6111f46119b1565b815161121457506040805180820190915260008082526020820152610462565b506040805180820190915281518152602082810190820152919050565b606061123c826117db565b6112775760405162461bcd60e51b815260040180806020018281038252602a815260200180611a20602a913960400191505060405180910390fd5b60006112828361182a565b90508067ffffffffffffffff8111801561129b57600080fd5b506040519080825280602002602001820160405280156112d557816020015b6112c26119b1565b8152602001906001900390816112ba5790505b50915060006112e78460200151611877565b60208501510190506000805b8381101561133e57611304836118e0565b915060405180604001604052808381526020018481525085828151811061132757fe5b6020908102919091010152918101916001016112f3565b50505050919050565b6000806113578360200151611877565b83516020948501518201519190039093036101000a90920492915050565b6060600083511161138557600080fd5b82516002028083111561139757600080fd5b8290038067ffffffffffffffff811180156113b157600080fd5b506040519080825280601f01601f1916602001820160405280156113dc576020820181803683370190505b5091506000835b8285018110156114a1576002810661144757600486600283048151811061140657fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061142b57fe5b60200101906001600160f81b031916908160001a905350611495565b600086600283048151811061145857fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061147d57fe5b60200101906001600160f81b031916908160001a9053505b600191820191016113e3565b50825181146114ac57fe5b505092915050565b606080826000015167ffffffffffffffff811180156114d257600080fd5b506040519080825280601f01601f1916602001820160405280156114fd576020820181803683370190505b509050600081602001905061151b8460200151828660000151611970565b5092915050565b600060208251101561153b575080516020820120610462565b816040516020018082805190602001908083835b6020831061156e5780518252601f19909201916020918201910161154f565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040528051906020012060405160200180828152602001915050604051602081830303815290604052805190602001209050610462565b606060006115e78360200151611877565b835190915081900360608167ffffffffffffffff8111801561160857600080fd5b506040519080825280601f01601f191660200182016040528015611633576020820181803683370190505b509050600081602001905061164f848760200151018285611970565b50949350505050565b60006060600083511161169a576040805162461bcd60e51b8152602060048201526005602482015264456d70747960d81b604482015290519081900360640190fd5b60006004846000815181106116ab57fe5b60209101015160f81c901c600f1690506000816116ce5750600092506002611750565b81600114156116e35750600092506001611750565b81600214156116f85750600192506002611750565b816003141561170c57506001925082611750565b6040805162461bcd60e51b81526020600482015260146024820152736661696c6564206465636f64696e67205472696560601b604482015290519081900360640190fd5b8361175b8683611375565b935093505050915091565b6000805b835185820110801561177c5750825181105b156117d35782818151811061178d57fe5b602001015160f81c60f81b6001600160f81b03191684868301815181106117b057fe5b01602001516001600160f81b031916146117cb5790506111e5565b60010161176a565b949350505050565b6020810151805160009190821a9060c08210156117fd57600092505050610462565b5060019392505050565b805160009060011461181b57506000610462565b50602001515160001a60801490565b6000806000905060006118408460200151611877565b602085015185519181019250015b8082101561186e5761185f826118e0565b6001909301929091019061184e565b50909392505050565b8051600090811a6080811015611891576000915050610462565b60b88110806118ac575060c081108015906118ac575060f881105b156118bb576001915050610462565b60c08110156118cf5760b519019050610462565b60f519019050610462565b50919050565b8051600090811a60808110156118fa576001915050610462565b60b881101561190e57607e19019050610462565b60c081101561193b5760b78103600184019350806020036101000a845104600182018101935050506118da565b60f881101561194f5760be19019050610462565b60019290920151602083900360f7016101000a900490910160f51901919050565b5b60208110611990578251825260209283019290910190601f1901611971565b915181516020939093036101000a6000190180199091169216919091179052565b60405180604001604052806000815260200160008152509056fe646976657267656e74206e6f6465206d75737420636f6d65206c61737420696e2070726f6f666966206272616e6368206e6f6465206561636820656c656d656e742068617320746f2062652061206e6962626c6543616e6e6f7420636f6e7665727420746f206c6973742061206e6f6e2d6c69737420524c504974656d2e657874656e73696f6e206e6f64652063616e6e6f74206265206174206c617374206c6576656ca2646970667358221220247ef3ccdb1042e58650f7b4851276fce3f5662364a8dc61e04d1e5e3935260b64736f6c634300060c0033"

// DeployTokenStorageProof deploys a new Ethereum contract, binding an instance of TokenStorageProof to it.
func DeployTokenStorageProof(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *TokenStorageProof, error) {
	parsed, err := abi.JSON(strings.NewReader(TokenStorageProofABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}

	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(TokenStorageProofBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &TokenStorageProof{TokenStorageProofCaller: TokenStorageProofCaller{contract: contract}, TokenStorageProofTransactor: TokenStorageProofTransactor{contract: contract}, TokenStorageProofFilterer: TokenStorageProofFilterer{contract: contract}}, nil
}

// TokenStorageProof is an auto generated Go binding around an Ethereum contract.
type TokenStorageProof struct {
	TokenStorageProofCaller     // Read-only binding to the contract
	TokenStorageProofTransactor // Write-only binding to the contract
	TokenStorageProofFilterer   // Log filterer for contract events
}

// TokenStorageProofCaller is an auto generated read-only Go binding around an Ethereum contract.
type TokenStorageProofCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenStorageProofTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TokenStorageProofTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenStorageProofFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TokenStorageProofFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenStorageProofSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TokenStorageProofSession struct {
	Contract     *TokenStorageProof // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// TokenStorageProofCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TokenStorageProofCallerSession struct {
	Contract *TokenStorageProofCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// TokenStorageProofTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TokenStorageProofTransactorSession struct {
	Contract     *TokenStorageProofTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// TokenStorageProofRaw is an auto generated low-level Go binding around an Ethereum contract.
type TokenStorageProofRaw struct {
	Contract *TokenStorageProof // Generic contract binding to access the raw methods on
}

// TokenStorageProofCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TokenStorageProofCallerRaw struct {
	Contract *TokenStorageProofCaller // Generic read-only contract binding to access the raw methods on
}

// TokenStorageProofTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TokenStorageProofTransactorRaw struct {
	Contract *TokenStorageProofTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTokenStorageProof creates a new instance of TokenStorageProof, bound to a specific deployed contract.
func NewTokenStorageProof(address common.Address, backend bind.ContractBackend) (*TokenStorageProof, error) {
	contract, err := bindTokenStorageProof(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TokenStorageProof{TokenStorageProofCaller: TokenStorageProofCaller{contract: contract}, TokenStorageProofTransactor: TokenStorageProofTransactor{contract: contract}, TokenStorageProofFilterer: TokenStorageProofFilterer{contract: contract}}, nil
}

// NewTokenStorageProofCaller creates a new read-only instance of TokenStorageProof, bound to a specific deployed contract.
func NewTokenStorageProofCaller(address common.Address, caller bind.ContractCaller) (*TokenStorageProofCaller, error) {
	contract, err := bindTokenStorageProof(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TokenStorageProofCaller{contract: contract}, nil
}

// NewTokenStorageProofTransactor creates a new write-only instance of TokenStorageProof, bound to a specific deployed contract.
func NewTokenStorageProofTransactor(address common.Address, transactor bind.ContractTransactor) (*TokenStorageProofTransactor, error) {
	contract, err := bindTokenStorageProof(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TokenStorageProofTransactor{contract: contract}, nil
}

// NewTokenStorageProofFilterer creates a new log filterer instance of TokenStorageProof, bound to a specific deployed contract.
func NewTokenStorageProofFilterer(address common.Address, filterer bind.ContractFilterer) (*TokenStorageProofFilterer, error) {
	contract, err := bindTokenStorageProof(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TokenStorageProofFilterer{contract: contract}, nil
}

// bindTokenStorageProof binds a generic wrapper to an already deployed contract.
func bindTokenStorageProof(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TokenStorageProofABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TokenStorageProof *TokenStorageProofRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TokenStorageProof.Contract.TokenStorageProofCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TokenStorageProof *TokenStorageProofRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.TokenStorageProofTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TokenStorageProof *TokenStorageProofRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.TokenStorageProofTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TokenStorageProof *TokenStorageProofCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _TokenStorageProof.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TokenStorageProof *TokenStorageProofTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TokenStorageProof *TokenStorageProofTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.contract.Transact(opts, method, params...)
}

// GetBalanceMappingPosition is a free data retrieval call binding the contract method 0x2815a86a.
//
// Solidity: function getBalanceMappingPosition(address tokenAddress) view returns(uint256)
func (_TokenStorageProof *TokenStorageProofCaller) GetBalanceMappingPosition(opts *bind.CallOpts, tokenAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "getBalanceMappingPosition", tokenAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBalanceMappingPosition is a free data retrieval call binding the contract method 0x2815a86a.
//
// Solidity: function getBalanceMappingPosition(address tokenAddress) view returns(uint256)
func (_TokenStorageProof *TokenStorageProofSession) GetBalanceMappingPosition(tokenAddress common.Address) (*big.Int, error) {
	return _TokenStorageProof.Contract.GetBalanceMappingPosition(&_TokenStorageProof.CallOpts, tokenAddress)
}

// GetBalanceMappingPosition is a free data retrieval call binding the contract method 0x2815a86a.
//
// Solidity: function getBalanceMappingPosition(address tokenAddress) view returns(uint256)
func (_TokenStorageProof *TokenStorageProofCallerSession) GetBalanceMappingPosition(tokenAddress common.Address) (*big.Int, error) {
	return _TokenStorageProof.Contract.GetBalanceMappingPosition(&_TokenStorageProof.CallOpts, tokenAddress)
}

// GetHolderBalanceSlot is a free data retrieval call binding the contract method 0x2fcd10a2.
//
// Solidity: function getHolderBalanceSlot(address holder, uint256 balanceMappingPosition) pure returns(bytes32)
func (_TokenStorageProof *TokenStorageProofCaller) GetHolderBalanceSlot(opts *bind.CallOpts, holder common.Address, balanceMappingPosition *big.Int) ([32]byte, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "getHolderBalanceSlot", holder, balanceMappingPosition)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetHolderBalanceSlot is a free data retrieval call binding the contract method 0x2fcd10a2.
//
// Solidity: function getHolderBalanceSlot(address holder, uint256 balanceMappingPosition) pure returns(bytes32)
func (_TokenStorageProof *TokenStorageProofSession) GetHolderBalanceSlot(holder common.Address, balanceMappingPosition *big.Int) ([32]byte, error) {
	return _TokenStorageProof.Contract.GetHolderBalanceSlot(&_TokenStorageProof.CallOpts, holder, balanceMappingPosition)
}

// GetHolderBalanceSlot is a free data retrieval call binding the contract method 0x2fcd10a2.
//
// Solidity: function getHolderBalanceSlot(address holder, uint256 balanceMappingPosition) pure returns(bytes32)
func (_TokenStorageProof *TokenStorageProofCallerSession) GetHolderBalanceSlot(holder common.Address, balanceMappingPosition *big.Int) ([32]byte, error) {
	return _TokenStorageProof.Contract.GetHolderBalanceSlot(&_TokenStorageProof.CallOpts, holder, balanceMappingPosition)
}

// IsRegistered is a free data retrieval call binding the contract method 0xc3c5a547.
//
// Solidity: function isRegistered(address ercTokenAddress) view returns(bool)
func (_TokenStorageProof *TokenStorageProofCaller) IsRegistered(opts *bind.CallOpts, ercTokenAddress common.Address) (bool, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "isRegistered", ercTokenAddress)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsRegistered is a free data retrieval call binding the contract method 0xc3c5a547.
//
// Solidity: function isRegistered(address ercTokenAddress) view returns(bool)
func (_TokenStorageProof *TokenStorageProofSession) IsRegistered(ercTokenAddress common.Address) (bool, error) {
	return _TokenStorageProof.Contract.IsRegistered(&_TokenStorageProof.CallOpts, ercTokenAddress)
}

// IsRegistered is a free data retrieval call binding the contract method 0xc3c5a547.
//
// Solidity: function isRegistered(address ercTokenAddress) view returns(bool)
func (_TokenStorageProof *TokenStorageProofCallerSession) IsRegistered(ercTokenAddress common.Address) (bool, error) {
	return _TokenStorageProof.Contract.IsRegistered(&_TokenStorageProof.CallOpts, ercTokenAddress)
}

// TokenAddresses is a free data retrieval call binding the contract method 0xe5df8b84.
//
// Solidity: function tokenAddresses(uint256 ) view returns(address)
func (_TokenStorageProof *TokenStorageProofCaller) TokenAddresses(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "tokenAddresses", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TokenAddresses is a free data retrieval call binding the contract method 0xe5df8b84.
//
// Solidity: function tokenAddresses(uint256 ) view returns(address)
func (_TokenStorageProof *TokenStorageProofSession) TokenAddresses(arg0 *big.Int) (common.Address, error) {
	return _TokenStorageProof.Contract.TokenAddresses(&_TokenStorageProof.CallOpts, arg0)
}

// TokenAddresses is a free data retrieval call binding the contract method 0xe5df8b84.
//
// Solidity: function tokenAddresses(uint256 ) view returns(address)
func (_TokenStorageProof *TokenStorageProofCallerSession) TokenAddresses(arg0 *big.Int) (common.Address, error) {
	return _TokenStorageProof.Contract.TokenAddresses(&_TokenStorageProof.CallOpts, arg0)
}

// TokenCount is a free data retrieval call binding the contract method 0x9f181b5e.
//
// Solidity: function tokenCount() view returns(uint32)
func (_TokenStorageProof *TokenStorageProofCaller) TokenCount(opts *bind.CallOpts) (uint32, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "tokenCount")

	if err != nil {
		return *new(uint32), err
	}

	out0 := *abi.ConvertType(out[0], new(uint32)).(*uint32)

	return out0, err

}

// TokenCount is a free data retrieval call binding the contract method 0x9f181b5e.
//
// Solidity: function tokenCount() view returns(uint32)
func (_TokenStorageProof *TokenStorageProofSession) TokenCount() (uint32, error) {
	return _TokenStorageProof.Contract.TokenCount(&_TokenStorageProof.CallOpts)
}

// TokenCount is a free data retrieval call binding the contract method 0x9f181b5e.
//
// Solidity: function tokenCount() view returns(uint32)
func (_TokenStorageProof *TokenStorageProofCallerSession) TokenCount() (uint32, error) {
	return _TokenStorageProof.Contract.TokenCount(&_TokenStorageProof.CallOpts)
}

// Tokens is a free data retrieval call binding the contract method 0xe4860339.
//
// Solidity: function tokens(address ) view returns(uint256 balanceMappingPosition, bool registered)
func (_TokenStorageProof *TokenStorageProofCaller) Tokens(opts *bind.CallOpts, arg0 common.Address) (struct {
	BalanceMappingPosition *big.Int
	Registered             bool
}, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "tokens", arg0)

	outstruct := new(struct {
		BalanceMappingPosition *big.Int
		Registered             bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.BalanceMappingPosition = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Registered = *abi.ConvertType(out[1], new(bool)).(*bool)

	return *outstruct, err

}

// Tokens is a free data retrieval call binding the contract method 0xe4860339.
//
// Solidity: function tokens(address ) view returns(uint256 balanceMappingPosition, bool registered)
func (_TokenStorageProof *TokenStorageProofSession) Tokens(arg0 common.Address) (struct {
	BalanceMappingPosition *big.Int
	Registered             bool
}, error) {
	return _TokenStorageProof.Contract.Tokens(&_TokenStorageProof.CallOpts, arg0)
}

// Tokens is a free data retrieval call binding the contract method 0xe4860339.
//
// Solidity: function tokens(address ) view returns(uint256 balanceMappingPosition, bool registered)
func (_TokenStorageProof *TokenStorageProofCallerSession) Tokens(arg0 common.Address) (struct {
	BalanceMappingPosition *big.Int
	Registered             bool
}, error) {
	return _TokenStorageProof.Contract.Tokens(&_TokenStorageProof.CallOpts, arg0)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x4ccdd506.
//
// Solidity: function registerToken(address tokenAddress, uint256 balanceMappingPosition, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof) returns()
func (_TokenStorageProof *TokenStorageProofTransactor) RegisterToken(opts *bind.TransactOpts, tokenAddress common.Address, balanceMappingPosition *big.Int, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte) (*types.Transaction, error) {
	return _TokenStorageProof.contract.Transact(opts, "registerToken", tokenAddress, balanceMappingPosition, blockNumber, blockHeaderRLP, accountStateProof, storageProof)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x4ccdd506.
//
// Solidity: function registerToken(address tokenAddress, uint256 balanceMappingPosition, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof) returns()
func (_TokenStorageProof *TokenStorageProofSession) RegisterToken(tokenAddress common.Address, balanceMappingPosition *big.Int, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.RegisterToken(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition, blockNumber, blockHeaderRLP, accountStateProof, storageProof)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x4ccdd506.
//
// Solidity: function registerToken(address tokenAddress, uint256 balanceMappingPosition, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof) returns()
func (_TokenStorageProof *TokenStorageProofTransactorSession) RegisterToken(tokenAddress common.Address, balanceMappingPosition *big.Int, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.RegisterToken(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition, blockNumber, blockHeaderRLP, accountStateProof, storageProof)
}

// TokenStorageProofTokenRegisteredIterator is returned from FilterTokenRegistered and is used to iterate over the raw logs and unpacked data for TokenRegistered events raised by the TokenStorageProof contract.
type TokenStorageProofTokenRegisteredIterator struct {
	Event *TokenStorageProofTokenRegistered // Event containing the contract specifics and raw log

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
func (it *TokenStorageProofTokenRegisteredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStorageProofTokenRegistered)
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
		it.Event = new(TokenStorageProofTokenRegistered)
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
func (it *TokenStorageProofTokenRegisteredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStorageProofTokenRegisteredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStorageProofTokenRegistered represents a TokenRegistered event raised by the TokenStorageProof contract.
type TokenStorageProofTokenRegistered struct {
	Token     common.Address
	Registrar common.Address
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterTokenRegistered is a free log retrieval operation binding the contract event 0x487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf0.
//
// Solidity: event TokenRegistered(address indexed token, address indexed registrar)
func (_TokenStorageProof *TokenStorageProofFilterer) FilterTokenRegistered(opts *bind.FilterOpts, token []common.Address, registrar []common.Address) (*TokenStorageProofTokenRegisteredIterator, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var registrarRule []interface{}
	for _, registrarItem := range registrar {
		registrarRule = append(registrarRule, registrarItem)
	}

	logs, sub, err := _TokenStorageProof.contract.FilterLogs(opts, "TokenRegistered", tokenRule, registrarRule)
	if err != nil {
		return nil, err
	}
	return &TokenStorageProofTokenRegisteredIterator{contract: _TokenStorageProof.contract, event: "TokenRegistered", logs: logs, sub: sub}, nil
}

// WatchTokenRegistered is a free log subscription operation binding the contract event 0x487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf0.
//
// Solidity: event TokenRegistered(address indexed token, address indexed registrar)
func (_TokenStorageProof *TokenStorageProofFilterer) WatchTokenRegistered(opts *bind.WatchOpts, sink chan<- *TokenStorageProofTokenRegistered, token []common.Address, registrar []common.Address) (event.Subscription, error) {

	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}
	var registrarRule []interface{}
	for _, registrarItem := range registrar {
		registrarRule = append(registrarRule, registrarItem)
	}

	logs, sub, err := _TokenStorageProof.contract.WatchLogs(opts, "TokenRegistered", tokenRule, registrarRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStorageProofTokenRegistered)
				if err := _TokenStorageProof.contract.UnpackLog(event, "TokenRegistered", log); err != nil {
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

// ParseTokenRegistered is a log parse operation binding the contract event 0x487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf0.
//
// Solidity: event TokenRegistered(address indexed token, address indexed registrar)
func (_TokenStorageProof *TokenStorageProofFilterer) ParseTokenRegistered(log types.Log) (*TokenStorageProofTokenRegistered, error) {
	event := new(TokenStorageProofTokenRegistered)
	if err := _TokenStorageProof.contract.UnpackLog(event, "TokenRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
