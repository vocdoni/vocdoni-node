// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package votingprocess

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
const VotingProcessABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"getProcessesLength\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"votesBatch\",\"type\":\"bytes32\"}],\"name\":\"addVotesBatch\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getRelayByIndex\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getRelaysLength\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"organizer\",\"type\":\"address\"}],\"name\":\"getProcessesIdByOrganizer\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getCensusMetadata\",\"outputs\":[{\"name\":\"censusMerkleRoot\",\"type\":\"bytes32\"},{\"name\":\"censusProofUrl\",\"type\":\"string\"},{\"name\":\"censusRequestUrl\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"processesIndex\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"voteEncryptionPrivateKey\",\"type\":\"string\"}],\"name\":\"setVotingQuestion\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"relayAddress\",\"type\":\"address\"},{\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getVotesBatch\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getVoteEncryptionPrivateKey\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"relayAddress\",\"type\":\"address\"}],\"name\":\"isRelayRegistered\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"name\",\"type\":\"string\"},{\"name\":\"startBlock\",\"type\":\"uint256\"},{\"name\":\"endBlock\",\"type\":\"uint256\"},{\"name\":\"censusMerkleRoot\",\"type\":\"bytes32\"},{\"name\":\"censusProofUrl\",\"type\":\"string\"},{\"name\":\"censusRequestUrl\",\"type\":\"string\"},{\"name\":\"question\",\"type\":\"string\"},{\"name\":\"votingOptions\",\"type\":\"bytes32[]\"},{\"name\":\"voteEncryptionPublicKey\",\"type\":\"string\"}],\"name\":\"createProcess\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"},{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"organizerProcesses\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"}],\"name\":\"getProcessMetadata\",\"outputs\":[{\"name\":\"name\",\"type\":\"string\"},{\"name\":\"startBlock\",\"type\":\"uint256\"},{\"name\":\"endBlock\",\"type\":\"uint256\"},{\"name\":\"question\",\"type\":\"string\"},{\"name\":\"votingOptions\",\"type\":\"bytes32[]\"},{\"name\":\"voteEncryptionPublicKey\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processIndex\",\"type\":\"uint256\"}],\"name\":\"getProcessIdByIndex\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getNullVotesBatchValue\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"voteEncryptionPrivateKey\",\"type\":\"string\"}],\"name\":\"publishVoteEncryptionPrivateKey\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"relayAddress\",\"type\":\"address\"}],\"name\":\"registerRelay\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"organizer\",\"type\":\"address\"},{\"name\":\"name\",\"type\":\"string\"}],\"name\":\"getProcessId\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\"}],\"payable\":false,\"stateMutability\":\"pure\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"processId\",\"type\":\"bytes32\"},{\"name\":\"relayAddress\",\"type\":\"address\"}],\"name\":\"getRelayVotesBatchesLength\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"

