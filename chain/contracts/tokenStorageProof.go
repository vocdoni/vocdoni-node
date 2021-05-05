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
const TokenStorageProofABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"BalanceMappingPositionUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"TokenRegistered\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"isRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"registerToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"setBalanceMappingPosition\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"setVerifiedBalanceMappingPosition\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"tokenAddresses\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"tokens\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"registered\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"verified\",\"type\":\"bool\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// TokenStorageProofBin is the compiled bytecode used for deploying new contracts.
var TokenStorageProofBin = "0x60806040526002805463ffffffff1916905534801561001d57600080fd5b50611eb28061002d6000396000f3fe608060405234801561001057600080fd5b506004361061007d5760003560e01c8063ae5d92c81161005b578063ae5d92c8146102a1578063c3c5a547146102cd578063e486033914610307578063e5df8b841461034d5761007d565b806302ca59941461008257806306f69547146100b05780639f181b5e14610280575b600080fd5b6100ae6004803603604081101561009857600080fd5b506001600160a01b038135169060200135610386565b005b6100ae600480360360c08110156100c657600080fd5b6001600160a01b0382351691602081013591604082013591908101906080810160608201356401000000008111156100fd57600080fd5b82018360208201111561010f57600080fd5b8035906020019184600183028401116401000000008311171561013157600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561018457600080fd5b82018360208201111561019657600080fd5b803590602001918460018302840111640100000000831117156101b857600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561020b57600080fd5b82018360208201111561021d57600080fd5b8035906020019184600183028401116401000000008311171561023f57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295506105cd945050505050565b610288610883565b6040805163ffffffff9092168252519081900360200190f35b6100ae600480360360408110156102b757600080fd5b506001600160a01b03813516906020013561088f565b6102f3600480360360208110156102e357600080fd5b50356001600160a01b0316610b26565b604080519115158252519081900360200190f35b61032d6004803603602081101561031d57600080fd5b50356001600160a01b0316610b48565b604080519315158452911515602084015282820152519081900360600190f35b61036a6004803603602081101561036357600080fd5b5035610b6e565b604080516001600160a01b039092168252519081900360200190f35b816103918133610b95565b61039a83610c89565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b8152509061044a5760405162461bcd60e51b81526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561040f5781810151838201526020016103f7565b50505050905090810190601f16801561043c5780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b506001600160a01b0383166000908152602081815260409182902054825180840190935260128352711053149150511657d49151d254d51154915160721b9183019190915260ff16156104de5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506104e7611d9e565b60018082526040808301858152825480840184557fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b0319166001600160a01b0389169081179091556000818152602081815290849020865181548389015160ff199091169115159190911761ff001916610100911515919091021781559251928501929092556002805463ffffffff19811663ffffffff9182169096011694909417909355815192835290517f158412daecdc1456d01568828bcdb18464cc7f1ce0215ddbc3f3cfede9d1e63d9281900390910190a150505050565b856105d88133610b95565b6105e187610c89565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b815250906106545760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b03871660009081526020818152604091829020548251808401909352600e83526d1393d517d49151d254d51154915160921b9183019190915260ff166106e35760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b03871660009081526020818152604091829020548251808401909352601083526f1053149150511657d59154925192515160821b91830191909152610100900460ff161561077a5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b50600061078988878787610cac565b905060006107993385848b610dc3565b9050600081116040518060400160405280601081526020016f4e4f545f454e4f5547485f46554e445360801b815250906108145760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b03891660008181526020818152604091829020805461ff0019166101001781556001018b9055815192835282018a905280517f093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d9281900390910190a1505050505050505050565b60025463ffffffff1681565b8161089a8133610b95565b6108a383610c89565b6040518060400160405280600e81526020016d1393d517d057d0d3d395149050d560921b815250906109165760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b03831660009081526020818152604091829020548251808401909352600e83526d1393d517d49151d254d51154915160921b9183019190915260ff166109a55760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b03831660009081526020818152604091829020548251808401909352601083526f1053149150511657d59154925192515160821b91830191909152610100900460ff1615610a3c5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b03831660009081526020818152604091829020600101548251808401909352600a83526953414d455f56414c554560b01b91830191909152831415610aca5760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506001600160a01b038316600081815260208181526040918290206001018590558151928352820184905280517f093e3625e665824aefc0339d7d4770d943ca03bad991d13a4d797efc4bb20b6d9281900390910190a1505050565b6001600160a01b03811660009081526020819052604090205460ff165b919050565b6000602081905290815260409020805460019091015460ff808316926101009004169083565b60018181548110610b7b57fe5b6000918252602090912001546001600160a01b0316905081565b6000826001600160a01b03166370a08231836040518263ffffffff1660e01b815260040180826001600160a01b0316815260200191505060206040518083038186803b158015610be457600080fd5b505afa158015610bf8573d6000803e3d6000fd5b505050506040513d6020811015610c0e57600080fd5b505160408051808201909152601081526f4e4f545f454e4f5547485f46554e445360801b60208201529110610c845760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b505050565b6000806001600160a01b038316610ca4576000915050610b43565b50503b151590565b60408051808201909152601781527f424c4f434b484153485f4e4f545f415641494c41424c45000000000000000000602082015260009084409081610d325760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b50604080516bffffffffffffffffffffffff19606089901b1660208083019190915282518083036014018152603490920190925280519101206000610d778684610ea2565b90506060610d86868385610fb0565b9050610db6610d9c610d97836115a1565b6115e6565b80516002908110610da957fe5b60200260200101516116fc565b9998505050505050505050565b60408051808201909152601881527f554e50524f4345535345445f53544f524147455f524f4f540000000000000000602082015260009083610e465760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b506000610e53868461172a565b60408051602080820184905282518083038201815291830190925280519101209091506060610e83878784610fb0565b9050610e96610e91826115a1565b6116fc565b98975050505050505050565b6000607b8351116040518060400160405280601481526020017324a72b20a624a22fa12627a1a5afa422a0a222a960611b81525090610f225760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b50818380519060200120146040518060400160405280601481526020017324a72b20a624a22fa12627a1a5afa422a0a222a960611b81525090610fa65760405162461bcd60e51b815260206004820181815283516024840152835190928392604490910191908501908083836000831561040f5781810151838201526020016103f7565b505050607b015190565b604080516020808252818301909252606091829190602082018180368337019050509050826020820152610fe5816000611762565b90506060610ff5610d97876115a1565b9050606060006060611005611dbe565b6000855160001415611095577f56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218a14611077576040805162461bcd60e51b815260206004820152600f60248201526e2130b21032b6b83a3c90383937b7b360891b604482015290519081900360640190fd5b5050604080516000815260208101909152955061159a945050505050565b60005b8651811015611591576110bd8782815181106110b057fe5b60200260200101516118a1565b9550801580156110d35750855160208701208b14155b1561111c576040805162461bcd60e51b815260206004820152601460248201527318985908199a5c9cdd081c1c9bdbd9881c185c9d60621b604482015290519081900360640190fd5b8015801590611133575061112f8661190f565b8514155b15611170576040805162461bcd60e51b81526020600482015260086024820152670c4c2c840d0c2e6d60c31b604482015290519081900360640190fd5b61118c87828151811061117f57fe5b60200260200101516115e6565b93508351600214156113ae57600060606111c16111bc876000815181106111af57fe5b60200260200101516119c3565b611a45565b909250905060006111d3858c84611b53565b9050808501945081518110156112685760018a51038410156112265760405162461bcd60e51b8152600401808060200182810382526026815260200180611dd96026913960400191505060405180910390fd5b6000805b506040519080825280601f01601f191660200182016040528015611255576020820181803683370190505b509b50505050505050505050505061159a565b821561130a5760018a51038410156112c7576040805162461bcd60e51b815260206004820152601c60248201527f6c656166206d75737420636f6d65206c61737420696e2070726f6f6600000000604482015290519081900360640190fd5b8a518510156112d85760008061122a565b866001815181106112e557fe5b602002602001015195506112f8866119c3565b9b50505050505050505050505061159a565b60018a510384141561134d5760405162461bcd60e51b8152600401808060200182810382526026815260200180611e576026913960400191505060405180910390fd5b61136a8760018151811061135d57fe5b6020026020010151611bc8565b61138c5761137e876001815181106111af57fe5b8051906020012097506113a6565b61139c876001815181106110b057fe5b8051906020012097505b505050611589565b83516011141561158957875182146115125760008883815181106113ce57fe5b01602001516001939093019260f81c90506010811061141e5760405162461bcd60e51b815260040180806020018281038252602e815260200180611dff602e913960400191505060405180910390fd5b61143d858260ff168151811061143057fe5b6020026020010151611bf4565b156114ba576001885103821461149a576040805162461bcd60e51b815260206004820152601d60248201527f6c656166206e6f646573206f6e6c79206174206c617374206c6576656c000000604482015290519081900360640190fd5b5050604080516000815260208101909152975061159a9650505050505050565b6114cc858260ff168151811061135d57fe5b6114f0576114e2858260ff16815181106111af57fe5b80519060200120955061150c565b611502858260ff16815181106110b057fe5b8051906020012095505b50611589565b6001875103811461156a576040805162461bcd60e51b815260206004820152601760248201527f73686f756c64206265206174206c617374206c6576656c000000000000000000604482015290519081900360640190fd5b61157a846010815181106111af57fe5b9850505050505050505061159a565b600101611098565b50505050505050505b9392505050565b6115a9611dbe565b81516115c957506040805180820190915260008082526020820152610b43565b506040805180820190915281518152602082810190820152919050565b60606115f182611bc8565b61162c5760405162461bcd60e51b815260040180806020018281038252602a815260200180611e2d602a913960400191505060405180910390fd5b600061163783611c17565b90508067ffffffffffffffff8111801561165057600080fd5b5060405190808252806020026020018201604052801561168a57816020015b611677611dbe565b81526020019060019003908161166f5790505b509150600061169c8460200151611c64565b60208501510190506000805b838110156116f3576116b983611ccd565b91506040518060400160405280838152602001848152508582815181106116dc57fe5b6020908102919091010152918101916001016116a8565b50505050919050565b60008061170c8360200151611c64565b83516020948501518201519190039093036101000a90920492915050565b604080516001600160a01b03939093166020808501919091528382019290925280518084038201815260609093019052815191012090565b6060600083511161177257600080fd5b82516002028083111561178457600080fd5b8290038067ffffffffffffffff8111801561179e57600080fd5b506040519080825280601f01601f1916602001820160405280156117c9576020820181803683370190505b5091506000835b82850181101561188e57600281066118345760048660028304815181106117f357fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061181857fe5b60200101906001600160f81b031916908160001a905350611882565b600086600283048151811061184557fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061186a57fe5b60200101906001600160f81b031916908160001a9053505b600191820191016117d0565b508251811461189957fe5b505092915050565b606080826000015167ffffffffffffffff811180156118bf57600080fd5b506040519080825280601f01601f1916602001820160405280156118ea576020820181803683370190505b50905060008160200190506119088460200151828660000151611d5d565b5092915050565b6000602082511015611928575080516020820120610b43565b816040516020018082805190602001908083835b6020831061195b5780518252601f19909201916020918201910161193c565b6001836020036101000a0380198251168184511680821785525050505050509050019150506040516020818303038152906040528051906020012060405160200180828152602001915050604051602081830303815290604052805190602001209050610b43565b606060006119d48360200151611c64565b835190915081900360608167ffffffffffffffff811180156119f557600080fd5b506040519080825280601f01601f191660200182016040528015611a20576020820181803683370190505b5090506000816020019050611a3c848760200151018285611d5d565b50949350505050565b600060606000835111611a87576040805162461bcd60e51b8152602060048201526005602482015264456d70747960d81b604482015290519081900360640190fd5b6000600484600081518110611a9857fe5b60209101015160f81c901c600f169050600081611abb5750600092506002611b3d565b8160011415611ad05750600092506001611b3d565b8160021415611ae55750600192506002611b3d565b8160031415611af957506001925082611b3d565b6040805162461bcd60e51b81526020600482015260146024820152736661696c6564206465636f64696e67205472696560601b604482015290519081900360640190fd5b83611b488683611762565b935093505050915091565b6000805b8351858201108015611b695750825181105b15611bc057828181518110611b7a57fe5b602001015160f81c60f81b6001600160f81b0319168486830181518110611b9d57fe5b01602001516001600160f81b03191614611bb857905061159a565b600101611b57565b949350505050565b6020810151805160009190821a9060c0821015611bea57600092505050610b43565b5060019392505050565b8051600090600114611c0857506000610b43565b50602001515160001a60801490565b600080600090506000611c2d8460200151611c64565b602085015185519181019250015b80821015611c5b57611c4c82611ccd565b60019093019290910190611c3b565b50909392505050565b8051600090811a6080811015611c7e576000915050610b43565b60b8811080611c99575060c08110801590611c99575060f881105b15611ca8576001915050610b43565b60c0811015611cbc5760b519019050610b43565b60f519019050610b43565b50919050565b8051600090811a6080811015611ce7576001915050610b43565b60b8811015611cfb57607e19019050610b43565b60c0811015611d285760b78103600184019350806020036101000a84510460018201810193505050611cc7565b60f8811015611d3c5760be19019050610b43565b60019290920151602083900360f7016101000a900490910160f51901919050565b5b60208110611d7d578251825260209283019290910190601f1901611d5e565b915181516020939093036101000a6000190180199091169216919091179052565b604080516060810182526000808252602082018190529181019190915290565b60405180604001604052806000815260200160008152509056fe646976657267656e74206e6f6465206d75737420636f6d65206c61737420696e2070726f6f666966206272616e6368206e6f6465206561636820656c656d656e742068617320746f2062652061206e6962626c6543616e6e6f7420636f6e7665727420746f206c6973742061206e6f6e2d6c69737420524c504974656d2e657874656e73696f6e206e6f64652063616e6e6f74206265206174206c617374206c6576656ca26469706673582212200ac3c50b4942abc715c0e7fac630d5e025e76a6d25edac4c1c988d73a6c94dad64736f6c634300060c0033"

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
	TokenAddress common.Address
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterTokenRegistered is a free log retrieval operation binding the contract event 0x158412daecdc1456d01568828bcdb18464cc7f1ce0215ddbc3f3cfede9d1e63d.
//
// Solidity: event TokenRegistered(address tokenAddress)
func (_TokenStorageProof *TokenStorageProofFilterer) FilterTokenRegistered(opts *bind.FilterOpts) (*TokenStorageProofTokenRegisteredIterator, error) {
	logs, sub, err := _TokenStorageProof.contract.FilterLogs(opts, "TokenRegistered")
	if err != nil {
		return nil, err
	}
	return &TokenStorageProofTokenRegisteredIterator{contract: _TokenStorageProof.contract, event: "TokenRegistered", logs: logs, sub: sub}, nil
}

// WatchTokenRegistered is a free log subscription operation binding the contract event 0x158412daecdc1456d01568828bcdb18464cc7f1ce0215ddbc3f3cfede9d1e63d.
//
// Solidity: event TokenRegistered(address tokenAddress)
func (_TokenStorageProof *TokenStorageProofFilterer) WatchTokenRegistered(opts *bind.WatchOpts, sink chan<- *TokenStorageProofTokenRegistered) (event.Subscription, error) {
	logs, sub, err := _TokenStorageProof.contract.WatchLogs(opts, "TokenRegistered")
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

// ParseTokenRegistered is a log parse operation binding the contract event 0x158412daecdc1456d01568828bcdb18464cc7f1ce0215ddbc3f3cfede9d1e63d.
//
// Solidity: event TokenRegistered(address tokenAddress)
func (_TokenStorageProof *TokenStorageProofFilterer) ParseTokenRegistered(log types.Log) (*TokenStorageProofTokenRegistered, error) {
	event := new(TokenStorageProofTokenRegistered)
	if err := _TokenStorageProof.contract.UnpackLog(event, "TokenRegistered", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
