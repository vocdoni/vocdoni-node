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
const TokenStorageProofABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"BalanceMappingPositionUpdated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"isRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"registerToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"setBalanceMappingPosition\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"setVerifiedBalanceMappingPosition\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"tokenAddresses\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"tokens\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"registered\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"verified\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// TokenStorageProofBin is the compiled bytecode used for deploying new contracts.
var TokenStorageProofBin = "0x608060405234801561001057600080fd5b50611e52806100206000396000f3fe608060405234801561001057600080fd5b506004361061007d5760003560e01c8063ae5d92c81161005b578063ae5d92c81461029a578063c3c5a547146102c6578063e486033914610300578063e5df8b84146103465761007d565b806302ca59941461008257806306f69547146100b05780639f181b5e14610280575b600080fd5b6100ae6004803603604081101561009857600080fd5b506001600160a01b03813516906020013561037f565b005b6100ae600480360360c08110156100c657600080fd5b6001600160a01b0382351691602081013591604082013591908101906080810160608201356401000000008111156100fd57600080fd5b82018360208201111561010f57600080fd5b8035906020019184600183028401116401000000008311171561013157600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561018457600080fd5b82018360208201111561019657600080fd5b803590602001918460018302840111640100000000831117156101b857600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561020b57600080fd5b82018360208201111561021d57600080fd5b8035906020019184600183028401116401000000008311171561023f57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610573945050505050565b610288610829565b60408051918252519081900360200190f35b6100ae600480360360408110156102b057600080fd5b506001600160a01b03813516906020013561082f565b6102ec600480360360208110156102dc57600080fd5b50356001600160a01b0316610ac6565b604080519115158252519081900360200190f35b6103266004803603602081101561031657600080fd5b50356001600160a01b0316610ae8565b604080519315158452911515602084015282820152519081900360600190f35b6103636004803603602081101561035c57600080fd5b5035610b0e565b604080516001600160a01b039092168252519081900360200190f35b8161038a8133610b35565b61039383610c29565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b815250906104435760405162461bcd60e51b81526004018080602001828103825283818151815260200191508051906020019080838360005b838110156104085781810151838201526020016103f0565b50505050905090810190601f1680156104355780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b506001600160a01b0383166000908152602081815260409182902054825180840190935260128352711053149150511657d49151d254d51154915160721b9183019190915260ff16156104d75760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506104e0611d3e565b60018082526040808301948552815480830183557fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b039097166001600160a01b03199097168717905560009586526020868152952082518154969093015115156101000261ff001993151560ff1990971696909617929092169490941781559151919092015550565b8561057e8133610b35565b61058787610c29565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b815250906105fa5760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b03871660009081526020818152604091829020548251808401909352600e83526d1393d517d49151d254d51154915160921b9183019190915260ff166106895760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b03871660009081526020818152604091829020548251808401909352601083526f1053149150511657d59154925192515160821b91830191909152610100900460ff16156107205760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b50600061072f88878787610c4c565b9050600061073f3385848b610d63565b9050600081116040518060400160405280601081526020016f4e4f545f454e4f5547485f46554e445360801b815250906107ba5760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b03891660008181526020818152604091829020805461ff0019166101001781556001018b9055815192835282018a905280517f093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d9281900390910190a1505050505050505050565b60015490565b8161083a8133610b35565b61084383610c29565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b815250906108b65760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b03831660009081526020818152604091829020548251808401909352600e83526d1393d517d49151d254d51154915160921b9183019190915260ff166109455760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b03831660009081526020818152604091829020548251808401909352601083526f1053149150511657d59154925192515160821b91830191909152610100900460ff16156109dc5760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b03831660009081526020818152604091829020600101548251808401909352600a83526953414d455f56414c554560b01b91830191909152831415610a6a5760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506001600160a01b038316600081815260208181526040918290206001018590558151928352820184905280517f093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d9281900390910190a1505050565b6001600160a01b03811660009081526020819052604090205460ff165b919050565b6000602081905290815260409020805460019091015460ff808316926101009004169083565b60018181548110610b1b57fe5b6000918252602090912001546001600160a01b0316905081565b6000826001600160a01b03166370a08231836040518263ffffffff1660e01b815260040180826001600160a01b0316815260200191505060206040518083038186803b158015610b8457600080fd5b505afa158015610b98573d6000803e3d6000fd5b505050506040513d6020811015610bae57600080fd5b505160408051808201909152601081526f4e4f545f454e4f5547485f46554e445360801b60208201529110610c245760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b505050565b6000806001600160a01b038316610c44576000915050610ae3565b50503b151590565b60408051808201909152601781527f424c4f434b484153485f4e4f545f415641494c41424c45000000000000000000602082015260009084409081610cd25760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b50604080516bffffffffffffffffffffffff19606089901b1660208083019190915282518083036014018152603490920190925280519101206000610d178684610e42565b90506060610d26868385610f50565b9050610d56610d3c610d3783611541565b611586565b80516002908110610d4957fe5b602002602001015161169c565b9998505050505050505050565b60408051808201909152601881527f554e50524f4345535345445f53544f524147455f524f4f540000000000000000602082015260009083610de65760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b506000610df386846116ca565b60408051602080820184905282518083038201815291830190925280519101209091506060610e23878784610f50565b9050610e36610e3182611541565b61169c565b98975050505050505050565b6000607b8351116040518060400160405280601481526020017324a72b20a624a22fa12627a1a5afa422a0a222a960611b81525090610ec25760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b50818380519060200120146040518060400160405280601481526020017324a72b20a624a22fa12627a1a5afa422a0a222a960611b81525090610f465760405162461bcd60e51b81526020600482018181528351602484015283519092839260449091019190850190808383600083156104085781810151838201526020016103f0565b505050607b015190565b604080516020808252818301909252606091829190602082018180368337019050509050826020820152610f85816000611702565b90506060610f95610d3787611541565b9050606060006060610fa5611d5e565b6000855160001415611035577f56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218a14611017576040805162461bcd60e51b815260206004820152600f60248201526e2130b21032b6b83a3c90383937b7b360891b604482015290519081900360640190fd5b5050604080516000815260208101909152955061153a945050505050565b60005b86518110156115315761105d87828151811061105057fe5b6020026020010151611841565b9550801580156110735750855160208701208b14155b156110bc576040805162461bcd60e51b815260206004820152601460248201527318985908199a5c9cdd081c1c9bdbd9881c185c9d60621b604482015290519081900360640190fd5b80158015906110d357506110cf866118af565b8514155b15611110576040805162461bcd60e51b81526020600482015260086024820152670c4c2c840d0c2e6d60c31b604482015290519081900360640190fd5b61112c87828151811061111f57fe5b6020026020010151611586565b935083516002141561134e576000606061116161115c8760008151811061114f57fe5b6020026020010151611963565b6119e5565b90925090506000611173858c84611af3565b9050808501945081518110156112085760018a51038410156111c65760405162461bcd60e51b8152600401808060200182810382526026815260200180611d796026913960400191505060405180910390fd5b6000805b506040519080825280601f01601f1916602001820160405280156111f5576020820181803683370190505b509b50505050505050505050505061153a565b82156112aa5760018a5103841015611267576040805162461bcd60e51b815260206004820152601c60248201527f6c656166206d75737420636f6d65206c61737420696e2070726f6f6600000000604482015290519081900360640190fd5b8a51851015611278576000806111ca565b8660018151811061128557fe5b6020026020010151955061129886611963565b9b50505050505050505050505061153a565b60018a51038414156112ed5760405162461bcd60e51b8152600401808060200182810382526026815260200180611df76026913960400191505060405180910390fd5b61130a876001815181106112fd57fe5b6020026020010151611b68565b61132c5761131e8760018151811061114f57fe5b805190602001209750611346565b61133c8760018151811061105057fe5b8051906020012097505b505050611529565b83516011141561152957875182146114b257600088838151811061136e57fe5b01602001516001939093019260f81c9050601081106113be5760405162461bcd60e51b815260040180806020018281038252602e815260200180611d9f602e913960400191505060405180910390fd5b6113dd858260ff16815181106113d057fe5b6020026020010151611b94565b1561145a576001885103821461143a576040805162461bcd60e51b815260206004820152601d60248201527f6c656166206e6f646573206f6e6c79206174206c617374206c6576656c000000604482015290519081900360640190fd5b5050604080516000815260208101909152975061153a9650505050505050565b61146c858260ff16815181106112fd57fe5b61149057611482858260ff168151811061114f57fe5b8051906020012095506114ac565b6114a2858260ff168151811061105057fe5b8051906020012095505b50611529565b6001875103811461150a576040805162461bcd60e51b815260206004820152601760248201527f73686f756c64206265206174206c617374206c6576656c000000000000000000604482015290519081900360640190fd5b61151a8460108151811061114f57fe5b9850505050505050505061153a565b600101611038565b50505050505050505b9392505050565b611549611d5e565b815161156957506040805180820190915260008082526020820152610ae3565b506040805180820190915281518152602082810190820152919050565b606061159182611b68565b6115cc5760405162461bcd60e51b815260040180806020018281038252602a815260200180611dcd602a913960400191505060405180910390fd5b60006115d783611bb7565b90508067ffffffffffffffff811180156115f057600080fd5b5060405190808252806020026020018201604052801561162a57816020015b611617611d5e565b81526020019060019003908161160f5790505b509150600061163c8460200151611c04565b60208501510190506000805b838110156116935761165983611c6d565b915060405180604001604052808381526020018481525085828151811061167c57fe5b602090810291909101015291810191600101611648565b50505050919050565b6000806116ac8360200151611c04565b83516020948501518201519190039093036101000a90920492915050565b604080516001600160a01b03939093166020808501919091528382019290925280518084038201815260609093019052815191012090565b6060600083511161171257600080fd5b82516002028083111561172457600080fd5b8290038067ffffffffffffffff8111801561173e57600080fd5b506040519080825280601f01601f191660200182016040528015611769576020820181803683370190505b5091506000835b82850181101561182e57600281066117d457600486600283048151811061179357fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b8483815181106117b857fe5b60200101906001600160f81b031916908160001a905350611822565b60008660028304815181106117e557fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061180a57fe5b60200101906001600160f81b031916908160001a9053505b60019182019101611770565b508251811461183957fe5b505092915050565b606080826000015167ffffffffffffffff8111801561185f57600080fd5b506040519080825280601f01601f19166020018201604052801561188a576020820181803683370190505b50905060008160200190506118a88460200151828660000151611cfd565b5092915050565b60006020825110156118c8575080516020820120610ae3565b816040516020018082805190602001908083835b602083106118fb5780518252601f1990920191602091820191016118dc565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040528051906020012060405160200180828152602001915050604051602081830303815290604052805190602001209050610ae3565b606060006119748360200151611c04565b835190915081900360608167ffffffffffffffff8111801561199557600080fd5b506040519080825280601f01601f1916602001820160405280156119c0576020820181803683370190505b50905060008160200190506119dc848760200151018285611cfd565b50949350505050565b600060606000835111611a27576040805162461bcd60e51b8152602060048201526005602482015264456d70747960d81b604482015290519081900360640190fd5b6000600484600081518110611a3857fe5b60209101015160f81c901c600f169050600081611a5b5750600092506002611add565b8160011415611a705750600092506001611add565b8160021415611a855750600192506002611add565b8160031415611a9957506001925082611add565b6040805162461bcd60e51b81526020600482015260146024820152736661696c6564206465636f64696e67205472696560601b604482015290519081900360640190fd5b83611ae88683611702565b935093505050915091565b6000805b8351858201108015611b095750825181105b15611b6057828181518110611b1a57fe5b602001015160f81c60f81b6001600160f81b0319168486830181518110611b3d57fe5b01602001516001600160f81b03191614611b5857905061153a565b600101611af7565b949350505050565b6020810151805160009190821a9060c0821015611b8a57600092505050610ae3565b5060019392505050565b8051600090600114611ba857506000610ae3565b50602001515160001a60801490565b600080600090506000611bcd8460200151611c04565b602085015185519181019250015b80821015611bfb57611bec82611c6d565b60019093019290910190611bdb565b50909392505050565b8051600090811a6080811015611c1e576000915050610ae3565b60b8811080611c39575060c08110801590611c39575060f881105b15611c48576001915050610ae3565b60c0811015611c5c5760b519019050610ae3565b60f519019050610ae3565b50919050565b8051600090811a6080811015611c87576001915050610ae3565b60b8811015611c9b57607e19019050610ae3565b60c0811015611cc85760b78103600184019350806020036101000a84510460018201810193505050611c67565b60f8811015611cdc5760be19019050610ae3565b60019290920151602083900360f7016101000a900490910160f51901919050565b5b60208110611d1d578251825260209283019290910190601f1901611cfe565b915181516020939093036101000a6000190180199091169216919091179052565b604080516060810182526000808252602082018190529181019190915290565b60405180604001604052806000815260200160008152509056fe646976657267656e74206e6f6465206d75737420636f6d65206c61737420696e2070726f6f666966206272616e6368206e6f6465206561636820656c656d656e742068617320746f2062652061206e6962626c6543616e6e6f7420636f6e7665727420746f206c6973742061206e6f6e2d6c69737420524c504974656d2e657874656e73696f6e206e6f64652063616e6e6f74206265206174206c617374206c6576656ca264697066735822122056520a52c7137f764906234074ef18561d68688ca9110c159ecedd4068df0e7c64736f6c634300060c0033"

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