// VotingProcessBin is the compiled bytecode used for deploying new contracts.
const VotingProcessBin = `0x608060405234801561001057600080fd5b50611882806100206000396000f3fe608060405234801561001057600080fd5b5060043610610149576000357c010000000000000000000000000000000000000000000000000000000090048063585cf030116100ca578063c705eddb1161008e578063c705eddb14610a84578063e113808a1461037e578063ea52908314610a8c578063f34e2fc814610ab8578063f849b2c714610b6e57610149565b8063585cf030146104ef5780636059b2f11461052f57806375eaf30014610888578063b2227d4a146108b4578063c5b59eae14610a6757610149565b80633d52c822116101115780633d52c8221461025f5780633ff26b04146103615780634168a18e1461037e5780634d58f2431461042b5780635602ef191461045d57610149565b806307b29b341461014e57806318a863d614610168578063324d70ef1461018d57806333f53cfc146101cc5780633c245cd2146101e9575b600080fd5b610156610b9a565b60408051918252519081900360200190f35b61018b6004803603604081101561017e57600080fd5b5080359060200135610ba1565b005b6101b0600480360360408110156101a357600080fd5b5080359060200135610c9b565b60408051600160a060020a039092168252519081900360200190f35b610156600480360360208110156101e257600080fd5b5035610cd6565b61020f600480360360208110156101ff57600080fd5b5035600160a060020a0316610ceb565b60408051602080825283518183015283519192839290830191858101910280838360005b8381101561024b578181015183820152602001610233565b505050509050019250505060405180910390f35b61027c6004803603602081101561027557600080fd5b5035610d57565b604051808481526020018060200180602001838103835285818151815260200191508051906020019080838360005b838110156102c35781810151838201526020016102ab565b50505050905090810190601f1680156102f05780820380516001836020036101000a031916815260200191505b50838103825284518152845160209182019186019080838360005b8381101561032357818101518382015260200161030b565b50505050905090810190601f1680156103505780820380516001836020036101000a031916815260200191505b509550505050505060405180910390f35b6101566004803603602081101561037757600080fd5b5035610ea0565b61018b6004803603604081101561039457600080fd5b813591908101906040810160208201356401000000008111156103b657600080fd5b8201836020820111156103c857600080fd5b803590602001918460018302840111640100000000831117156103ea57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610ebf945050505050565b6101566004803603606081101561044157600080fd5b50803590600160a060020a036020820135169060400135610fc8565b61047a6004803603602081101561047357600080fd5b503561100b565b6040805160208082528351818301528351919283929083019185019080838360005b838110156104b457818101518382015260200161049c565b50505050905090810190601f1680156104e15780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b61051b6004803603604081101561050557600080fd5b5080359060200135600160a060020a03166110a3565b604080519115158252519081900360200190f35b61018b600480360361012081101561054657600080fd5b81019060208101813564010000000081111561056157600080fd5b82018360208201111561057357600080fd5b8035906020019184600183028401116401000000008311171561059557600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092958435956020860135956040810135955091935091506080810190606001356401000000008111156105fa57600080fd5b82018360208201111561060c57600080fd5b8035906020019184600183028401116401000000008311171561062e57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561068157600080fd5b82018360208201111561069357600080fd5b803590602001918460018302840111640100000000831117156106b557600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561070857600080fd5b82018360208201111561071a57600080fd5b8035906020019184600183028401116401000000008311171561073c57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929594936020810193503591505064010000000081111561078f57600080fd5b8201836020820111156107a157600080fd5b803590602001918460208302840111640100000000831117156107c357600080fd5b919080806020026020016040519081016040528093929190818152602001838360200280828437600092019190915250929594936020810193503591505064010000000081111561081357600080fd5b82018360208201111561082557600080fd5b8035906020019184600183028401116401000000008311171561084757600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295506110de945050505050565b6101566004803603604081101561089e57600080fd5b50600160a060020a0381351690602001356112ce565b6108d1600480360360208110156108ca57600080fd5b50356112fe565b604051808060200187815260200186815260200180602001806020018060200185810385528b818151815260200191508051906020019080838360005b8381101561092657818101518382015260200161090e565b50505050905090810190601f1680156109535780820380516001836020036101000a031916815260200191505b5085810384528851815288516020918201918a019080838360005b8381101561098657818101518382015260200161096e565b50505050905090810190601f1680156109b35780820380516001836020036101000a031916815260200191505b508581038352875181528751602091820191808a01910280838360005b838110156109e85781810151838201526020016109d0565b50505050905001858103825286818151815260200191508051906020019080838360005b83811015610a24578181015183820152602001610a0c565b50505050905090810190601f168015610a515780820380516001836020036101000a031916815260200191505b509a505050505050505050505060405180910390f35b61015660048036036020811015610a7d57600080fd5b5035611544565b610156611567565b61018b60048036036040811015610aa257600080fd5b5080359060200135600160a060020a031661156d565b61015660048036036040811015610ace57600080fd5b600160a060020a038235169190810190604081016020820135640100000000811115610af957600080fd5b820183602082011115610b0b57600080fd5b80359060200191846001830284011164010000000083111715610b2d57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295506116b9945050505050565b61015660048036036040811015610b8457600080fd5b5080359060200135600160a060020a031661175a565b6001545b90565b60008281526020819052604090206002015482904311610c0b576040805160e560020a62461bcd02815260206004820152601b60248201527f50726f6365737320686173206e6f742073746172746564207965740000000000604482015290519081900360640190fd5b610c1583336110a3565b1515600114610c6e576040805160e560020a62461bcd02815260206004820152601760248201527f52656c6179206973206e6f742072656769737465726564000000000000000000604482015290519081900360640190fd5b50600091825260208281526040808420338552600c01825283208054600181018255908452922090910155565b6000828152602081905260408120600b01805483908110610cb857fe5b600091825260209091200154600160a060020a031690505b92915050565b6000908152602081905260409020600b015490565b600160a060020a038116600090815260026020908152604091829020805483518184028101840190945280845260609392830182828015610d4b57602002820191906000526020600020905b815481526020019060010190808311610d37575b50505050509050919050565b6000818152602081815260408083206008810154600982018054845160026001831615610100026000190190921691909104601f8101879004870282018701909552848152606095869593949293600a909301928491830182828015610dfe5780601f10610dd357610100808354040283529160200191610dfe565b820191906000526020600020905b815481529060010190602001808311610de157829003601f168201915b5050845460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815295975086945092508401905082828015610e8c5780601f10610e6157610100808354040283529160200191610e8c565b820191906000526020600020905b815481529060010190602001808311610e6f57829003601f168201915b505050505090509250925092509193909250565b6001805482908110610eae57fe5b600091825260209091200154905081565b60008281526020819052604090205482906101009004600160a060020a03163314610f34576040805160e560020a62461bcd02815260206004820152601b60248201527f6d73672e73656e646572206973206e6f74206f7267616e697a65720000000000604482015290519081900360640190fd5b60008381526020819052604090206003015483904311610f9e576040805160e560020a62461bcd02815260206004820152601560248201527f50726f63657373207374696c6c2072756e6e696e670000000000000000000000604482015290519081900360640190fd5b6000848152602081815260409091208451610fc192600790920191860190611784565b5050505050565b600083815260208181526040808320600160a060020a0386168452600c019091528120805483908110610ff757fe5b906000526020600020015490509392505050565b6000818152602081815260409182902060070180548351601f6002600019610100600186161502019093169290920491820184900484028101840190945280845260609392830182828015610d4b5780601f1061107657610100808354040283529160200191610d4b565b820191906000526020600020905b8154815290600101906020018083116110845750939695505050505050565b600082815260208181526040808320600160a060020a0385168452600c0190915281205415156110d557506000610cd0565b50600192915050565b60006110ea338b6116b9565b60008181526020819052604090205490915060ff1615611154576040805160e560020a62461bcd02815260206004820152601860248201527f50726f63636573496420616c7265616479206578697374730000000000000000604482015290519081900360640190fd5b6000818152602081815260409091208b51611177926001909201918d0190611784565b50600081815260208181526040909120805474ffffffffffffffffffffffffffffffffffffffff0019163361010002178155600281018b9055600381018a90556008810189905587516111d292600990920191890190611784565b5060008181526020818152604090912086516111f692600a90920191880190611784565b50600081815260208181526040909120855161121a92600490920191870190611784565b50600081815260208181526040909120845161123e92600590920191860190611802565b50600081815260208181526040909120835161126292600690920191850190611784565b50600081815260208181526040808320805460ff19166001908117909155805480820182557fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6018590553384526002835290832080549182018155835291200155505050505050505050565b6002602052816000526040600020818154811015156112e957fe5b90600052602060002001600091509150505481565b60008181526020818152604080832060028082015460038301546001808501805487519281161561010002600019011694909404601f81018890048802820188019096528581526060979687968996879687969095909490936004830193600584019360060192918891908301828280156113ba5780601f1061138f576101008083540402835291602001916113ba565b820191906000526020600020905b81548152906001019060200180831161139d57829003601f168201915b5050865460408051602060026001851615610100026000190190941693909304601f8101849004840282018401909252818152959b50889450925084019050828280156114485780601f1061141d57610100808354040283529160200191611448565b820191906000526020600020905b81548152906001019060200180831161142b57829003601f168201915b505050505092508180548060200260200160405190810160405280929190818152602001828054801561149a57602002820191906000526020600020905b815481526020019060010190808311611486575b5050845460408051602060026001851615610100026000190190941693909304601f8101849004840282018401909252818152959750869450925084019050828280156115285780601f106114fd57610100808354040283529160200191611528565b820191906000526020600020905b81548152906001019060200180831161150b57829003601f168201915b5050505050905095509550955095509550955091939550919395565b600060018281548110151561155557fe5b90600052602060002001549050919050565b60001990565b60008281526020819052604090205482906101009004600160a060020a031633146115e2576040805160e560020a62461bcd02815260206004820152601b60248201527f6d73672e73656e646572206973206e6f74206f7267616e697a65720000000000604482015290519081900360640190fd5b6115ec83836110a3565b15611641576040805160e560020a62461bcd02815260206004820181905260248201527f43616e277420726567697374657220616e206578736974696e672072656c6179604482015290519081900360640190fd5b600083815260208181526040808320600b8101805460018101825590855283852001805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0388169081179091558452600c01909152902061169e611567565b81546001810183556000928352602090922090910155505050565b600082826040516020018083600160a060020a0316600160a060020a03166c0100000000000000000000000002815260140182805190602001908083835b602083106117165780518252601f1990920191602091820191016116f7565b6001836020036101000a0380198251168184511680821785525050505050509050019250505060405160208183030381529060405280519060200120905092915050565b600082815260208181526040808320600160a060020a0385168452600c0190915290205492915050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106117c557805160ff19168380011785556117f2565b828001600101855582156117f2579182015b828111156117f25782518255916020019190600101906117d7565b506117fe92915061183c565b5090565b8280548282559060005260206000209081019282156117f257916020028201828111156117f25782518255916020019190600101906117d7565b610b9e91905b808211156117fe576000815560010161184256fea165627a7a72305820886ce092d4205d48c64d1aee11bb7350e6394672e1b54a0391bfcac90436dfe10029`

