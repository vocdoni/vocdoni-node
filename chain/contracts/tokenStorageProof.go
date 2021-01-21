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
const TokenStorageProofABI = "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"registrar\",\"type\":\"address\"}],\"name\":\"TokenRegistered\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"holder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"getBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"ercTokenAddress\",\"type\":\"address\"}],\"name\":\"getBalanceMappingPosition\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"blockHash\",\"type\":\"bytes32\"}],\"name\":\"getBlockHeaderStateRoot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"stateRoot\",\"type\":\"bytes32\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"holder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"}],\"name\":\"getHolderBalanceSlot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"slot\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"stateRoot\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"getStorage\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"}],\"name\":\"getStorageRoot\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"ercTokenAddress\",\"type\":\"address\"}],\"name\":\"isRegistered\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"blockHeaderRLP\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"accountStateProof\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"storageProof\",\"type\":\"bytes\"}],\"name\":\"registerToken\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"tokenList\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"tokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"balanceMappingPosition\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"registered\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]"

// TokenStorageProofBin is the compiled bytecode used for deploying new contracts.
var TokenStorageProofBin = "0x608060405234801561001057600080fd5b50611bbb806100206000396000f3fe608060405234801561001057600080fd5b506004361061009e5760003560e01c8063c3c5a54711610066578063c3c5a547146103b2578063ca45b561146103ec578063cb8d5e92146105bc578063e3e197291461066c578063e4860339146107aa5761009e565b8063210eea11146100a35780632815a86a1461015b5780632fcd10a2146101815780634ccdd506146101ad5780639ead722214610379575b600080fd5b610149600480360360408110156100b957600080fd5b810190602081018135600160201b8111156100d357600080fd5b8201836020820111156100e557600080fd5b803590602001918460018302840111600160201b8311171561010657600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955050913592506107e9915050565b60408051918252519081900360200190f35b6101496004803603602081101561017157600080fd5b50356001600160a01b0316610885565b6101496004803603604081101561019757600080fd5b506001600160a01b0381351690602001356108f4565b610377600480360360c08110156101c357600080fd5b6001600160a01b038235169160208101359160408201359190810190608081016060820135600160201b8111156101f957600080fd5b82018360208201111561020b57600080fd5b803590602001918460018302840111600160201b8311171561022c57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561027e57600080fd5b82018360208201111561029057600080fd5b803590602001918460018302840111600160201b831117156102b157600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561030357600080fd5b82018360208201111561031557600080fd5b803590602001918460018302840111600160201b8311171561033657600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955061092c945050505050565b005b6103966004803603602081101561038f57600080fd5b5035610b3f565b604080516001600160a01b039092168252519081900360200190f35b6103d8600480360360208110156103c857600080fd5b50356001600160a01b0316610b66565b604080519115158252519081900360200190f35b610149600480360360e081101561040257600080fd5b6001600160a01b03823581169260208101359091169160408201359190810190608081016060820135600160201b81111561043c57600080fd5b82018360208201111561044e57600080fd5b803590602001918460018302840111600160201b8311171561046f57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b8111156104c157600080fd5b8201836020820111156104d357600080fd5b803590602001918460018302840111600160201b831117156104f457600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561054657600080fd5b82018360208201111561055857600080fd5b803590602001918460018302840111600160201b8311171561057957600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295505091359250610bd7915050565b610149600480360360608110156105d257600080fd5b813591602081013591810190606081016040820135600160201b8111156105f857600080fd5b82018360208201111561060a57600080fd5b803590602001918460018302840111600160201b8311171561062b57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610c61945050505050565b6101496004803603608081101561068257600080fd5b6001600160a01b0382351691602081013591810190606081016040820135600160201b8111156106b157600080fd5b8201836020820111156106c357600080fd5b803590602001918460018302840111600160201b831117156106e457600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295949360208101935035915050600160201b81111561073657600080fd5b82018360208201111561074857600080fd5b803590602001918460018302840111600160201b8311171561076957600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610ce5945050505050565b6107d0600480360360208110156107c057600080fd5b50356001600160a01b0316610dca565b6040805192835290151560208301528051918290030190f35b6000607b835111610838576040805162461bcd60e51b815260206004820152601460248201527334b73b30b634b210313637b1b5903432b0b232b960611b604482015290519081900360640190fd5b82516020840120821461087c5760405162461bcd60e51b8152600401808060200182810382526022815260200180611b106022913960400191505060405180910390fd5b5050607b015190565b60006001600160a01b0382166108d4576040805162461bcd60e51b815260206004820152600f60248201526e496e76616c6964206164647265737360881b604482015290519081900360640190fd5b506001600160a01b0381166000908152602081905260409020545b919050565b604080516001600160a01b03939093166020808501919091528382019290925280518084038201815260609093019052815191012090565b61093586610de6565b610986576040805162461bcd60e51b815260206004820152601e60248201527f5468652061646472657373206d757374206265206120636f6e74726163740000604482015290519081900360640190fd5b61098f86610b66565b156109e1576040805162461bcd60e51b815260206004820152601860248201527f546f6b656e20616c726561647920726567697374657265640000000000000000604482015290519081900360640190fd5b604080516370a0823160e01b8152336004820152905187916000916001600160a01b038416916370a08231916024808301926020929190829003018186803b158015610a2c57600080fd5b505afa158015610a40573d6000803e3d6000fd5b505050506040513d6020811015610a5657600080fd5b5051905080610aa1576040805162461bcd60e51b8152602060048201526012602482015271496e73756666696369656e742066756e647360701b604482015290519081900360640190fd5b6001600160a01b0388166000818152602081905260408082206001808201805460ff1916821790558b8255805480820182559084527fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b0319168517905590519092339290917f487c37289624c10056468f1f98ebffbad01edce11374975179672e32e2543bf09190a3505050505050505050565b60018181548110610b4c57fe5b6000918252602090912001546001600160a01b0316905081565b60006001600160a01b038216610bb5576040805162461bcd60e51b815260206004820152600f60248201526e496e76616c6964206164647265737360881b604482015290519081900360640190fd5b506001600160a01b031660009081526020819052604090206001015460ff1690565b6000610be288610b66565b610c2a576040805162461bcd60e51b8152602060048201526014602482015273151bdad95b881b9bdd081c9959da5cdd195c995960621b604482015290519081900360640190fd5b6000610c3688846108f4565b90506000610c4689898989610ce5565b9050610c53828287610c61565b9a9950505050505050505050565b600082610c9f576040805162461bcd60e51b81526020600482015260076024820152666e6f206461746160c81b604482015290519081900360640190fd5b6040805160208082018790528251808303820181529183019092528051910120610cda610cd5610cd0858785610e09565b6113a5565b6113ca565b9150505b9392505050565b6000834080610d3b576040805162461bcd60e51b815260206004820152601760248201527f626c6f636b68617368206e6f7420617661696c61626c65000000000000000000604482015290519081900360640190fd5b6000610d4785836107e9565b905060008760405160200180826001600160a01b031660601b81526014019150506040516020818303038152906040528051906020012090506060610d8d868484610e09565b9050610dbd610da3610d9e836113a5565b61142a565b80516002908110610db057fe5b60200260200101516113ca565b9998505050505050505050565b6000602081905290815260409020805460019091015460ff1682565b6000806001600160a01b038316610e015760009150506108ef565b50503b151590565b604080516020808252818301909252606091829190602082018180368337019050509050826020820152610e3e816000611512565b90506060610e4e610d9e876113a5565b9050606060006060610e5e611acf565b6000855160001415610eee577f56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b4218a14610ed0576040805162461bcd60e51b815260206004820152600f60248201526e2130b21032b6b83a3c90383937b7b360891b604482015290519081900360640190fd5b50506040805160008152602081019091529550610cde945050505050565b60005b865181101561139657610f16878281518110610f0957fe5b6020026020010151611651565b955080158015610f2c5750855160208701208b14155b15610f75576040805162461bcd60e51b815260206004820152601460248201527318985908199a5c9cdd081c1c9bdbd9881c185c9d60621b604482015290519081900360640190fd5b610f91878281518110610f8457fe5b602002602001015161142a565b93508351600214156111b35760006060610fc6610fc187600081518110610fb457fe5b60200260200101516116d0565b611755565b90925090506000610fd8858c84611863565b90508085019450815181101561106d5760018a510384101561102b5760405162461bcd60e51b8152600401808060200182810382526026815260200180611aea6026913960400191505060405180910390fd5b6000805b506040519080825280601f01601f19166020018201604052801561105a576020820181803683370190505b509b505050505050505050505050610cde565b821561110f5760018a51038410156110cc576040805162461bcd60e51b815260206004820152601c60248201527f6c656166206d75737420636f6d65206c61737420696e2070726f6f6600000000604482015290519081900360640190fd5b8a518510156110dd5760008061102f565b866001815181106110ea57fe5b602002602001015195506110fd866116d0565b9b505050505050505050505050610cde565b60018a51038414156111525760405162461bcd60e51b8152600401808060200182810382526026815260200180611b606026913960400191505060405180910390fd5b61116f8760018151811061116257fe5b60200260200101516118d8565b6111915761118387600181518110610fb457fe5b8051906020012097506111ab565b6111a187600181518110610f0957fe5b8051906020012097505b50505061138e565b83516011141561138e57875182146113175760008883815181106111d357fe5b01602001516001939093019260f81c9050601081106112235760405162461bcd60e51b815260040180806020018281038252602e815260200180611b32602e913960400191505060405180910390fd5b611242858260ff168151811061123557fe5b6020026020010151611912565b156112bf576001885103821461129f576040805162461bcd60e51b815260206004820152601d60248201527f6c656166206e6f646573206f6e6c79206174206c617374206c6576656c000000604482015290519081900360640190fd5b50506040805160008152602081019091529750610cde9650505050505050565b6112d1858260ff168151811061116257fe5b6112f5576112e7858260ff1681518110610fb457fe5b805190602001209550611311565b611307858260ff1681518110610f0957fe5b8051906020012095505b5061138e565b6001875103811461136f576040805162461bcd60e51b815260206004820152601760248201527f73686f756c64206265206174206c617374206c6576656c000000000000000000604482015290519081900360640190fd5b61137f84601081518110610fb457fe5b98505050505050505050610cde565b600101610ef1565b50505050505050509392505050565b6113ad611acf565b506040805180820190915281518152602082810190820152919050565b8051600090158015906113df57508151602110155b6113e857600080fd5b60006113f78360200151611935565b8351602080860151830180519394509184900392919083101561142157826020036101000a820491505b50949350505050565b6060611435826118d8565b61143e57600080fd5b600061144983611998565b905060608167ffffffffffffffff8111801561146457600080fd5b5060405190808252806020026020018201604052801561149e57816020015b61148b611acf565b8152602001906001900390816114835790505b50905060006114b08560200151611935565b60208601510190506000805b84811015611507576114cd836119f0565b91506040518060400160405280838152602001848152508482815181106114f057fe5b6020908102919091010152918101916001016114bc565b509195945050505050565b6060600083511161152257600080fd5b82516002028083111561153457600080fd5b8290038067ffffffffffffffff8111801561154e57600080fd5b506040519080825280601f01601f191660200182016040528015611579576020820181803683370190505b5091506000835b82850181101561163e57600281066115e45760048660028304815181106115a357fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b8483815181106115c857fe5b60200101906001600160f81b031916908160001a905350611632565b60008660028304815181106115f557fe5b602001015160f81c60f81b60f81c60ff16901c600f1660f81b84838151811061161a57fe5b60200101906001600160f81b031916908160001a9053505b60019182019101611580565b508251811461164957fe5b505092915050565b606080826000015167ffffffffffffffff8111801561166f57600080fd5b506040519080825280601f01601f19166020018201604052801561169a576020820181803683370190505b5090508051600014156116ae5790506108ef565b60008160200190506116c98460200151828660000151611a84565b5092915050565b80516060906116de57600080fd5b60006116ed8360200151611935565b835190915081900360608167ffffffffffffffff8111801561170e57600080fd5b506040519080825280601f01601f191660200182016040528015611739576020820181803683370190505b5090506000816020019050611421848760200151018285611a84565b600060606000835111611797576040805162461bcd60e51b8152602060048201526005602482015264456d70747960d81b604482015290519081900360640190fd5b60006004846000815181106117a857fe5b60209101015160f81c901c600f1690506000816117cb575060009250600261184d565b81600114156117e0575060009250600161184d565b81600214156117f5575060019250600261184d565b81600314156118095750600192508261184d565b6040805162461bcd60e51b81526020600482015260146024820152736661696c6564206465636f64696e67205472696560601b604482015290519081900360640190fd5b836118588683611512565b935093505050915091565b6000805b83518582011080156118795750825181105b156118d05782818151811061188a57fe5b602001015160f81c60f81b6001600160f81b03191684868301815181106118ad57fe5b01602001516001600160f81b031916146118c8579050610cde565b600101611867565b949350505050565b80516000906118e9575060006108ef565b6020820151805160001a9060c0821015611908576000925050506108ef565b5060019392505050565b8051600090600114611926575060006108ef565b50602001515160001a60801490565b8051600090811a608081101561194f5760009150506108ef565b60b881108061196a575060c0811080159061196a575060f881105b156119795760019150506108ef565b60c081101561198d5760b5190190506108ef565b60f5190190506108ef565b80516000906119a9575060006108ef565b6000806119b98460200151611935565b602085015185519181019250015b808210156119e7576119d8826119f0565b600190930192909101906119c7565b50909392505050565b80516000908190811a6080811015611a0b57600191506116c9565b60b8811015611a2057607e19810191506116c9565b60c0811015611a4d5760b78103600185019450806020036101000a855104600182018101935050506116c9565b60f8811015611a625760be19810191506116c9565b60019390930151602084900360f7016101000a900490920160f5190192915050565b80611a8e57611aca565b5b60208110611aae578251825260209283019290910190601f1901611a8f565b8251825160208390036101000a60001901801990921691161782525b505050565b60405180604001604052806000815260200160008152509056fe646976657267656e74206e6f6465206d75737420636f6d65206c61737420696e2070726f6f666d69736d61746368206f6e2074686520626c6f636b686173682070726f76696465646966206272616e6368206e6f6465206561636820656c656d656e742068617320746f2062652061206e6962626c65657874656e73696f6e206e6f64652063616e6e6f74206265206174206c617374206c6576656ca264697066735822122087b686354bd388d0d51c8fe04da27b3ab2fddb66080b03e64ca29f5ac288636464736f6c634300060c0033"

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

// TokenList is a free data retrieval call binding the contract method 0x9ead7222.
//
// Solidity: function tokenList(uint256 ) view returns(address)
func (_TokenStorageProof *TokenStorageProofCaller) TokenList(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _TokenStorageProof.contract.Call(opts, &out, "tokenList", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// TokenList is a free data retrieval call binding the contract method 0x9ead7222.
//
// Solidity: function tokenList(uint256 ) view returns(address)
func (_TokenStorageProof *TokenStorageProofSession) TokenList(arg0 *big.Int) (common.Address, error) {
	return _TokenStorageProof.Contract.TokenList(&_TokenStorageProof.CallOpts, arg0)
}

// TokenList is a free data retrieval call binding the contract method 0x9ead7222.
//
// Solidity: function tokenList(uint256 ) view returns(address)
func (_TokenStorageProof *TokenStorageProofCallerSession) TokenList(arg0 *big.Int) (common.Address, error) {
	return _TokenStorageProof.Contract.TokenList(&_TokenStorageProof.CallOpts, arg0)
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

	outstruct.BalanceMappingPosition = out[0].(*big.Int)
	outstruct.Registered = out[1].(bool)

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