// IsRegistered is a free data retrieval call binding the contract method 0xc3c5a547.
//
// Solidity: function isRegistered(address tokenAddress) view returns(bool)
func (_TokenStorageProof *TokenStorageProofCaller) IsRegistered(opts *bind.CallOpts, tokenAddress common.Address) (bool, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "isRegistered", tokenAddress)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsRegistered is a free data retrieval call binding the contract method 0xc3c5a547.
//
// Solidity: function isRegistered(address tokenAddress) view returns(bool)
func (_TokenStorageProof *TokenStorageProofSession) IsRegistered(tokenAddress common.Address) (bool, error) {
	return _TokenStorageProof.Contract.IsRegistered(&_TokenStorageProof.CallOpts, tokenAddress)
}

// IsRegistered is a free data retrieval call binding the contract method 0xc3c5a547.
//
// Solidity: function isRegistered(address tokenAddress) view returns(bool)
func (_TokenStorageProof *TokenStorageProofCallerSession) IsRegistered(tokenAddress common.Address) (bool, error) {
	return _TokenStorageProof.Contract.IsRegistered(&_TokenStorageProof.CallOpts, tokenAddress)
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
// Solidity: function tokenCount() view returns(uint256)
func (_TokenStorageProof *TokenStorageProofCaller) TokenCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "tokenCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TokenCount is a free data retrieval call binding the contract method 0x9f181b5e.
//
// Solidity: function tokenCount() view returns(uint256)
func (_TokenStorageProof *TokenStorageProofSession) TokenCount() (*big.Int, error) {
	return _TokenStorageProof.Contract.TokenCount(&_TokenStorageProof.CallOpts)
}

// TokenCount is a free data retrieval call binding the contract method 0x9f181b5e.
//
// Solidity: function tokenCount() view returns(uint256)
func (_TokenStorageProof *TokenStorageProofCallerSession) TokenCount() (*big.Int, error) {
	return _TokenStorageProof.Contract.TokenCount(&_TokenStorageProof.CallOpts)
}

// Tokens is a free data retrieval call binding the contract method 0xe4860339.
//
// Solidity: function tokens(address ) view returns(bool registered, bool verified, uint256 balanceMappingPosition)
func (_TokenStorageProof *TokenStorageProofCaller) Tokens(opts *bind.CallOpts, arg0 common.Address) (struct {
	Registered             bool
	Verified               bool
	BalanceMappingPosition *big.Int
}, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "tokens", arg0)

	outstruct := new(struct {
		Registered             bool
		Verified               bool
		BalanceMappingPosition *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Registered = *abi.ConvertType(out[0], new(bool)).(*bool)
	outstruct.Verified = *abi.ConvertType(out[1], new(bool)).(*bool)
	outstruct.BalanceMappingPosition = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Tokens is a free data retrieval call binding the contract method 0xe4860339.
//
// Solidity: function tokens(address ) view returns(bool registered, bool verified, uint256 balanceMappingPosition)
func (_TokenStorageProof *TokenStorageProofSession) Tokens(arg0 common.Address) (struct {
	Registered             bool
	Verified               bool
	BalanceMappingPosition *big.Int
}, error) {
	return _TokenStorageProof.Contract.Tokens(&_TokenStorageProof.CallOpts, arg0)
}

// Tokens is a free data retrieval call binding the contract method 0xe4860339.
//
// Solidity: function tokens(address ) view returns(bool registered, bool verified, uint256 balanceMappingPosition)
func (_TokenStorageProof *TokenStorageProofCallerSession) Tokens(arg0 common.Address) (struct {
	Registered             bool
	Verified               bool
	BalanceMappingPosition *big.Int
}, error) {
	return _TokenStorageProof.Contract.Tokens(&_TokenStorageProof.CallOpts, arg0)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x02ca5994.
//
// Solidity: function registerToken(address tokenAddress, uint256 balanceMappingPosition) returns()
func (_TokenStorageProof *TokenStorageProofTransactor) RegisterToken(opts *bind.TransactOpts, tokenAddress common.Address, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.contract.Transact(opts, "registerToken", tokenAddress, balanceMappingPosition)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x02ca5994.
//
// Solidity: function registerToken(address tokenAddress, uint256 balanceMappingPosition) returns()
func (_TokenStorageProof *TokenStorageProofSession) RegisterToken(tokenAddress common.Address, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.RegisterToken(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition)
}

// RegisterToken is a paid mutator transaction binding the contract method 0x02ca5994.
//
// Solidity: function registerToken(address tokenAddress, uint256 balanceMappingPosition) returns()
func (_TokenStorageProof *TokenStorageProofTransactorSession) RegisterToken(tokenAddress common.Address, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.RegisterToken(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition)
}

// SetBalanceMappingPosition is a paid mutator transaction binding the contract method 0xae5d92c8.
//
// Solidity: function setBalanceMappingPosition(address tokenAddress, uint256 balanceMappingPosition) returns()
func (_TokenStorageProof *TokenStorageProofTransactor) SetBalanceMappingPosition(opts *bind.TransactOpts, tokenAddress common.Address, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.contract.Transact(opts, "setBalanceMappingPosition", tokenAddress, balanceMappingPosition)
}

// SetBalanceMappingPosition is a paid mutator transaction binding the contract method 0xae5d92c8.
//
// Solidity: function setBalanceMappingPosition(address tokenAddress, uint256 balanceMappingPosition) returns()
func (_TokenStorageProof *TokenStorageProofSession) SetBalanceMappingPosition(tokenAddress common.Address, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.SetBalanceMappingPosition(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition)
}

// SetBalanceMappingPosition is a paid mutator transaction binding the contract method 0xae5d92c8.
//
// Solidity: function setBalanceMappingPosition(address tokenAddress, uint256 balanceMappingPosition) returns()
func (_TokenStorageProof *TokenStorageProofTransactorSession) SetBalanceMappingPosition(tokenAddress common.Address, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.SetBalanceMappingPosition(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition)
}

// SetVerifiedBalanceMappingPosition is a paid mutator transaction binding the contract method 0x06f69547.
//
// Solidity: function setVerifiedBalanceMappingPosition(address tokenAddress, uint256 balanceMappingPosition, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof) returns()
func (_TokenStorageProof *TokenStorageProofTransactor) SetVerifiedBalanceMappingPosition(opts *bind.TransactOpts, tokenAddress common.Address, balanceMappingPosition *big.Int, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte) (*types.Transaction, error) {
	return _TokenStorageProof.contract.Transact(opts, "setVerifiedBalanceMappingPosition", tokenAddress, balanceMappingPosition, blockNumber, blockHeaderRLP, accountStateProof, storageProof)
}

// SetVerifiedBalanceMappingPosition is a paid mutator transaction binding the contract method 0x06f69547.
//
// Solidity: function setVerifiedBalanceMappingPosition(address tokenAddress, uint256 balanceMappingPosition, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof) returns()
func (_TokenStorageProof *TokenStorageProofSession) SetVerifiedBalanceMappingPosition(tokenAddress common.Address, balanceMappingPosition *big.Int, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.SetVerifiedBalanceMappingPosition(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition, blockNumber, blockHeaderRLP, accountStateProof, storageProof)
}

// SetVerifiedBalanceMappingPosition is a paid mutator transaction binding the contract method 0x06f69547.
//
// Solidity: function setVerifiedBalanceMappingPosition(address tokenAddress, uint256 balanceMappingPosition, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof) returns()
func (_TokenStorageProof *TokenStorageProofTransactorSession) SetVerifiedBalanceMappingPosition(tokenAddress common.Address, balanceMappingPosition *big.Int, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.SetVerifiedBalanceMappingPosition(&_TokenStorageProof.TransactOpts, tokenAddress, balanceMappingPosition, blockNumber, blockHeaderRLP, accountStateProof, storageProof)
}

// TokenStorageProofBalanceMappingPositionUpdatedIterator is returned from FilterBalanceMappingPositionUpdated and is used to iterate over the raw logs and unpacked data for BalanceMappingPositionUpdated events raised by the TokenStorageProof contract.
type TokenStorageProofBalanceMappingPositionUpdatedIterator struct {
	Event *TokenStorageProofBalanceMappingPositionUpdated // Event containing the contract specifics and raw log

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
func (it *TokenStorageProofBalanceMappingPositionUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenStorageProofBalanceMappingPositionUpdated)
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
		it.Event = new(TokenStorageProofBalanceMappingPositionUpdated)
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
func (it *TokenStorageProofBalanceMappingPositionUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenStorageProofBalanceMappingPositionUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenStorageProofBalanceMappingPositionUpdated represents a BalanceMappingPositionUpdated event raised by the TokenStorageProof contract.
type TokenStorageProofBalanceMappingPositionUpdated struct {
	TokenAddress           common.Address
	BalanceMappingPosition *big.Int
	Raw                    types.Log // Blockchain specific contextual infos
}

// FilterBalanceMappingPositionUpdated is a free log retrieval operation binding the contract event 0x093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d.
//
// Solidity: event BalanceMappingPositionUpdated(address tokenAddress, uint256 balanceMappingPosition)
func (_TokenStorageProof *TokenStorageProofFilterer) FilterBalanceMappingPositionUpdated(opts *bind.FilterOpts) (*TokenStorageProofBalanceMappingPositionUpdatedIterator, error) {

	logs, sub, err := _TokenStorageProof.contract.FilterLogs(opts, "BalanceMappingPositionUpdated")
	if err != nil {
		return nil, err
	}
	return &TokenStorageProofBalanceMappingPositionUpdatedIterator{contract: _TokenStorageProof.contract, event: "BalanceMappingPositionUpdated", logs: logs, sub: sub}, nil
}

// WatchBalanceMappingPositionUpdated is a free log subscription operation binding the contract event 0x093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d.
//
// Solidity: event BalanceMappingPositionUpdated(address tokenAddress, uint256 balanceMappingPosition)
func (_TokenStorageProof *TokenStorageProofFilterer) WatchBalanceMappingPositionUpdated(opts *bind.WatchOpts, sink chan<- *TokenStorageProofBalanceMappingPositionUpdated) (event.Subscription, error) {

	logs, sub, err := _TokenStorageProof.contract.WatchLogs(opts, "BalanceMappingPositionUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenStorageProofBalanceMappingPositionUpdated)
				if err := _TokenStorageProof.contract.UnpackLog(event, "BalanceMappingPositionUpdated", log); err != nil {
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

// ParseBalanceMappingPositionUpdated is a log parse operation binding the contract event 0x093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d.
//
// Solidity: event BalanceMappingPositionUpdated(address tokenAddress, uint256 balanceMappingPosition)
func (_TokenStorageProof *TokenStorageProofFilterer) ParseBalanceMappingPositionUpdated(log types.Log) (*TokenStorageProofBalanceMappingPositionUpdated, error) {
	event := new(TokenStorageProofBalanceMappingPositionUpdated)
	if err := _TokenStorageProof.contract.UnpackLog(event, "BalanceMappingPositionUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