// DeployVotingProcess deploys a new Ethereum contract, binding an instance of VotingProcess to it.
func DeployVotingProcess(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *VotingProcess, error) {
	parsed, err := abi.JSON(strings.NewReader(VotingProcessABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(VotingProcessBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &VotingProcess{VotingProcessCaller: VotingProcessCaller{contract: contract}, VotingProcessTransactor: VotingProcessTransactor{contract: contract}, VotingProcessFilterer: VotingProcessFilterer{contract: contract}}, nil
}

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

// GetCensusMetadata is a free data retrieval call binding the contract method 0x3d52c822.
//
// Solidity: function getCensusMetadata(bytes32 processId) constant returns(bytes32 censusMerkleRoot, string censusProofUrl, string censusRequestUrl)
func (_VotingProcess *VotingProcessCaller) GetCensusMetadata(opts *bind.CallOpts, processId [32]byte) (struct {
	CensusMerkleRoot [32]byte
	CensusProofUrl   string
	CensusRequestUrl string
}, error) {
	ret := new(struct {
		CensusMerkleRoot [32]byte
		CensusProofUrl   string
		CensusRequestUrl string
	})
	out := ret
	err := _VotingProcess.contract.Call(opts, out, "getCensusMetadata", processId)
	return *ret, err
}

// GetCensusMetadata is a free data retrieval call binding the contract method 0x3d52c822.
//
// Solidity: function getCensusMetadata(bytes32 processId) constant returns(bytes32 censusMerkleRoot, string censusProofUrl, string censusRequestUrl)
func (_VotingProcess *VotingProcessSession) GetCensusMetadata(processId [32]byte) (struct {
	CensusMerkleRoot [32]byte
	CensusProofUrl   string
	CensusRequestUrl string
}, error) {
	return _VotingProcess.Contract.GetCensusMetadata(&_VotingProcess.CallOpts, processId)
}

// GetCensusMetadata is a free data retrieval call binding the contract method 0x3d52c822.
//
// Solidity: function getCensusMetadata(bytes32 processId) constant returns(bytes32 censusMerkleRoot, string censusProofUrl, string censusRequestUrl)
func (_VotingProcess *VotingProcessCallerSession) GetCensusMetadata(processId [32]byte) (struct {
	CensusMerkleRoot [32]byte
	CensusProofUrl   string
	CensusRequestUrl string
}, error) {
	return _VotingProcess.Contract.GetCensusMetadata(&_VotingProcess.CallOpts, processId)
}

// GetNullVotesBatchValue is a free data retrieval call binding the contract method 0xc705eddb.
//
// Solidity: function getNullVotesBatchValue() constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) GetNullVotesBatchValue(opts *bind.CallOpts) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getNullVotesBatchValue")
	return *ret0, err
}

// GetNullVotesBatchValue is a free data retrieval call binding the contract method 0xc705eddb.
//
// Solidity: function getNullVotesBatchValue() constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) GetNullVotesBatchValue() ([32]byte, error) {
	return _VotingProcess.Contract.GetNullVotesBatchValue(&_VotingProcess.CallOpts)
}

// GetNullVotesBatchValue is a free data retrieval call binding the contract method 0xc705eddb.
//
// Solidity: function getNullVotesBatchValue() constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) GetNullVotesBatchValue() ([32]byte, error) {
	return _VotingProcess.Contract.GetNullVotesBatchValue(&_VotingProcess.CallOpts)
}

