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
const TokenStorageProofABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"registrar\",\"type\":\"address\"}],\"name\":\"TokenRegistered\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"holder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"ercTokenAddress\",\"type\":\"address\"}],\"name\":\"getBalanceMappingPosition\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"blockHash\",\"type\":\"bytes32\"}],\"name\":\"getBlockHeaderStateRoot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"stateRoot\",\"type\":\"bytes32\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"holder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"getHolderBalanceSlot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"slot\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"stateRoot\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"getStorage\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"}],\"name\":\"getStorageRoot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"ercTokenAddress\",\"type\":\"address\"}],\"name\":\"isRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"registerToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"tokenAddresses\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tokenCount\",\"outputs\":[{\"internalType\":\"uint32\",\"name\":\"\",\"type\":\"uint32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"tokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"registered\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// TokenStorageProofBin is the compiled bytecode used for deploying new contracts.
var TokenStorageProofBin = "0x60806040526002805463ffffffff1916905534801561001d57600080fd5b50611c118061002d6000396000f3fe608060405234801561001057600080fd5b50600436106100a95760003560e01c8063c3c5a54711610071578063c3c5a547146103a5578063ca45b561146103df578063cb8d5e92146105af578063e3e197291461065f578063e48603391461079d578063e5df8b84146107dc576100a9565b8063210eea11146100ae5780632815a86a146101665780632fcd10a21461018c5780634ccdd506146101b85780639f181b5e14610384575b600080fd5b610154600480360360408110156100c457600080fd5b810190602081018135600160201b8111156100de57600080fd5b8201836020820111156100f057600080fd5b803590602001918460018302840111600160201b8311171561011157600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295505091359250610815915050565b60408051918252519081900360200190f35b6101546004803603602081101561017c57600080fd5b50356001600160a01b03166108b1565b610154600480360360408110156101a257600080fd5b506001600160a01b038135169060200135610920565b610382600480360360c08110156101ce57600080fd5b6001600160a01b038235169160208101359160408201359190810190608081016060820135600160201b81111561020457600080fd5b82018360208201111561021657600080fd5b803590602001918460018302840111600160201b8311171561023757600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561028957600080fd5b82018360208201111561029b57600080fd5b803590602001918460018302840111600160201b831117156102bc57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561030e57600080fd5b82018360208201111561032057600080fd5b803590602001918460018302840111600160201b8311171561034157600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610958945050505050565b005b61038c610b89565b6040805163ffffffff9092168252519081900360200190f35b6103cb600480360360208110156103bb57600080fd5b50356001600160a01b0316610b95565b604080519115158252519081900360200190f35b610154600480360360e08110156103f557600080fd5b6001600160a01b03823581169260208101359091169160408201359190810190608081016060820135600160201b81111561042f57600080fd5b82018360208201111561044157600080fd5b803590602001918460018302840111600160201b8311171561046257600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b8111156104b457600080fd5b8201836020820111156104c657600080fd5b803590602001918460018302840111600160201b831117156104e757600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561053957600080fd5b82018360208201111561054b57600080fd5b803590602001918460018302840111600160201b8311171561056c57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295505091359250610c06915050565b610154600480360360608110156105c557600080fd5b813591602081013591810190606081016040820135600160201b8111156105eb57600080fd5b8201836020820111156105fd57600080fd5b803590602001918460018302840111600160201b8311171561061e57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610c90945050505050565b6101546004803603608081101561067557600080fd5b6001600160a01b0382351691602081013591810190606081016040820135600160201b8111156106a457600080fd5b8201836020820111156106b657600080fd5b803590602001918460018302840111600160201b831117156106d757600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561072957600080fd5b82018360208201111561073b57600080fd5b803590602001918460018302840111600160201b8311171561075c57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610d14945050505050565b6107c3600480360360208110156107b357600080fd5b50356001600160a01b0316610df9565b6040805192835290151560208301528051918290030190f35b6107f9600480360360208110156107f257600080fd5b5035610e15565b604080516001600160a01b039092168252519081900360200190f35b6000607b835111610864576040805162461bcd60e51b815260206004820152601460248201527334b73b30b634b210313637b1b5903432b0b232b960611b604482015290519081900360640190fd5b8251602084012082146108a85760405162461bcd60e51b8152600401808060200182810382526022815260200180611b666022913960400191505060405180910390fd5b5050607b015190565b60006001600160a01b038216610900576040805162461bcd60e51b815260206004820152600f60248201526e496e76616c6964206164647265737360881b604482015290519081900360640190fd5b506001600160a01b0381166000908152602081905260409020545b919050565b604080516001600160a01b03939093166020808501919091528382019290925280518084038201815260609093019052815191012090565b61096186610e3c565b6109b2576040805162461bcd60e51b815260206004820152601e60248201527f5468652061646472657373206d757374206265206120636f6e74726163740000604482015290519081900360640190fd5b6109bb86610b95565b15610a0d576040805162461bcd60e51b815260206004820152601860248201527f546f6b656e20616c726561647920726567697374657265640000000000000000604482015290519081900360640190fd5b604080516370a0823160e01b8152336004820152905187916000916001600160a01b038416916370a08231916024808301926020929190829003018186803b158015610a5857600080fd5b505afa158015610a6c573d6000803e3d6000fd5b505050506040513d6020811015610a8257600080fd5b5051905080610acd576040805162461bcd60e51b8152602060048201526012602482015271496e73756666696369656e742066756e647360701b604482015290519081900360640190fd5b6001600160a01b0388166000818152602081905260408082206001808201805460ff1916821790558b8255805480820182558185527fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b031916861790556002805463ffffffff19811663ffffffff9182169093011691909117905590519092339290917f487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf09190a3505050505050505050565b60025463ffffffff1681565b60006001600160a01b038216610be4576040805162461bcd60e51b815260206004820152600f60248201526e496e76616c6964206164647265737360881b604482015290519081900360640190fd5b506001600160a01b031660009081526020819052604090206001015460ff1690565b6000610c1188610b95565b610c59576040805162461bcd60e51b8152602060048201526014602482015273151bdad95b881b9bdd081c9959da5cdd195c995960621b604482015290519081900360640190fd5b6000610c658884610920565b90506000610c7589898989610d14565b9050610c82828287610c90565b9a9950505050505050505050565b600082610cce576040805162461bcd60e51b81526020600482015260076024820152666e6f206461746160c81b604482015290519081900360640190fd5b6040805160208082018790528251808303820181529183019092528051910120610d09610d04610cff858785610e5f565b6113fb565b611420565b9150505b9392505050565b6000834080610d6a576040805162461bcd60e51b815260206004820152601760248201527f626c6f636b68617368206e6f7420617661696c61626c65000000000000000000604482015290519081900360640190fd5b6000610d768583610815565b905060008760405160200180826001600160a01b031660601b81526014019150506040516020818303038152906040528051906020012090506060610dbc868484610e5f565b9050610dec610dd2610dcd836113fb565b611480565b80516002908110610ddf57fe5b6020026020010151611420565b9998505050505050505050565b6000602081905290815260409020805460019091015460ff1682565b60018181548110610e2257fe5b6000918252602090912001546001600160a01b0316905081565b6000806001600160a01b038316610e5757600091505061091b565b50503b151590565b604080516020808252818301909252606091829190602082018180368337019050509050826020820152610e94816000611568565b90506060610ea4610dcd876113fb565b9050606060006060610eb4611b25565b6000855160001415610f44577f56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218a14610f26576040805162461bcd60e51b815260206004820152600f60248201526e2130b21032b6b83a3c90383937b7b360891b604482015290519081900360640190fd5b50506040805160008152602081019091529550610d0d945050505050565b60005b86518110156113ec57610f6c878281518110610f5f57fe5b60200260200101516116a7565b955080158015610f825750855160208701208b14155b15610fcb576040805162461bcd60e51b815260206004820152601460248201527318985908199a5c9cdd081c1c9bdbd9881c185c9d60621b604482015290519081900360640190fd5b610fe7878281518110610fda57fe5b6020026020010151611480565b9350835160021415611209576000606061101c6110178760008151811061100a57fe5b6020026020010151611726565b6117ab565b9092509050600061102e858c846118b9565b9050808501945081518110156110c35760018a51038410156110815760405162461bcd60e51b8152600401808060200182810382526026815260200180611b406026913960400191505060405180910390fd5b6000805b506040519080825280601f01601f1916602001820160405280156110b0576020820181803683370190505b509b505050505050505050505050610d0d565b82156111655760018a5103841015611122576040805162461bcd60e51b815260206004820152601c60248201527f6c656166206d75737420636f6d65206c61737420696e2070726f6f6600000000604482015290519081900360640190fd5b8a5185101561113357600080611085565b8660018151811061114057fe5b6020026020010151955061115386611726565b9b505050505050505050505050610d0d565b60018a51038414156111a85760405162461bcd60e51b8152600401808060200182810382526026815260200180611bb66026913960400191505060405180910390fd5b6111c5876001815181106111b857fe5b602002602001015161192e565b6111e7576111d98760018151811061100a57fe5b805190602001209750611201565b6111f787600181518110610f5f57fe5b8051906020012097505b5050506113e4565b8351601114156113e4578751821461136d57600088838151811061122957fe5b01602001516001939093019260f81c9050601081106112795760405162461bcd60e51b815260040180806020018281038252602e815260200180611b88602e913960400191505060405180910390fd5b611298858260ff168151811061128b57fe5b6020026020010151611968565b1561131557600188510382146112f5576040805162461bcd60e51b815260206004820152601d60248201527f6c656166206e6f646573206f6e6c79206174206c617374206c6576656c000000604482015290519081900360640190fd5b50506040805160008152602081019091529750610d0d9650505050505050565b611327858260ff16815181106111b857fe5b61134b5761133d858260ff168151811061100a57fe5b805190602001209550611367565b61135d858260ff1681518110610f5f57fe5b8051906020012095505b506113e4565b600187510381146113c5576040805162461bcd60e51b815260206004820152601760248201527f73686f756c64206265206174206c617374206c6576656c000000000000000000604482015290519081900360640190fd5b6113d58460108151811061100a57fe5b98505050505050505050610d0d565b600101610f47565b50505050505050509392505050565b611403611b25565b506040805180820190915281518152602082810190820152919050565b80516000901580159061143557508151602110155b61143e57600080fd5b600061144d836020015161198b565b8351602080860151830180519394509184900392919083101561147757826020036101000a820491505b50949350505050565b606061148b8261192e565b61149457600080fd5b600061149f836119ee565b905060608167ffffffffffffffff811180156114ba57600080fd5b506040519080825280602002602001820160405280156114f457816020015b6114e1611b25565b8152602001906001900390816114d95790505b5090506000611506856020015161198b565b60208601510190506000805b8481101561155d5761152383611a46565b915060405180604001604052808381526020018481525084828151811061154657fe5b602090810291909101015291810191600101611512565b509195945050505050565b6060600083511161157857600080fd5b82516002028083111561158a57600080fd5b8290038067ffffffffffffffff811180156115a457600080fd5b506040519080825280601f01601f1916602001820160405280156115cf576020820181803683370190505b5091506000835b828501811015611694576002810661163a5760048660028304815181106115f957fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061161e57fe5b60200101906001600160f81b031916908160001a905350611688565b600086600283048151811061164b57fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061167057fe5b60200101906001600160f81b031916908160001a9053505b600191820191016115d6565b508251811461169f57fe5b505092915050565b606080826000015167ffffffffffffffff811180156116c557600080fd5b506040519080825280601f01601f1916602001820160405280156116f0576020820181803683370190505b50905080516000141561170457905061091b565b600081602001905061171f8460200151828660000151611ada565b5092915050565b805160609061173457600080fd5b6000611743836020015161198b565b835190915081900360608167ffffffffffffffff8111801561176457600080fd5b506040519080825280601f01601f19166020018201604052801561178f576020820181803683370190505b5090506000816020019050611477848760200151018285611ada565b6000606060008351116117ed576040805162461bcd60e51b8152602060048201526005602482015264456d70747960d81b604482015290519081900360640190fd5b60006004846000815181106117fe57fe5b60209101015160f81c901c600f16905060008161182157506000925060026118a3565b816001141561183657506000925060016118a3565b816002141561184b57506001925060026118a3565b816003141561185f575060019250826118a3565b6040805162461bcd60e51b81526020600482015260146024820152736661696c6564206465636f64696e67205472696560601b604482015290519081900360640190fd5b836118ae8683611568565b935093505050915091565b6000805b83518582011080156118cf5750825181105b15611926578281815181106118e057fe5b602001015160f81c60f81b6001600160f81b031916848683018151811061190357fe5b01602001516001600160f81b0319161461191e579050610d0d565b6001016118bd565b949350505050565b805160009061193f5750600061091b565b6020820151805160001a9060c082101561195e5760009250505061091b565b5060019392505050565b805160009060011461197c5750600061091b565b50602001515160001a60801490565b8051600090811a60808110156119a557600091505061091b565b60b88110806119c0575060c081108015906119c0575060f881105b156119cf57600191505061091b565b60c08110156119e35760b51901905061091b565b60f51901905061091b565b80516000906119ff5750600061091b565b600080611a0f846020015161198b565b602085015185519181019250015b80821015611a3d57611a2e82611a46565b60019093019290910190611a1d565b50909392505050565b80516000908190811a6080811015611a61576001915061171f565b60b8811015611a7657607e198101915061171f565b60c0811015611aa35760b78103600185019450806020036101000a8551046001820181019350505061171f565b60f8811015611ab85760be198101915061171f565b60019390930151602084900360f7016101000a900490920160f5190192915050565b80611ae457611b20565b5b60208110611b04578251825260209283019290910190601f1901611ae5565b8251825160208390036101000a60001901801990921691161782525b505050565b60405180604001604052806000815260200160008152509056fe646976657267656e74206e6f6465206d75737420636f6d65206c61737420696e2070726f6f666d69736d61746368206f6e2074686520626c6f636b686173682070726f76696465646966206272616e6368206e6f6465206561636820656c656d656e742068617320746f2062652061206e6962626c65657874656e73696f6e206e6f64652063616e6e6f74206265206174206c617374206c6576656ca264697066735822122043536b5e80ce7ca786cf8206020d0e88e744c7730991adc84d5e550e9d07701a64736f6c634300060c0033"

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
// Solidity: function getBalanceMappingPosition(address ercTokenAddress) view returns(uint256)
func (_TokenStorageProof *TokenStorageProofCaller) GetBalanceMappingPosition(opts *bind.CallOpts, ercTokenAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "getBalanceMappingPosition", ercTokenAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBalanceMappingPosition is a free data retrieval call binding the contract method 0x2815a86a.
//
// Solidity: function getBalanceMappingPosition(address ercTokenAddress) view returns(uint256)
func (_TokenStorageProof *TokenStorageProofSession) GetBalanceMappingPosition(ercTokenAddress common.Address) (*big.Int, error) {
	return _TokenStorageProof.Contract.GetBalanceMappingPosition(&_TokenStorageProof.CallOpts, ercTokenAddress)
}

// GetBalanceMappingPosition is a free data retrieval call binding the contract method 0x2815a86a.
//
// Solidity: function getBalanceMappingPosition(address ercTokenAddress) view returns(uint256)
func (_TokenStorageProof *TokenStorageProofCallerSession) GetBalanceMappingPosition(ercTokenAddress common.Address) (*big.Int, error) {
	return _TokenStorageProof.Contract.GetBalanceMappingPosition(&_TokenStorageProof.CallOpts, ercTokenAddress)
}

// GetBlockHeaderStateRoot is a free data retrieval call binding the contract method 0x210eea11.
//
// Solidity: function getBlockHeaderStateRoot(bytes blockHeaderRLP, bytes32 blockHash) pure returns(bytes32 stateRoot)
func (_TokenStorageProof *TokenStorageProofCaller) GetBlockHeaderStateRoot(opts *bind.CallOpts, blockHeaderRLP []byte, blockHash [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "getBlockHeaderStateRoot", blockHeaderRLP, blockHash)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetBlockHeaderStateRoot is a free data retrieval call binding the contract method 0x210eea11.
//
// Solidity: function getBlockHeaderStateRoot(bytes blockHeaderRLP, bytes32 blockHash) pure returns(bytes32 stateRoot)
func (_TokenStorageProof *TokenStorageProofSession) GetBlockHeaderStateRoot(blockHeaderRLP []byte, blockHash [32]byte) ([32]byte, error) {
	return _TokenStorageProof.Contract.GetBlockHeaderStateRoot(&_TokenStorageProof.CallOpts, blockHeaderRLP, blockHash)
}

// GetBlockHeaderStateRoot is a free data retrieval call binding the contract method 0x210eea11.
//
// Solidity: function getBlockHeaderStateRoot(bytes blockHeaderRLP, bytes32 blockHash) pure returns(bytes32 stateRoot)
func (_TokenStorageProof *TokenStorageProofCallerSession) GetBlockHeaderStateRoot(blockHeaderRLP []byte, blockHash [32]byte) ([32]byte, error) {
	return _TokenStorageProof.Contract.GetBlockHeaderStateRoot(&_TokenStorageProof.CallOpts, blockHeaderRLP, blockHash)
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

// GetStorage is a free data retrieval call binding the contract method 0xcb8d5e92.
//
// Solidity: function getStorage(uint256 slot, bytes32 stateRoot, bytes storageProof) pure returns(uint256)
func (_TokenStorageProof *TokenStorageProofCaller) GetStorage(opts *bind.CallOpts, slot *big.Int, stateRoot [32]byte, storageProof []byte) (*big.Int, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "getStorage", slot, stateRoot, storageProof)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetStorage is a free data retrieval call binding the contract method 0xcb8d5e92.
//
// Solidity: function getStorage(uint256 slot, bytes32 stateRoot, bytes storageProof) pure returns(uint256)
func (_TokenStorageProof *TokenStorageProofSession) GetStorage(slot *big.Int, stateRoot [32]byte, storageProof []byte) (*big.Int, error) {
	return _TokenStorageProof.Contract.GetStorage(&_TokenStorageProof.CallOpts, slot, stateRoot, storageProof)
}

// GetStorage is a free data retrieval call binding the contract method 0xcb8d5e92.
//
// Solidity: function getStorage(uint256 slot, bytes32 stateRoot, bytes storageProof) pure returns(uint256)
func (_TokenStorageProof *TokenStorageProofCallerSession) GetStorage(slot *big.Int, stateRoot [32]byte, storageProof []byte) (*big.Int, error) {
	return _TokenStorageProof.Contract.GetStorage(&_TokenStorageProof.CallOpts, slot, stateRoot, storageProof)
}

// GetStorageRoot is a free data retrieval call binding the contract method 0xe3e19729.
//
// Solidity: function getStorageRoot(address account, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof) view returns(bytes32)
func (_TokenStorageProof *TokenStorageProofCaller) GetStorageRoot(opts *bind.CallOpts, account common.Address, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte) ([32]byte, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "getStorageRoot", account, blockNumber, blockHeaderRLP, accountStateProof)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetStorageRoot is a free data retrieval call binding the contract method 0xe3e19729.
//
// Solidity: function getStorageRoot(address account, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof) view returns(bytes32)
func (_TokenStorageProof *TokenStorageProofSession) GetStorageRoot(account common.Address, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte) ([32]byte, error) {
	return _TokenStorageProof.Contract.GetStorageRoot(&_TokenStorageProof.CallOpts, account, blockNumber, blockHeaderRLP, accountStateProof)
}

// GetStorageRoot is a free data retrieval call binding the contract method 0xe3e19729.
//
// Solidity: function getStorageRoot(address account, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof) view returns(bytes32)
func (_TokenStorageProof *TokenStorageProofCallerSession) GetStorageRoot(account common.Address, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte) ([32]byte, error) {
	return _TokenStorageProof.Contract.GetStorageRoot(&_TokenStorageProof.CallOpts, account, blockNumber, blockHeaderRLP, accountStateProof)
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

// GetBalance is a paid mutator transaction binding the contract method 0xca45b561.
//
// Solidity: function getBalance(address token, address holder, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof, uint256 balanceMappingPosition) returns(uint256)
func (_TokenStorageProof *TokenStorageProofTransactor) GetBalance(opts *bind.TransactOpts, token common.Address, holder common.Address, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.contract.Transact(opts, "getBalance", token, holder, blockNumber, blockHeaderRLP, accountStateProof, storageProof, balanceMappingPosition)
}

// GetBalance is a paid mutator transaction binding the contract method 0xca45b561.
//
// Solidity: function getBalance(address token, address holder, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof, uint256 balanceMappingPosition) returns(uint256)
func (_TokenStorageProof *TokenStorageProofSession) GetBalance(token common.Address, holder common.Address, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.GetBalance(&_TokenStorageProof.TransactOpts, token, holder, blockNumber, blockHeaderRLP, accountStateProof, storageProof, balanceMappingPosition)
}

// GetBalance is a paid mutator transaction binding the contract method 0xca45b561.
//
// Solidity: function getBalance(address token, address holder, uint256 blockNumber, bytes blockHeaderRLP, bytes accountStateProof, bytes storageProof, uint256 balanceMappingPosition) returns(uint256)
func (_TokenStorageProof *TokenStorageProofTransactorSession) GetBalance(token common.Address, holder common.Address, blockNumber *big.Int, blockHeaderRLP []byte, accountStateProof []byte, storageProof []byte, balanceMappingPosition *big.Int) (*types.Transaction, error) {
	return _TokenStorageProof.Contract.GetBalance(&_TokenStorageProof.TransactOpts, token, holder, blockNumber, blockHeaderRLP, accountStateProof, storageProof, balanceMappingPosition)
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