// GetProcessId is a free data retrieval call binding the contract method 0xf34e2fc8.
//
// Solidity: function getProcessId(address organizer, string name) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) GetProcessId(opts *bind.CallOpts, organizer common.Address, name string) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getProcessId", organizer, name)
	return *ret0, err
}

// GetProcessId is a free data retrieval call binding the contract method 0xf34e2fc8.
//
// Solidity: function getProcessId(address organizer, string name) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) GetProcessId(organizer common.Address, name string) ([32]byte, error) {
	return _VotingProcess.Contract.GetProcessId(&_VotingProcess.CallOpts, organizer, name)
}

// GetProcessId is a free data retrieval call binding the contract method 0xf34e2fc8.
//
// Solidity: function getProcessId(address organizer, string name) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) GetProcessId(organizer common.Address, name string) ([32]byte, error) {
	return _VotingProcess.Contract.GetProcessId(&_VotingProcess.CallOpts, organizer, name)
}

// GetProcessIdByIndex is a free data retrieval call binding the contract method 0xc5b59eae.
//
// Solidity: function getProcessIdByIndex(uint256 processIndex) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) GetProcessIdByIndex(opts *bind.CallOpts, processIndex *big.Int) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getProcessIdByIndex", processIndex)
	return *ret0, err
}

// GetProcessIdByIndex is a free data retrieval call binding the contract method 0xc5b59eae.
//
// Solidity: function getProcessIdByIndex(uint256 processIndex) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) GetProcessIdByIndex(processIndex *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.GetProcessIdByIndex(&_VotingProcess.CallOpts, processIndex)
}

// GetProcessIdByIndex is a free data retrieval call binding the contract method 0xc5b59eae.
//
// Solidity: function getProcessIdByIndex(uint256 processIndex) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) GetProcessIdByIndex(processIndex *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.GetProcessIdByIndex(&_VotingProcess.CallOpts, processIndex)
}

// GetProcessMetadata is a free data retrieval call binding the contract method 0xb2227d4a.
//
// Solidity: function getProcessMetadata(bytes32 processId) constant returns(string name, uint256 startBlock, uint256 endBlock, string question, bytes32[] votingOptions, string voteEncryptionPublicKey)
func (_VotingProcess *VotingProcessCaller) GetProcessMetadata(opts *bind.CallOpts, processId [32]byte) (struct {
	Name                    string
	StartBlock              *big.Int
	EndBlock                *big.Int
	Question                string
	VotingOptions           [][32]byte
	VoteEncryptionPublicKey string
}, error) {
	ret := new(struct {
		Name                    string
		StartBlock              *big.Int
		EndBlock                *big.Int
		Question                string
		VotingOptions           [][32]byte
		VoteEncryptionPublicKey string
	})
	out := ret
	err := _VotingProcess.contract.Call(opts, out, "getProcessMetadata", processId)
	return *ret, err
}

// GetProcessMetadata is a free data retrieval call binding the contract method 0xb2227d4a.
//
// Solidity: function getProcessMetadata(bytes32 processId) constant returns(string name, uint256 startBlock, uint256 endBlock, string question, bytes32[] votingOptions, string voteEncryptionPublicKey)
func (_VotingProcess *VotingProcessSession) GetProcessMetadata(processId [32]byte) (struct {
	Name                    string
	StartBlock              *big.Int
	EndBlock                *big.Int
	Question                string
	VotingOptions           [][32]byte
	VoteEncryptionPublicKey string
}, error) {
	return _VotingProcess.Contract.GetProcessMetadata(&_VotingProcess.CallOpts, processId)
}

// GetProcessMetadata is a free data retrieval call binding the contract method 0xb2227d4a.
//
// Solidity: function getProcessMetadata(bytes32 processId) constant returns(string name, uint256 startBlock, uint256 endBlock, string question, bytes32[] votingOptions, string voteEncryptionPublicKey)
func (_VotingProcess *VotingProcessCallerSession) GetProcessMetadata(processId [32]byte) (struct {
	Name                    string
	StartBlock              *big.Int
	EndBlock                *big.Int
	Question                string
	VotingOptions           [][32]byte
	VoteEncryptionPublicKey string
}, error) {
	return _VotingProcess.Contract.GetProcessMetadata(&_VotingProcess.CallOpts, processId)
}

// GetProcessesIdByOrganizer is a free data retrieval call binding the contract method 0x3c245cd2.
//
// Solidity: function getProcessesIdByOrganizer(address organizer) constant returns(bytes32[])
func (_VotingProcess *VotingProcessCaller) GetProcessesIdByOrganizer(opts *bind.CallOpts, organizer common.Address) ([][32]byte, error) {
	var (
		ret0 = new([][32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getProcessesIdByOrganizer", organizer)
	return *ret0, err
}

// GetProcessesIdByOrganizer is a free data retrieval call binding the contract method 0x3c245cd2.
//
// Solidity: function getProcessesIdByOrganizer(address organizer) constant returns(bytes32[])
func (_VotingProcess *VotingProcessSession) GetProcessesIdByOrganizer(organizer common.Address) ([][32]byte, error) {
	return _VotingProcess.Contract.GetProcessesIdByOrganizer(&_VotingProcess.CallOpts, organizer)
}

// GetProcessesIdByOrganizer is a free data retrieval call binding the contract method 0x3c245cd2.
//
// Solidity: function getProcessesIdByOrganizer(address organizer) constant returns(bytes32[])
func (_VotingProcess *VotingProcessCallerSession) GetProcessesIdByOrganizer(organizer common.Address) ([][32]byte, error) {
	return _VotingProcess.Contract.GetProcessesIdByOrganizer(&_VotingProcess.CallOpts, organizer)
}

// GetProcessesLength is a free data retrieval call binding the contract method 0x07b29b34.
//
// Solidity: function getProcessesLength() constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) GetProcessesLength(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getProcessesLength")
	return *ret0, err
}

// GetProcessesLength is a free data retrieval call binding the contract method 0x07b29b34.
//
// Solidity: function getProcessesLength() constant returns(uint256)
func (_VotingProcess *VotingProcessSession) GetProcessesLength() (*big.Int, error) {
	return _VotingProcess.Contract.GetProcessesLength(&_VotingProcess.CallOpts)
}

// GetProcessesLength is a free data retrieval call binding the contract method 0x07b29b34.
//
// Solidity: function getProcessesLength() constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) GetProcessesLength() (*big.Int, error) {
	return _VotingProcess.Contract.GetProcessesLength(&_VotingProcess.CallOpts)
}

// GetRelayByIndex is a free data retrieval call binding the contract method 0x324d70ef.
//
// Solidity: function getRelayByIndex(bytes32 processId, uint256 index) constant returns(address)
func (_VotingProcess *VotingProcessCaller) GetRelayByIndex(opts *bind.CallOpts, processId [32]byte, index *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getRelayByIndex", processId, index)
	return *ret0, err
}

// GetRelayByIndex is a free data retrieval call binding the contract method 0x324d70ef.
//
// Solidity: function getRelayByIndex(bytes32 processId, uint256 index) constant returns(address)
func (_VotingProcess *VotingProcessSession) GetRelayByIndex(processId [32]byte, index *big.Int) (common.Address, error) {
	return _VotingProcess.Contract.GetRelayByIndex(&_VotingProcess.CallOpts, processId, index)
}

// GetRelayByIndex is a free data retrieval call binding the contract method 0x324d70ef.
//
// Solidity: function getRelayByIndex(bytes32 processId, uint256 index) constant returns(address)
func (_VotingProcess *VotingProcessCallerSession) GetRelayByIndex(processId [32]byte, index *big.Int) (common.Address, error) {
	return _VotingProcess.Contract.GetRelayByIndex(&_VotingProcess.CallOpts, processId, index)
}

// GetRelayVotesBatchesLength is a free data retrieval call binding the contract method 0xf849b2c7.
//
// Solidity: function getRelayVotesBatchesLength(bytes32 processId, address relayAddress) constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) GetRelayVotesBatchesLength(opts *bind.CallOpts, processId [32]byte, relayAddress common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getRelayVotesBatchesLength", processId, relayAddress)
	return *ret0, err
}

// GetRelayVotesBatchesLength is a free data retrieval call binding the contract method 0xf849b2c7.
//
// Solidity: function getRelayVotesBatchesLength(bytes32 processId, address relayAddress) constant returns(uint256)
func (_VotingProcess *VotingProcessSession) GetRelayVotesBatchesLength(processId [32]byte, relayAddress common.Address) (*big.Int, error) {
	return _VotingProcess.Contract.GetRelayVotesBatchesLength(&_VotingProcess.CallOpts, processId, relayAddress)
}

// GetRelayVotesBatchesLength is a free data retrieval call binding the contract method 0xf849b2c7.
//
// Solidity: function getRelayVotesBatchesLength(bytes32 processId, address relayAddress) constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) GetRelayVotesBatchesLength(processId [32]byte, relayAddress common.Address) (*big.Int, error) {
	return _VotingProcess.Contract.GetRelayVotesBatchesLength(&_VotingProcess.CallOpts, processId, relayAddress)
}

// GetRelaysLength is a free data retrieval call binding the contract method 0x33f53cfc.
//
// Solidity: function getRelaysLength(bytes32 processId) constant returns(uint256)
func (_VotingProcess *VotingProcessCaller) GetRelaysLength(opts *bind.CallOpts, processId [32]byte) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getRelaysLength", processId)
	return *ret0, err
}

// GetRelaysLength is a free data retrieval call binding the contract method 0x33f53cfc.
//
// Solidity: function getRelaysLength(bytes32 processId) constant returns(uint256)
func (_VotingProcess *VotingProcessSession) GetRelaysLength(processId [32]byte) (*big.Int, error) {
	return _VotingProcess.Contract.GetRelaysLength(&_VotingProcess.CallOpts, processId)
}

// GetRelaysLength is a free data retrieval call binding the contract method 0x33f53cfc.
//
// Solidity: function getRelaysLength(bytes32 processId) constant returns(uint256)
func (_VotingProcess *VotingProcessCallerSession) GetRelaysLength(processId [32]byte) (*big.Int, error) {
	return _VotingProcess.Contract.GetRelaysLength(&_VotingProcess.CallOpts, processId)
}

// GetVoteEncryptionPrivateKey is a free data retrieval call binding the contract method 0x5602ef19.
//
// Solidity: function getVoteEncryptionPrivateKey(bytes32 processId) constant returns(string)
func (_VotingProcess *VotingProcessCaller) GetVoteEncryptionPrivateKey(opts *bind.CallOpts, processId [32]byte) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getVoteEncryptionPrivateKey", processId)
	return *ret0, err
}

// GetVoteEncryptionPrivateKey is a free data retrieval call binding the contract method 0x5602ef19.
//
// Solidity: function getVoteEncryptionPrivateKey(bytes32 processId) constant returns(string)
func (_VotingProcess *VotingProcessSession) GetVoteEncryptionPrivateKey(processId [32]byte) (string, error) {
	return _VotingProcess.Contract.GetVoteEncryptionPrivateKey(&_VotingProcess.CallOpts, processId)
}

// GetVoteEncryptionPrivateKey is a free data retrieval call binding the contract method 0x5602ef19.
//
// Solidity: function getVoteEncryptionPrivateKey(bytes32 processId) constant returns(string)
func (_VotingProcess *VotingProcessCallerSession) GetVoteEncryptionPrivateKey(processId [32]byte) (string, error) {
	return _VotingProcess.Contract.GetVoteEncryptionPrivateKey(&_VotingProcess.CallOpts, processId)
}

// GetVotesBatch is a free data retrieval call binding the contract method 0x4d58f243.
//
// Solidity: function getVotesBatch(bytes32 processId, address relayAddress, uint256 index) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) GetVotesBatch(opts *bind.CallOpts, processId [32]byte, relayAddress common.Address, index *big.Int) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "getVotesBatch", processId, relayAddress, index)
	return *ret0, err
}

// GetVotesBatch is a free data retrieval call binding the contract method 0x4d58f243.
//
// Solidity: function getVotesBatch(bytes32 processId, address relayAddress, uint256 index) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) GetVotesBatch(processId [32]byte, relayAddress common.Address, index *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.GetVotesBatch(&_VotingProcess.CallOpts, processId, relayAddress, index)
}

// GetVotesBatch is a free data retrieval call binding the contract method 0x4d58f243.
//
// Solidity: function getVotesBatch(bytes32 processId, address relayAddress, uint256 index) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) GetVotesBatch(processId [32]byte, relayAddress common.Address, index *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.GetVotesBatch(&_VotingProcess.CallOpts, processId, relayAddress, index)
}

// IsRelayRegistered is a free data retrieval call binding the contract method 0x585cf030.
//
// Solidity: function isRelayRegistered(bytes32 processId, address relayAddress) constant returns(bool)
func (_VotingProcess *VotingProcessCaller) IsRelayRegistered(opts *bind.CallOpts, processId [32]byte, relayAddress common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "isRelayRegistered", processId, relayAddress)
	return *ret0, err
}

// IsRelayRegistered is a free data retrieval call binding the contract method 0x585cf030.
//
// Solidity: function isRelayRegistered(bytes32 processId, address relayAddress) constant returns(bool)
func (_VotingProcess *VotingProcessSession) IsRelayRegistered(processId [32]byte, relayAddress common.Address) (bool, error) {
	return _VotingProcess.Contract.IsRelayRegistered(&_VotingProcess.CallOpts, processId, relayAddress)
}

// IsRelayRegistered is a free data retrieval call binding the contract method 0x585cf030.
//
// Solidity: function isRelayRegistered(bytes32 processId, address relayAddress) constant returns(bool)
func (_VotingProcess *VotingProcessCallerSession) IsRelayRegistered(processId [32]byte, relayAddress common.Address) (bool, error) {
	return _VotingProcess.Contract.IsRelayRegistered(&_VotingProcess.CallOpts, processId, relayAddress)
}

// OrganizerProcesses is a free data retrieval call binding the contract method 0x75eaf300.
//
// Solidity: function organizerProcesses(address , uint256 ) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) OrganizerProcesses(opts *bind.CallOpts, arg0 common.Address, arg1 *big.Int) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "organizerProcesses", arg0, arg1)
	return *ret0, err
}

// OrganizerProcesses is a free data retrieval call binding the contract method 0x75eaf300.
//
// Solidity: function organizerProcesses(address , uint256 ) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) OrganizerProcesses(arg0 common.Address, arg1 *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.OrganizerProcesses(&_VotingProcess.CallOpts, arg0, arg1)
}

// OrganizerProcesses is a free data retrieval call binding the contract method 0x75eaf300.
//
// Solidity: function organizerProcesses(address , uint256 ) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) OrganizerProcesses(arg0 common.Address, arg1 *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.OrganizerProcesses(&_VotingProcess.CallOpts, arg0, arg1)
}

// ProcessesIndex is a free data retrieval call binding the contract method 0x3ff26b04.
//
// Solidity: function processesIndex(uint256 ) constant returns(bytes32)
func (_VotingProcess *VotingProcessCaller) ProcessesIndex(opts *bind.CallOpts, arg0 *big.Int) ([32]byte, error) {
	var (
		ret0 = new([32]byte)
	)
	out := ret0
	err := _VotingProcess.contract.Call(opts, out, "processesIndex", arg0)
	return *ret0, err
}

// ProcessesIndex is a free data retrieval call binding the contract method 0x3ff26b04.
//
// Solidity: function processesIndex(uint256 ) constant returns(bytes32)
func (_VotingProcess *VotingProcessSession) ProcessesIndex(arg0 *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.ProcessesIndex(&_VotingProcess.CallOpts, arg0)
}

// ProcessesIndex is a free data retrieval call binding the contract method 0x3ff26b04.
//
// Solidity: function processesIndex(uint256 ) constant returns(bytes32)
func (_VotingProcess *VotingProcessCallerSession) ProcessesIndex(arg0 *big.Int) ([32]byte, error) {
	return _VotingProcess.Contract.ProcessesIndex(&_VotingProcess.CallOpts, arg0)
}

// AddVotesBatch is a paid mutator transaction binding the contract method 0x18a863d6.
//
// Solidity: function addVotesBatch(bytes32 processId, bytes32 votesBatch) returns()
func (_VotingProcess *VotingProcessTransactor) AddVotesBatch(opts *bind.TransactOpts, processId [32]byte, votesBatch [32]byte) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "addVotesBatch", processId, votesBatch)
}

// AddVotesBatch is a paid mutator transaction binding the contract method 0x18a863d6.
//
// Solidity: function addVotesBatch(bytes32 processId, bytes32 votesBatch) returns()
func (_VotingProcess *VotingProcessSession) AddVotesBatch(processId [32]byte, votesBatch [32]byte) (*types.Transaction, error) {
	return _VotingProcess.Contract.AddVotesBatch(&_VotingProcess.TransactOpts, processId, votesBatch)
}

// AddVotesBatch is a paid mutator transaction binding the contract method 0x18a863d6.
//
// Solidity: function addVotesBatch(bytes32 processId, bytes32 votesBatch) returns()
func (_VotingProcess *VotingProcessTransactorSession) AddVotesBatch(processId [32]byte, votesBatch [32]byte) (*types.Transaction, error) {
	return _VotingProcess.Contract.AddVotesBatch(&_VotingProcess.TransactOpts, processId, votesBatch)
}

// CreateProcess is a paid mutator transaction binding the contract method 0x6059b2f1.
//
// Solidity: function createProcess(string name, uint256 startBlock, uint256 endBlock, bytes32 censusMerkleRoot, string censusProofUrl, string censusRequestUrl, string question, bytes32[] votingOptions, string voteEncryptionPublicKey) returns()
func (_VotingProcess *VotingProcessTransactor) CreateProcess(opts *bind.TransactOpts, name string, startBlock *big.Int, endBlock *big.Int, censusMerkleRoot [32]byte, censusProofUrl string, censusRequestUrl string, question string, votingOptions [][32]byte, voteEncryptionPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "createProcess", name, startBlock, endBlock, censusMerkleRoot, censusProofUrl, censusRequestUrl, question, votingOptions, voteEncryptionPublicKey)
}

// CreateProcess is a paid mutator transaction binding the contract method 0x6059b2f1.
//
// Solidity: function createProcess(string name, uint256 startBlock, uint256 endBlock, bytes32 censusMerkleRoot, string censusProofUrl, string censusRequestUrl, string question, bytes32[] votingOptions, string voteEncryptionPublicKey) returns()
func (_VotingProcess *VotingProcessSession) CreateProcess(name string, startBlock *big.Int, endBlock *big.Int, censusMerkleRoot [32]byte, censusProofUrl string, censusRequestUrl string, question string, votingOptions [][32]byte, voteEncryptionPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.CreateProcess(&_VotingProcess.TransactOpts, name, startBlock, endBlock, censusMerkleRoot, censusProofUrl, censusRequestUrl, question, votingOptions, voteEncryptionPublicKey)
}

// CreateProcess is a paid mutator transaction binding the contract method 0x6059b2f1.
//
// Solidity: function createProcess(string name, uint256 startBlock, uint256 endBlock, bytes32 censusMerkleRoot, string censusProofUrl, string censusRequestUrl, string question, bytes32[] votingOptions, string voteEncryptionPublicKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) CreateProcess(name string, startBlock *big.Int, endBlock *big.Int, censusMerkleRoot [32]byte, censusProofUrl string, censusRequestUrl string, question string, votingOptions [][32]byte, voteEncryptionPublicKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.CreateProcess(&_VotingProcess.TransactOpts, name, startBlock, endBlock, censusMerkleRoot, censusProofUrl, censusRequestUrl, question, votingOptions, voteEncryptionPublicKey)
}

// PublishVoteEncryptionPrivateKey is a paid mutator transaction binding the contract method 0xe113808a.
//
// Solidity: function publishVoteEncryptionPrivateKey(bytes32 processId, string voteEncryptionPrivateKey) returns()
func (_VotingProcess *VotingProcessTransactor) PublishVoteEncryptionPrivateKey(opts *bind.TransactOpts, processId [32]byte, voteEncryptionPrivateKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "publishVoteEncryptionPrivateKey", processId, voteEncryptionPrivateKey)
}

// PublishVoteEncryptionPrivateKey is a paid mutator transaction binding the contract method 0xe113808a.
//
// Solidity: function publishVoteEncryptionPrivateKey(bytes32 processId, string voteEncryptionPrivateKey) returns()
func (_VotingProcess *VotingProcessSession) PublishVoteEncryptionPrivateKey(processId [32]byte, voteEncryptionPrivateKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.PublishVoteEncryptionPrivateKey(&_VotingProcess.TransactOpts, processId, voteEncryptionPrivateKey)
}

// PublishVoteEncryptionPrivateKey is a paid mutator transaction binding the contract method 0xe113808a.
//
// Solidity: function publishVoteEncryptionPrivateKey(bytes32 processId, string voteEncryptionPrivateKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) PublishVoteEncryptionPrivateKey(processId [32]byte, voteEncryptionPrivateKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.PublishVoteEncryptionPrivateKey(&_VotingProcess.TransactOpts, processId, voteEncryptionPrivateKey)
}

// RegisterRelay is a paid mutator transaction binding the contract method 0xea529083.
//
// Solidity: function registerRelay(bytes32 processId, address relayAddress) returns()
func (_VotingProcess *VotingProcessTransactor) RegisterRelay(opts *bind.TransactOpts, processId [32]byte, relayAddress common.Address) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "registerRelay", processId, relayAddress)
}

// RegisterRelay is a paid mutator transaction binding the contract method 0xea529083.
//
// Solidity: function registerRelay(bytes32 processId, address relayAddress) returns()
func (_VotingProcess *VotingProcessSession) RegisterRelay(processId [32]byte, relayAddress common.Address) (*types.Transaction, error) {
	return _VotingProcess.Contract.RegisterRelay(&_VotingProcess.TransactOpts, processId, relayAddress)
}

// RegisterRelay is a paid mutator transaction binding the contract method 0xea529083.
//
// Solidity: function registerRelay(bytes32 processId, address relayAddress) returns()
func (_VotingProcess *VotingProcessTransactorSession) RegisterRelay(processId [32]byte, relayAddress common.Address) (*types.Transaction, error) {
	return _VotingProcess.Contract.RegisterRelay(&_VotingProcess.TransactOpts, processId, relayAddress)
}

// SetVotingQuestion is a paid mutator transaction binding the contract method 0x4168a18e.
//
// Solidity: function setVotingQuestion(bytes32 processId, string voteEncryptionPrivateKey) returns()
func (_VotingProcess *VotingProcessTransactor) SetVotingQuestion(opts *bind.TransactOpts, processId [32]byte, voteEncryptionPrivateKey string) (*types.Transaction, error) {
	return _VotingProcess.contract.Transact(opts, "setVotingQuestion", processId, voteEncryptionPrivateKey)
}

// SetVotingQuestion is a paid mutator transaction binding the contract method 0x4168a18e.
//
// Solidity: function setVotingQuestion(bytes32 processId, string voteEncryptionPrivateKey) returns()
func (_VotingProcess *VotingProcessSession) SetVotingQuestion(processId [32]byte, voteEncryptionPrivateKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.SetVotingQuestion(&_VotingProcess.TransactOpts, processId, voteEncryptionPrivateKey)
}

// SetVotingQuestion is a paid mutator transaction binding the contract method 0x4168a18e.
//
// Solidity: function setVotingQuestion(bytes32 processId, string voteEncryptionPrivateKey) returns()
func (_VotingProcess *VotingProcessTransactorSession) SetVotingQuestion(processId [32]byte, voteEncryptionPrivateKey string) (*types.Transaction, error) {
	return _VotingProcess.Contract.SetVotingQuestion(&_VotingProcess.TransactOpts, processId, voteEncryptionPrivateKey)
}
